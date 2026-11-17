# 父级文件夹统一目录 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 选定一个父级文件夹后，steamcmd、游戏服务器、模组、备份、存档全部收进这一棵树（相对路径 + 默认值），并提供旧数据的检测与逐项勾选搬家。

**Architecture:** `dst_config` 新增 `parent_path`；路径解析集中在 `dstConfigUtils.ResolvePath` 纯函数，在读取边界（`clusterUtils.GetCluster` 与少数直接消费点）解析成绝对路径，其余代码不动。迁移功能独立成 `service/migration`（scan + migrate），经 `/api/dst/config/migration/*` 暴露，前端在 `DstConfigSetting` 页加检测/搬家面板。

**Tech Stack:** Go 1.20 / Gin / GORM（仅改配置层，不动 DB）；React 18 + antd 5 + vite；不引入任何新依赖（Go 与 npm 均是）。

**Spec:** `dst-admin-go/docs/superpowers/specs/2026-11-13-parent-folder-design.md`（下称 §n 指该文档章节）

## Global Constraints

- 不引入新 Go module、新 npm 依赖；Windows 与 Linux 双平台行为一致（路径用 `filepath`，移动不走 shell）。
- 迁移反目标：任何情况下不硬删源数据；目标已存在且非空一律拒绝该项；验证失败不得破坏源（§4）。
- `parent_path` 为空 = 完全旧行为（§2.7）；绝对路径（含 `~/` 展开后）运行时语义不变（§8.3）。
- JSON/配置键名固定：`parent_path`、`last_parent_path`；类别代码固定：`steamcmd` / `server` / `mod` / `backup` / `worlds`；状态代码固定：`migratable` / `target_conflict` / `game_running` / `already_in_place` / `not_found`。
- 迁移后配置改写为相对默认值（§11.4）：`steamcmd=steamcmd`、`force_install_dir=dst-dedicated-server`、`backup=backup`、`mod_download_path=mod`、`persistent_storage_root=worlds`。
- 提交信息结尾附：`Co-Authored-By: Claude Code <noreply@anthropic.com>`。后端仓库在 `dst-admin-go/`，前端仓库在 `dst-admin-web/`，各自提交。
- i18n 只需补齐 `zh.json` 与 `en.json`（其余语言维持现状）。

## Review Focus

1. **老配置回归**（parent 为空）：任何路径行为必须与改动前逐字节一致——Task 3 用空 parent 固定 `GetCluster` 原样返回；复审时对照 `GetDstConfig` 的默认值物化行为未被误删。
2. **含空格路径**（Windows 常见 `C:\Program Files\...`）：解析与迁移不得走 shell 拼接——Task 2/6 的测试目录名带空格。
3. **迁移失败源完好**：复制校验失败时源目录原样、目标副本清除——Task 6 有专门用例。
4. **beta=1 存档目录**：世界归位到 `DoNotStarveTogetherBetaBranch`，与运行时一致——Task 3 固定 `GetKleiDstPath` 行为。
5. **游戏运行中拒搬 server/worlds**：Task 5/6 固定 `game_running` 状态与拒绝逻辑。

---

### Task 1: dst_config 字段扩展（parent_path / last_parent_path）

**Files:**
- Modify: `dst-admin-go/utils/dstConfigUtils/dstConfig.go`
- Test: `dst-admin-go/utils/dstConfigUtils/dstConfig_test.go`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces:
  - `DstConfig` 新增字段 `Parent_path string \`json:"parent_path"\``、`Last_parent_path string \`json:"last_parent_path"\``
  - `var dst_config_path = "./dst_config"`（由 const 改 var，测试可指向临时文件）
  - `SaveDstConfig(dstConfig *DstConfig)` 持久化两键；当 `dstConfig.Parent_path != old.Parent_path` 且旧值非空时，写入 `last_parent_path=<旧 parent>`；`GetDstConfig()` 解析两键。

- [ ] **Step 1: 写失败测试**（`dstConfig_test.go`，测试内先把 `dst_config_path` 指到 `t.TempDir()` 下文件，结束后恢复）

```go
func TestSaveGetRoundTripParentPath(t *testing.T)     // 存 parent_path=/data/dst 再读回，两字段一致
func TestParentChangeRecordsLastParentPath(t *testing.T) // 先存 A 再存 B → Get 读到 parent=B, last=A
func TestParentSameSaveKeepsLastParentPath(t *testing.T) // 两次存 A → last 保持不变
func TestLegacyFileWithoutParentKeys(t *testing.T)        // 写入无 parent 键的旧格式文件 → 读出空值不报错
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./utils/dstConfigUtils/ -run "Parent|Legacy" -v`
Expected: FAIL（字段/函数不存在或行为不符）

- [ ] **Step 3: 实现**

`DstConfig` 加两个字段；解析循环加 `parent_path=`、`last_parent_path=` 两个分支（照抄现有 `cluster=` 分支的写法）；`SaveDstConfig` 在写行列表头部加两行（顺序：`parent_path=`、`last_parent_path=` 在最前），并按上述规则在保存前计算 `last_parent_path`。`const dst_config_path` 改为 `var`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./utils/dstConfigUtils/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add utils/dstConfigUtils/dstConfig.go utils/dstConfigUtils/dstConfig_test.go
git commit -m "feat: add parent_path/last_parent_path to dst_config

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 2: ResolvePath 纯函数（§8 路径解析规则）

**Files:**
- Modify: `dst-admin-go/utils/dstConfigUtils/dstConfig.go`
- Test: `dst-admin-go/utils/dstConfigUtils/resolve_test.go`

**Interfaces:**
- Consumes: Task 1 的字段（不依赖其行为，纯函数独立）
- Produces:
  - `func ExpandHome(p string) string`：`~` / `~/` / `~\` 前缀展开为用户主目录；其余原样返回
  - `func ResolvePath(parent, p, def string) string`，判定顺序严格按 §8.3：
    1. `p == ""` 时先取 `def`；`def` 也空则返回 `""`
    2. `~` 前缀 → `ExpandHome(p)`
    3. `filepath.IsAbs(p)` → 原样返回
    4. `parent == ""` → 原样返回 `p`（旧逻辑）
    5. 否则 → `filepath.Join(parent, p)`
  - 默认相对值常量：`DefaultSteamcmd = "steamcmd"`、`DefaultServerDir = "dst-dedicated-server"`、`DefaultBackup = "backup"`、`DefaultMod = "mod"`、`DefaultWorlds = "worlds"`（放在本文件）

- [ ] **Step 1: 写失败测试**（表驱动，目录名含空格用例必须有）

```go
func TestExpandHome(t *testing.T)          // "~/steamcmd" → 主目录拼接；"~" → 主目录；"/data/x" 原样
func TestResolvePath(t *testing.T)         // 表驱动用例：
// ("D:\\DST", "steamcmd", "steamcmd")          → <D:\DST\steamcmd>
// ("D:\\DST", "C:\\other\\backup", "backup")   → C:\other\backup（绝对不变）
// ("D:\\DST", "~/steamcmd", "steamcmd")        → <主目录>\steamcmd
// ("D:\\DST", "", "mod")                       → <D:\DST\mod>（空取默认）
// ("", "steamcmd", "steamcmd")                 → "steamcmd"（parent 空原样）
// ("D:\\DST", "", "")                          → ""
// ("/opt/dst parent", "mod", "mod")            → /opt/dst parent/mod（含空格）
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./utils/dstConfigUtils/ -run "Resolve|Expand" -v`
Expected: FAIL

- [ ] **Step 3: 实现**两个函数与五个常量（签名见 Interfaces，判定顺序不可改）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./utils/dstConfigUtils/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add utils/dstConfigUtils/dstConfig.go utils/dstConfigUtils/resolve_test.go
git commit -m "feat: add ResolvePath/ExpandHome path resolution helpers

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 3: 读取边界接线 + GetKleiDstPath 修正（§8、§9）

**Files:**
- Modify: `dst-admin-go/utils/clusterUtils/clusterUtils.go`（`GetCluster`、`GetClusterFromGin`）
- Modify: `dst-admin-go/utils/dstUtils/dstUtils.go`（`GetKleiDstPath` 及 `GetUgcWorkshopModPath`/`GetUgcModPath`/`GetUgcAcfPath` 中的路径拼接）
- Modify: `dst-admin-go/service/mod/mod_download.go`（steamcmd 与 mod_download_path 两处）
- Modify: `dst-admin-go/api/modApi.go:171,250,300`、`dst-admin-go/api/fileApi.go:27-30`、`dst-admin-go/api/gameLevel2Api.go:168`
- Test: `dst-admin-go/utils/dstUtils/dstUtils_test.go`、`dst-admin-go/utils/clusterUtils/clusterUtils_test.go`

**Interfaces:**
- Consumes: Task 2 的 `ResolvePath` / 常量
- Produces:
  - `GetCluster` / `GetClusterFromGin` 返回的 `model.Cluster` 中 `SteamCmd`、`ForceInstallDir`、`Backup`、`ModDownloadPath`、`Persistent_storage_root` 为**解析后的绝对路径**（`Persistent_storage_root` 空且 parent 非空时解析 `DefaultWorlds`）；`Ugc_directory`、`Conf_dir` 原样透传
  - `dstUtils.GetKleiDstPath()`：`persistent_storage_root` 非空时 `conf_dir` 默认值改为 `beta == 1 ? "DoNotStarveTogetherBetaBranch" : "DoNotStarveTogether"`（修 §9 现状隐患）；其余逻辑不动
  - 各直接消费点改为调用 `ResolvePath`（`mod_download.go` 用 `ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)` 同款写法）

- [ ] **Step 1: 写失败测试**

```go
// clusterUtils_test.go（用 Task 1 的 var dst_config_path 指到临时文件）
func TestGetClusterResolvesRelativePaths(t *testing.T) // parent=D:\DST, 四字段相对 → 绝对拼接
func TestGetClusterKeepsAbsolutePaths(t *testing.T)    // 绝对路径原样
func TestGetClusterEmptyParentLegacy(t *testing.T)     // parent 空 → 值原样返回（Review Focus 1）
// dstUtils_test.go
func TestGetKleiDstPathBetaBranch(t *testing.T)   // persistent 非空、conf 空、beta=1 → <persistent>/DoNotStarveTogetherBetaBranch
func TestGetKleiDstPathNonBeta(t *testing.T)      // 同上 beta=0 → <persistent>/DoNotStarveTogether
func TestGetKleiDstPathDefaultKlei(t *testing.T)  // persistent 空 → KleiDstPath() 现状（含 BetaBranch 后缀）
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./utils/... -v`
Expected: 新测试 FAIL

- [ ] **Step 3: 实现**接线与 `GetKleiDstPath` 修正。游戏启动命令拼接处（`gameService.go` 等）**不动**——它们消费 `cluster.*` 已解析值；`beta=1` 时 `-beta` 后缀追加行为（`gameService.go:232`）保持现状。另改 `GetDstConfig` 的默认值物化块（`dstConfig.go:183-191`）：`Parent_path != ""` 时**不**把 `Backup`/`Mod_download_path` 物化成 Klei 绝对路径（留空，由 `ResolvePath` 供给默认值，保证 GET 回显原始值 §8.1）；`Parent_path == ""` 时行为逐字节不变（Review Focus 1）。新增测试 `TestGetDstConfigNoMaterializeWhenParentSet`、`TestGetDstConfigLegacyMaterializeUnchanged` 固定这两半行为。

- [ ] **Step 4: 运行全部后端测试确认通过**

Run: `cd dst-admin-go && go test ./... 2>&1 | tail -20`
Expected: 全部 PASS（含既有测试）

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add -A utils/ service/mod/mod_download.go api/modApi.go api/fileApi.go api/gameLevel2Api.go
git commit -m "feat: resolve dst paths at read boundaries, fix BetaBranch klei path

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 4: 迁移类型与分类纯函数（§10、§11.5）

**Files:**
- Create: `dst-admin-go/service/migration/migrationTypes.go`
- Create: `dst-admin-go/service/migration/classify.go`
- Test: `dst-admin-go/service/migration/classify_test.go`

**Interfaces:**
- Consumes: 无（纯函数与类型）
- Produces:

```go
type MigrationUnit struct {
    ID        string `json:"id"`         // 稳定标识，如 "worlds:MyDediServer"、"steamcmd"
    Kind      string `json:"kind"`       // steamcmd|server|mod|backup|worlds
    Source    string `json:"source"`     // 待搬目录绝对路径（worlds 为单个 cluster 目录）
    SourceLabel string `json:"sourceLabel"` // "当前配置"|"旧默认位置"|"上次父级文件夹"（中文，前端展示）
    SizeBytes int64  `json:"sizeBytes"`
    Target    string `json:"target"`     // 迁移目标绝对路径
    Status    string `json:"status"`     // migratable|target_conflict|game_running|already_in_place|not_found
    Note      string `json:"note"`       // 拆分/排除说明，可为空
    TopLevelFilesOnly bool `json:"topLevelFilesOnly"` // true=只搬 Source 顶层文件，目录保留（共享源拆分，§11.5）
}

func IsClusterDir(entries []os.DirEntry) bool // 含 cluster.ini，或含 Master/ 或 Caves/ 子目录
func IsJunkName(name string) bool             // bak、backup、mod_download、mod_config_download、*.pre-migrate-*、隐藏名
func KnownLegacyRoots(home string) []string   // 旧默认位置清单，见下
func IsNestedOrSame(a, b string) bool         // 规范化后同路径或父子关系（去重用）
```

`KnownLegacyRoots` 清单（按 §10 与 builder 核对结论固定）：`~/steamcmd`、`~/dst-dedicated-server`、`~/dst`、`~/dontstarve_dedicated_server`、Klei 默认存档根（Windows: `<home>/Documents/klei/DoNotStarveTogether` 与 `...BetaBranch`；Linux: `<home>/.klei/DoNotStarveTogether` 与 `...BetaBranch`）、Docker：`/app/steamcmd`、`/app/dst-dedicated-server`、`/app/backup`、`/app/mod`。`last_parent_path` 的子树由 scan 侧追加，不在此函数内。

- [ ] **Step 1: 写失败测试**

```go
func TestIsClusterDir(t *testing.T)  // 有 cluster.ini → true；有 Master 子目录 → true；只有 save/ → false
func TestIsJunkName(t *testing.T)    // bak、backup、mod_download、mod_config_download、x.pre-migrate-20260101-000000、.git → true；MyDediServer、Master → false
func TestKnownLegacyRoots(t *testing.T)  // 含 ~/steamcmd 与平台对应的 Klei 路径；Linux 下含 /app/*
func TestIsNestedOrSame(t *testing.T)    // 同路径、父含子 → true；兄弟目录 → false；含空格与大小写差异 → 正确判定
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./service/migration/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**类型与四个函数（`IsNestedOrSame` 用 `filepath.Clean` + 大小写不敏感比较仅限 Windows：`runtime.GOOS == "windows"` 时 `strings.EqualFold`）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./service/migration/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add service/migration/
git commit -m "feat: migration unit types and legacy-path classification

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 5: Scan 服务（§10 检测）

**Files:**
- Create: `dst-admin-go/service/migration/scan.go`
- Test: `dst-admin-go/service/migration/scan_test.go`

**Interfaces:**
- Consumes: Task 4 全部；Task 1/2 的 `dstConfigUtils.DstConfig`、`ResolvePath`、默认常量
- Produces:

```go
func Scan(cfg dstConfigUtils.DstConfig, gameRunning bool) ([]MigrationUnit, error)
func DirSize(path string, topLevelFilesOnly bool) (int64, error) // 递归求和；true 时只算顶层文件
```

Scan 行为合同：
- 候选源：当前配置解析后的位置（五类）、`KnownLegacyRoots` 中存在的目录、`cfg.Last_parent_path` 下各默认子目录（若 last 非空）。
- worlds 单元 = 各候选 Klei 根下的 cluster 子目录（`IsClusterDir`），`IsJunkName` 的子目录跳过（Note 记"已排除：<名>"）。
- 共享源拆分（§11.5）：同一根目录既配置为 backup 又含 cluster 子目录时，cluster 子目录出 worlds 单元，根顶层文件出一个 `backup` 单元（`TopLevelFilesOnly=true`）；**同一 Source 规范化路径只出现一次**，嵌套外层含已出单元的子树时外层 `Note` 标注并跳过该子树。
- 各类 Target：`filepath.Join(parent, 默认相对值)`（parent 空时 Target 为空且 Status=`not_found`）；worlds 的 Target = `<parent>/worlds/<conf_dir 默认含 BetaBranch>/<cluster名>`（`beta==1` 用 `DoNotStarveTogetherBetaBranch`）。
- server 类 Source/Target 在 `beta==1` 时追加 `-beta` 后缀（与 `gameService.go:232` 运行时一致）。
- Status：Source 不存在 → `not_found`；Source 规范化等于 Target → `already_in_place`；Target 存在且非空 → `target_conflict`；kind 为 `server`/`worlds` 且 `gameRunning` → `game_running`；否则 → `migratable`。
- SizeBytes 用 `DirSize`。

- [ ] **Step 1: 写失败测试**（全部用 `t.TempDir()` 搭目录树）

```go
func TestScanFindsLegacyKleiClusters(t *testing.T)  // 搭 Klei 根含 2 个 cluster + bak/ → 2 个 worlds 单元，bak 不出单元
func TestScanSharedRootSplitsBackupAndWorlds(t *testing.T) // backup 配置=Klei 根 → 2 个 worlds 单元 + 1 个 backup 单元(TopLevelFilesOnly=true)，同源去重
func TestScanTargetConflictAndInPlace(t *testing.T)  // 目标已非空 → target_conflict；源=目标 → already_in_place
func TestScanGameRunningBlocks(t *testing.T)         // gameRunning=true → server/worlds 状态 game_running，其他不受影响
func TestScanBetaSuffixServerTarget(t *testing.T)    // beta=1 → server 的 Source/Target 带 -beta，worlds Target 用 BetaBranch 目录
func TestDirSizeTopLevelFilesOnly(t *testing.T)      // 含子目录的树：true 只算顶层文件
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./service/migration/ -run Scan -v`
Expected: FAIL

- [ ] **Step 3: 实现** `Scan` 与 `DirSize`（签名与行为合同照上；`os.ReadDir` + 递归，不引依赖）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./service/migration/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add service/migration/
git commit -m "feat: migration scan service with shared-source split rules

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 6: Migrate 服务（§11 搬家）

**Files:**
- Create: `dst-admin-go/service/migration/migrate.go`
- Test: `dst-admin-go/service/migration/migrate_test.go`

**Interfaces:**
- Consumes: Task 4/5 的 `MigrationUnit`、`DirSize`
- Produces:

```go
type MigrateResult struct {
    ID      string `json:"id"`
    OK      bool   `json:"ok"`
    Message string `json:"message"`
}

func Migrate(units []MigrationUnit, selectedIDs []string, gameRunning bool) []MigrateResult
func RewriteConfigAfterMigration(cfg *dstConfigUtils.DstConfig, succeeded []MigrationUnit) // 见下
```

Migrate 行为合同：
- 只处理 `selectedIDs` 中的单元；未选中的一字节不动。
- `gameRunning` 且 kind 为 `server`/`worlds` → 直接 `OK=false, Message="游戏运行中"`。
- `status != migratable`（`target_conflict`/`already_in_place`/`not_found`）→ 拒绝并说明。
- `TopLevelFilesOnly=true`：把 Source 顶层文件逐个 `os.Rename` 到 Target（Target 不存在则创建）；跨卷失败时复制该文件并校验大小后删源文件（这是唯一允许删源的场景，仅限单文件且已校验）。
- 整目录移动 `moveDir(src, dst)`：
  1. `dst` 存在且非空 → 拒绝（双保险）；
  2. 尝试 `os.Rename(src, dst)`，成功即完；
  3. 失败（跨卷）→ 递归复制到 `dst`，校验 `DirSize` 文件数与总字节（复制前后各算一次源/目标比对）；校验通过 → `os.Rename(src, src+".pre-migrate-"+time.Now().Format("20060102-150405"))`；校验失败 → `os.RemoveAll(dst)` 后返回错误，**源不动**。
- 全部处理完返回逐项结果，允许部分成功。

`RewriteConfigAfterMigration`：对 `succeeded` 中出现过的 kind 改写 cfg 字段为相对默认值（全局约束列出的 5 个键；`worlds` 成功时同时改 `Persistent_storage_root="worlds"`，`Conf_dir` 保持/置为含 BetaBranch 的默认目录名）；未出现过的 kind 字段不动。API 层随后调用 `dstConfigUtils.SaveDstConfig` 落盘。

- [ ] **Step 1: 写失败测试**（`t.TempDir()` 搭源/目标；目录名至少一个含空格）

```go
func TestMoveDirRename(t *testing.T)              // 同卷 rename 移动成功，源消失目标在
func TestMoveDirConflictRejected(t *testing.T)    // 目标非空 → 错误，源目标均不变
func TestVerifyDirEqualDetectsMismatch(t *testing.T) // moveDir 内部的 `verifyDirEqual(src, dst) error` 独立单测：缺文件/字节数不一致 → 非 nil；一致 → nil
func TestMoveTopLevelFilesOnly(t *testing.T)      // 顶层 2 文件 + 1 子目录 → 文件到目标，子目录留在源，源目录本身保留
func TestMigrateSkipsUnselectedAndBlocked(t *testing.T) // 未选中不动；gameRunning 拒 server/worlds；target_conflict 拒
func TestRewriteConfigAfterMigration(t *testing.T) // 成功 kind 改相对默认值，未成功 kind 原样，worlds 连带 persistent_storage_root
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./service/migration/ -run "Move|Migrate|Rewrite" -v`
Expected: FAIL

- [ ] **Step 3: 实现** `moveDir` / `moveTopLevelFiles` / `Migrate` / `RewriteConfigAfterMigration`（`moveDir` 的复制-校验-改名保留源是反目标防线，不可省）；大小写与分隔符归一用 `filepath.Clean` 比对，不走 shell。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./service/migration/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add service/migration/
git commit -m "feat: migration execute service with safe move strategy

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 7: API、路由与 container_mode（§10-§12 后端侧）

**Files:**
- Create: `dst-admin-go/api/migrationApi.go`
- Modify: `dst-admin-go/router/dstConfig.go`（追加路由）
- Modify: `dst-admin-go/api/dstConfigApi.go`（`GetDstConfig` 响应加计算字段）
- Test: `dst-admin-go/api/migrationApi_test.go`

**Interfaces:**
- Consumes: Task 5 `migration.Scan`；Task 6 `migration.Migrate` / `RewriteConfigAfterMigration`；`service.GameService{}.GetLevelStatus`
- Produces:
  - `GET /api/dst/config/migration/scan` → `vo.Response{Code:200, Data:{units:[MigrationUnit...], gameRunning:bool}}`
  - `POST /api/dst/config/migration/execute`，请求体 `{"ids":["worlds:MyDediServer", ...]}` → `vo.Response{Code:200, Data:{results:[MigrateResult...]}}`；执行前取 `GameService.GetLevelStatus(cfg.Cluster,"Master")||GetLevelStatus(cfg.Cluster,"Caves")` 作为 `gameRunning`；成功项调用 `RewriteConfigAfterMigration` + `SaveDstConfig` 落盘
  - `GET /api/dst/config` 响应体（在 handler 内组 map 或嵌入结构）额外含：
    - `container_mode bool`：`os.Getenv("DOCKER_CONTAINER") != ""`
    - `resolved map[string]string`：五个键（`steamcmd`/`force_install_dir`/`backup`/`mod_download_path`/`persistent_storage_root`）的 `ResolvePath` 结果，前端做路径预览
  - 前端 `MigrationUnit` JSON 字段名以 Task 4 的 struct tag 为准

- [ ] **Step 1: 写失败测试**（httptest + 临时 `dst_config_path`，覆盖 `home` 用例不依赖真实环境的部分）

```go
func TestScanEndpointReturnsUnits(t *testing.T)      // 200，Data.units 为数组、Data.gameRunning 为 bool
func TestExecuteEndpointRejectsUnknownID(t *testing.T) // 未知 id → 对应 result.ok=false，code 仍 200
func TestGetDstConfigIncludesContainerModeAndResolved(t *testing.T) // 响应含 container_mode 与 resolved 五键
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd dst-admin-go && go test ./api/ -run "Migration|DstConfig" -v`
Expected: FAIL

- [ ] **Step 3: 实现** `MigrationApi` 两个 handler（`vo.Response` 包裹、错误不 panic 走 `code!=200`）、路由挂到 `/api/dst/config/migration` 组、`GetDstConfig` 扩展响应。container_mode 下 handler 直接返回 `code=403, msg="container mode disabled"`（§12 裁断的服务端门禁，前端同时隐藏）。另改 `api/installSteamcmdApi.go:213-216`：`Parent_path` 非空时不再写 `consts.HomePath` 绝对路径，改写相对默认值（`steamcmd`/`dst-dedicated-server`/`backup`/`mod`），让全新安装落进父级树；parent 为空时保持现状。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd dst-admin-go && go test ./... 2>&1 | tail -20`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
cd dst-admin-go && git add api/ router/
git commit -m "feat: migration scan/execute API with container_mode gating

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 8: 前端配置页 — 父级文件夹与路径预览

**Files:**
- Modify: `dst-admin-web/src/pages/System/DstConfigSetting/index.jsx`
- Modify: `dst-admin-web/src/api/dstConfigApi.jsx`（新增 `scanMigration()`、`executeMigration(ids)` 封装，本任务只加封装函数）
- Modify: `dst-admin-web/src/locales/lang/zh.json`、`en.json`
- 无测试基础设施 → 验证 = build + lint

**Interfaces:**
- Consumes: Task 7 的 `GET /api/dst/config` 响应新字段 `parent_path`、`container_mode`、`resolved`
- Produces:
  - 表单字段 `parent_path`（置于表单最前，label `setting.dstConfig.parent_path`）
  - 四个路径项下方各加一行灰色预览文本：`resolved[fieldName]`
  - `data.container_mode` 为 true 时：`parent_path` 输入框 disabled、检测/搬家入口不渲染（Task 9 挂接点预留容器 div `id="migration-panel"` 或直接在 Task 9 渲染组件时判断）
  - i18n 键（zh/en 同名）：`setting.dstConfig.parent_path`、`.parent_path.placeholder`、`.parent_path.tooltip`、`.resolvedPreview`

- [ ] **Step 1: 实现**表单与预览（保存逻辑不动；`onFinish` 同样提交 `parent_path`，空字符串合法表示"不启用"）。另按 §11.6：保存时若 `parent_path` 变更且任一 `resolved` 项指向不存在的路径，`Modal.warning` 提示"数据仍在旧父级文件夹，可运行检测搬家"，**不阻塞**保存（确认后仍执行保存）。

- [ ] **Step 2: 构建验证**

Run: `cd dst-admin-web && npm run build 2>&1 | tail -5`
Expected: 构建成功

- [ ] **Step 3: lint 验证**

Run: `cd dst-admin-web && npm run lint 2>&1 | tail -5`
Expected: 无 error（warning 可接受，与现状一致）

- [ ] **Step 4: Commit**

```bash
cd dst-admin-web && git add src/pages/System/DstConfigSetting/index.jsx src/api/dstConfigApi.jsx src/locales/lang/zh.json src/locales/lang/en.json
git commit -m "feat: parent folder field and resolved path preview in dst config page

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 9: 前端检测/搬家面板（§12）

**Files:**
- Create: `dst-admin-web/src/pages/System/DstConfigSetting/MigrationPanel.jsx`
- Modify: `dst-admin-web/src/pages/System/DstConfigSetting/index.jsx`（渲染面板）
- Modify: `dst-admin-web/src/api/dstConfigApi.jsx`（如 Task 8 未加完则补全）
- Modify: `dst-admin-web/src/locales/lang/zh.json`、`en.json`
- 验证 = build + lint

**Interfaces:**
- Consumes: Task 8 的表单容器；`scanMigration()` → `{units, gameRunning}`；`executeMigration(ids)` → `{results}`
- Produces: `MigrationPanel` 组件，props：`parentPath string`、`containerMode bool`、`onMigrated () => void`（搬家成功后父组件刷新配置）
  - `containerMode` 或 `parentPath` 为空 → 组件不渲染任何按钮（parent 为空时显示一行提示：先设置父级文件夹）
  - "检测"按钮 → `scanMigration()` → antd Table：勾选框（默认全不勾）、kind、sourceLabel、source、target、sizeBytes（人类可读格式）、status（i18n 标签）、note
  - "搬家"按钮（有勾选时可用）→ `Modal.confirm` 列出勾选项 `source → target` → `executeMigration(ids)` → 结果列表 + `onMigrated()`
  - i18n 键：`setting.dstConfig.migration.scan`、`.migrate`、`.confirm`、`.column.*`（kind/source/target/size/status/note/checkbox）、`.status.migratable|target_conflict|game_running|already_in_place|not_found`、`.result.*`、`.needParent`、`.kind.steamcmd|server|mod|backup|worlds`

- [ ] **Step 1: 实现**组件与挂接（表格列与状态码严格对齐 Global Constraints 的固定值）

- [ ] **Step 2: 构建验证**

Run: `cd dst-admin-web && npm run build 2>&1 | tail -5`
Expected: 构建成功

- [ ] **Step 3: lint 验证**

Run: `cd dst-admin-web && npm run lint 2>&1 | tail -5`
Expected: 无 error

- [ ] **Step 4: Commit**

```bash
cd dst-admin-web && git add src/pages/System/DstConfigSetting/ src/api/dstConfigApi.jsx src/locales/lang/zh.json src/locales/lang/en.json
git commit -m "feat: migration scan/migrate panel in dst config page

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 10: 首次设置向导同步 + 收尾

**Files:**
- Modify: `dst-admin-web/src/pages/Begin/component/Setting.jsx`
- Modify: `dst-admin-web/src/locales/lang/zh.json`、`en.json`
- 验证 = build + lint + 后端全量测试

**Interfaces:**
- Consumes: Task 7 的 `POST /api/init` 兼容性（`Setting.jsx` 现只提交 userInfo，dstConfig 未随 init 提交——保持现状，只改表单字段与默认值展示；若 `Setting.jsx` 有提交 dstConfig 的路径则带 `parent_path`）
- Produces:
  - `Setting.jsx` 新增 `parent_path` 表单项（可空）
  - 默认值从 `~/steamcmd`、`~/dst`、`~/.klei/DoNotStarveTogether/mod` 改为相对默认值 `steamcmd`、`dst-dedicated-server`、`mod`、`backup`（placeholder 显示"相对父级文件夹"）

- [ ] **Step 1: 实现**向导字段与默认值替换

- [ ] **Step 2: 前后端全量验证**

Run: `cd dst-admin-web && npm run build && npm run lint`
Run: `cd dst-admin-go && go test ./... 2>&1 | tail -20`
Expected: 构建成功、lint 无 error、Go 测试全 PASS

- [ ] **Step 3: Commit**

```bash
cd dst-admin-web && git add src/pages/Begin/component/Setting.jsx src/locales/lang/zh.json src/locales/lang/en.json
git commit -m "feat: parent folder field in first-run wizard with relative defaults

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

## 任务依赖

Task 1 → 2 → 3 顺序执行；Task 4 可与 1-3 并行；Task 5 依赖 2/4；Task 6 依赖 5 的类型；Task 7 依赖 5/6；Task 8 依赖 7 的响应字段；Task 9 依赖 8；Task 10 依赖 8。

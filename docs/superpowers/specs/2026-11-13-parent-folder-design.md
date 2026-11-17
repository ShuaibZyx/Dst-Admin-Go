# 父级文件夹统一目录 — 设计文档

日期：2026-11-13
状态：待批准

## 1. 意图与动机

现在游戏相关的资产散落在各处：steamcmd、专用服务器安装目录、模组下载目录、备份目录各自独立配置（`dst_config`），存档更是默认藏在系统的 Klei 目录（`~/.klei/DoNotStarveTogether` 或 Windows 的"文档\klei"）。管理、备份、换机器都很麻烦。

目标体验：**用户只选定一个父级文件夹，所有游戏相关资产自动收进这一棵树**；每个子目录都有默认值，不想管的人只填一个路径就完事；想细调的人还能改子目录名；已有的散装数据可以被检测出来并一键搬家。

## 2. 目标

1. `dst_config` 新增**父级文件夹**（`parent_path`）配置项。
2. 现有四个路径选项（`steamcmd`、`force_install_dir`、`backup`、`mod_download_path`）**保持不动**，语义扩展为：填相对路径 → 拼在父级文件夹下；填绝对路径 → 完全照旧生效。
3. 每个路径选项有**默认相对值**，最省事的用法是只填父级文件夹。
4. **存档（世界）** 自动放进父级文件夹（借助现有的 `-persistent_storage_root` / `-conf_dir` 游戏参数，游戏无感知）。
5. **检测**：识别现有数据（含旧默认位置）在不在、在哪、多大、将迁去哪。
6. **搬家**：把检测到的数据迁移到新目录树，安全第一。
7. **兼容**：未设置父级文件夹 = 完全旧行为；设置了但某项是绝对路径 = 该项按绝对路径走。老用户升级后配置不受影响。

## 3. 非目标

- 不管理面板自身文件（`dst-db`、`dst-admin-go.log`、`config.yml`、`session.secret` 等）——面板有自己的管理方式。
- 不改变多房间（cluster）机制：父级文件夹是**全局一个**，各 cluster 的路径仍来自 `dst_config`。
- 不改变 `bin`（32/64/luajit/box86/box64）与 `beta` 的语义。
- 不做未经用户确认的自动搬家；不合并、不覆盖已存在的目标目录。

## 4. 反目标（做到即失败）

- 迁移过程中丢失或损坏存档、模组、备份（宁可慢、宁可多占一份空间）。
- 升级后面板读不懂老配置、游戏起不来。
- 路径拼接出现歧义（把相对路径当绝对路径用，或反过来）。

## 5. 约束

- Windows 与 Linux 都要支持（项目现状：Linux 走 screen 启动，Windows 走 `windowGameCli`）。
- 后端 Go/Gin，配置持久化在 `./dst_config`（key=value 行格式，`utils/dstConfigUtils/dstConfig.go`）；前端 React + antd，设置页在 `dst-admin-web/src/pages/System/DstConfigSetting/index.jsx`。
- 游戏参数 `-ugc_directory` / `-persistent_storage_root` / `-conf_dir` 仅在非空时追加（现状，`service/gameService.go`、`service/windowGameCli.go`）。

## 6. 已定决策（来自与用户的对话，不再改）

| 决策 | 内容 |
|---|---|
| 交互模型 | 全局父级文件夹 + 子路径相对化，默认值齐备 |
| 覆盖范围 | steamcmd、服务器安装、模组下载、备份、存档（世界），共五类 |
| 面板文件 | 不纳入 |
| 存档 | 一并收进父级文件夹 |
| 老数据 | 提供**检测 + 搬家**功能 |
| 搬家参与范围 | 检测列出全部五类（含自定义绝对路径），逐项勾选确认才搬；未勾选的保持原样，勾选的搬后改写为相对路径 |
| 兼容 | 现有配置项不动；绝对路径照旧；未设父级 = 旧行为 |

## 7. 目录布局与默认相对值

```
<parent_path>/                    ← 用户选定的父级文件夹
├── steamcmd/                     ← steamcmd 默认相对值: "steamcmd"
├── dst-dedicated-server/         ← force_install_dir 默认: "dst-dedicated-server"
├── mod/                          ← mod_download_path 默认: "mod"
│   └── steamapps/workshop/content/322330/<创意工坊ID>/...
├── backup/                       ← backup 默认: "backup"
└── worlds/                       ← 存档（见 §9）
    └── DoNotStarveTogether[+BetaBranch]/
        └── <cluster>/
            ├── Master/ ...
            └── Caves/ ...
```

- `cluster`（集群名）不是路径，维持现状（默认 `Cluster1`）。
- `bin`、`beta`、`ugc_directory` 等非路径/高级项维持现状。
- 模组下载目录沿用 Steam workshop 缓存布局 `<mod_download_path>/steamapps/workshop/content/322330/<id>`（`api/modApi.go` 现状）。
- 服务器安装目录自带的 `ugc_mods/`（创意工坊模组实际解压处）随 `force_install_dir` 一起迁移。

## 8. 路径解析规则（核心约定）

对四个路径选项 + `parent_path` 自身：

1. **文件里存原始值**（相对或绝对）。UI 读取/回显原始值——防止"保存一圈后相对信息丢失"。
2. **解析只发生在读取边界**：内部消费代码拿绝对路径。改两处接缝即可，其余调用点不用动：
   - `utils/clusterUtils/clusterUtils.go`（`GetCluster` / `GetClusterFromGin`，游戏启动、备份等都从这里拿路径）；
   - 直接调用 `dstConfigUtils.GetDstConfig()` 取路径的调用点（`service/mod/mod_download.go`、`api/fileApi.go`、`utils/dstUtils/dstUtils.go`、`api/modApi.go` 等，实施时 grep 全量核对）。
   - 实现建议：新增 `dstConfigUtils.Resolve(p string) string` 与/或 `GetResolvedDstConfig()`，原始值读写不经过它。
3. **判定顺序**：
   - `parent_path` 为空 → 所有路径原样使用（旧逻辑，兼容开关）。
   - 路径以 `~/` 或 `~\` 开头 → 展开为用户主目录后按绝对路径用（现有 UI 默认值如 `~/steamcmd` 依赖这个；这是现状能跑通的原因之一，必须保住）。
   - `filepath.IsAbs` 为真（含 Windows 盘符/UNC）→ 原样使用。
   - 否则 → `filepath.Join(parent_path, 路径)`。
   - 空值 → 用默认相对值（§7）参与上述判定。
4. 存档（§9）的 `persistent_storage_root` / `conf_dir` 遵循同样的判定与"用户显式配置优先"原则。

## 9. 存档（世界）布局

- 默认：`persistent_storage_root = <parent_path>/worlds`，`conf_dir = "DoNotStarveTogether"`。
- `beta = 1` 时 `conf_dir = "DoNotStarveTogetherBetaBranch"`，与 `dstConfigUtils.KleiDstPath()` 现状命名一致。
- 面板侧 `dstUtils.GetKleiDstPath()` 必须与游戏实际写入位置一致（现状 bug 隐患：`persistent_storage_root` 非空时它不再拼 BetaBranch，本次一并修正）。
- 用户显式设置过 `persistent_storage_root` / `conf_dir`（非空且非默认）→ 尊重用户值，不被父级文件夹改写。

## 10. 检测（scan）

新增 API（命名留给 builder，形如 `GET /api/dst/config/migration/scan`），对五类资产逐类返回：

- **当前配置解析后的位置**是否存在、大小；
- **已知旧默认位置**是否另有数据（`~/steamcmd`、`~/dst-dedicated-server`、`~/dst`、`~/dontstarve_dedicated_server`、Klei 默认存档目录（Windows: `<主目录>\Documents\klei\DoNotStarveTogether[+BetaBranch]`，Linux: `~/.klei/DoNotStarveTogether[+BetaBranch]`）、Docker 默认 `/app/*` 等，清单留给 builder 补全）；
- **目标位置**（按 §8 解析）是否已被占用；
- **游戏是否在运行**（Windows/双平台都要查；迁移涉及该项时要求停机）。

每项状态：`可迁移` / `目标冲突` / `游戏中需停机` / `已在目标位置` / `未发现`。

**参与范围（用户已拍板）**：检测**列出全部五类资产**，无论当前配置是绝对路径、相对路径还是空值——包括用户特意配到别处的绝对路径（如另一块盘上的 backup）。每项的**搬家目标统一为父级文件夹下该类的默认位置**（§7）。与 §2.7"绝对路径照旧"不矛盾：那条说的是**运行时语义**（不搬家时绝对路径继续原样生效）；搬家是用户逐项显式勾选的一次性动作，勾选即表示同意该项改写为相对路径、收编进父级树。

## 11. 搬家（migrate）

新增 API（形如 `POST /api/dst/config/migration/execute`），按检测报告逐项或批量执行：

0. **只执行用户勾选的项**（前端复选框，默认不勾）；未勾选的项一个字节都不动，配置保持原样。
1. **目标已存在且非空 → 拒绝该项**，不合并、不覆盖（整批里允许部分成功，逐项报告）。
2. 迁移服务器安装 / 存档前**必须游戏已停止**，否则拒绝。
3. 移动策略：
   - 同一卷：优先 `os.Rename`（秒级）；
   - 跨卷：复制 → 校验（文件数与总字节数一致）→ 把**源目录改名为 `<原名>.pre-migrate-<时间戳>`** 保留，不硬删除。用户确认无误后自行删除。
4. 成功后把该项在 `dst_config` 中写成新位置（优先写相对路径），再自动重跑一次检测做验证。
5. 中断恢复：单项链式执行、逐项落状态；已改名的源天然可回滚（改回原名）。

### 11.5 旧位置嵌套/同源与"世界"搬家单元（裁断）

旧默认位置经常互相嵌套甚至相同（如 backup = Klei 世界目录本身、mod 目录在世界目录内部）。规则：

- **"世界"类的搬家单元 = 各 cluster 子目录**，不是整个旧 Klei 目录。判定一个子目录是 cluster：含 `cluster.ini`，或含 `Master/`、`Caves/` 子目录。
- 旧目录根层的非 cluster 内容**不**拖进 `worlds/` 目标，按内容归属：备份产物归 `backup` 目标（仅当该路径被配置为 backup 的源时）；`bak/`、`*.pre-migrate-*`、`mod_download/`、`mod_config_download/` 等杂物排除出"世界"类（若某个杂物目录本身就是某类配置的源，则归该类）。
- **同一源路径只处理一次**：若多个配置项指向同一/嵌套路径，按内容拆分到各自目标，绝不整树复制多份；嵌套源先搬内层，外层跳过已搬走的子树。
- 检测报告对拆分场景要逐内容块展示（哪几块去 worlds、哪几块去 backup），状态不重复计。

### 11.6 搬家完成后再次修改 `parent_path`（裁断）

- `parent_path` 变更时，把旧值记入 `dst_config` 的 `last_parent_path`（覆盖式保留最近一次）。
- 下次**检测把 `last_parent_path` 的树当作已知旧位置**纳入扫描，避免数据静默遗弃。
- 保存新 `parent_path` 时，若任一配置项的解析结果指向不存在的位置，前端给出明确警告（"数据仍在旧父级文件夹，是否运行检测搬家？"），但**不阻塞**保存。
- 迁移成功后 `last_parent_path` 中对应内容已搬空的条目自然消失（源改名为 `.pre-migrate-*`，不重复检测）。

## 12. 前端（DstConfigSetting 页）

- 新增"**父级文件夹**"输入框（`parent_path`）。
- 四个路径项保留，placeholder/tooltip 更新为"相对父级文件夹的相对路径，或填绝对路径"，并展示**解析后的完整路径预览**（随输入实时刷新）。
- "**检测**"按钮 → 报告表格（类别 / 发现位置 / 大小 / 目标 / 状态 / **勾选框，默认不勾**）。
- "**搬家**"按钮 → 确认对话框（明示将移动哪些勾选内容、源→目标）→ 执行 → 结果报告。
- `src/pages/Begin/component/Setting.jsx`（首次设置向导）同步加父级文件夹字段，默认值改为 §7 的相对值（顺带摆脱 `~/xxx` 旧默认）。
- **Docker/container_mode 下整个功能关闭**（裁断）：`parent_path` 字段、检测、搬家均不开放，维持现有"改位置请重映射数据卷"的约定，防止资产被挪进非挂载路径、重建容器即丢失。
- 文案全部走 i18n（`locales/` 补中英文）。

## 13. 留给 builder 的部分

- API 具体形状与 REST 命名、错误码约定。
- 迁移扫描旧位置清单的最终补全与各平台判定细节。
- 复制+校验+改名保留源的具体实现（进度回报方式：同步/轮询/SSE，按现有代码风格选）。
- `GetDstConfig()` 消费点改造清单的最终核对方式（grep 范围）。
- 首次安装流程（`api/installSteamcmdApi.go`、`static/script/install_steamcmd.sh` 现写死装到主目录）如何尊重 `parent_path`——目标是装进 `<parent_path>`，实现方式你定。
- 前端组件拆分与表单状态管理。
- 测试策略与用例落点（Go 单测优先；前后端各层怎么测你定）。

## 14. 我做的裁断（builder 可核对，但默认按此执行）

- 新配置键名用 `parent_path`（UI 显示"父级文件夹"）。
- 迁移安全策略：源保留为 `.pre-migrate-<时间戳>`，不硬删（多占空间换安全；这是反目标"丢数据"的直接防线）。
- 存档进 `<parent_path>/worlds/DoNotStarveTogether[+BetaBranch]/<cluster>`（保持 Klei 布局，BetaBranch 与 beta=1 对齐）。
- `~/` 前缀按绝对路径处理并展开（兼容现有 UI 默认值）。
- 父级文件夹为可选；不强制新安装必须设置。
- 检测/搬家只针对 `dst_config` 中当前配置的这组路径（多 cluster 各自的存档目录同属被迁移的"世界"类，一并处理）。

## 15. 验收要点

- 只填 `parent_path`、其余留空 → 五类资产全部落在 §7 的树里，游戏能正常启停、模组能下载、备份能用。
- 填绝对路径的项行为与现在逐字节一致。
- 清空 `parent_path` → 面板回到旧行为，老配置不丢。
- 检测能发现 Klei 默认目录里的旧存档并报告迁移目标。
- 搬家后：游戏从新位置正常启动、存档进得去、源目录还在（改名状态）、目标冲突时拒绝而不是覆盖。

# SDD ledger — plan: D:\New World\docs\superpowers\plans\2026-11-10-gezige-features.md

Setup: 2026-11-10. 后端 git init 基线 977af9f（main），工作分支 feat/gezige-features。前端 dst-manage-web2 同名分支。Go 1.27.3 (D:\Go) 基线编译 BUILD_OK。
Ruling: sdd-workspace/task-start 脚本在 Windows+计划文件位于非 git 目录（D:\New World\docs）时不可用（cd null directory）— 手动建 workspace 于后端仓库 .superpowers/sdd/<plan>/ — cost if wrong: ledger 路径与脚本约定不一致，仅影响恢复定位。

Pre-flight 共享接口：
- Task 2 produces SafeJoin/Quote → Task 3 consumes（B3/备份路径/注入面）— 一致，无冲突
- Task 1 produces LevelInstance 状态语义（running/lock）→ Task 4 consumes（Start 前钩子）— 一致，无冲突
- Task 3 修 Send 转义 → Task 4/5/7 指令发送共用 — 一致，无冲突

Ruling: TDD 范围 — 纯逻辑（白名单/SafeJoin/解析/Exists/断言安全函数）严格 RED→GREEN；进程时序与并发（A1/A2/B15/B16/B14）用并发压力测试与状态断言验证（-race），不模拟真实游戏进程 — cost if wrong: 进程交互边角依赖最终 review 兜底。
Task 1: complete (commits 977af9f..9966c8d, tests: go test ./... → ok)
Task 1: Ruling: B18 不成立 — robfig/cron v3 New() 默认含 Recover — 危险降级为“定时任务半成品状态”deferred minor — cost if wrong: 极端 cron 版本行为差异再兜一层
Task 1: Ruling: B9.4 采用 uuid|checkType 复合键 + Delete 按 uuid/level_name 多键删 — 根因是三处键语义混用 — cost if wrong: 遗留任务删除遗漏
Task 1: Ruling: A2 测试模拟写者需与产品写者同锁 — 测试裸写 map 曾复现假阳性 — cost if wrong: 无
Task 1: minor (deferred): 定时任务中途 panic 留下“已停服未启动”半成品状态
Task 2: complete (commits 9966c8d..a9467c1, tests: go test ./... → ok 2 pkgs)
Task 2: Ruling: gameService 内部三函数 level 校验正则未匹配（签名差异）— 依赖 API 层 ValidName 入口防线 — cost if wrong: 内部调用路径漏防
Task 2: minor (deferred): Steam API key 仍为常量（可配置化待后续）；A3 shell 参数 Quote 仅接线 helper 与部分入口
Task 3: complete (commits a9467c1..HEAD, tests: go test ./... → ok 2 pkgs)
Task 3: Ruling: C5 注释插入括号内曾致语法错误 — 替换脚本注释位置教训 — cost if wrong: 无
Task 3: minor (deferred): B17 steamcmd 更新命令仍为 cmd 字符串拼接（已加引号）；C4 缺 config.yml 报错信息、C7 arr 长度校验未做
Task 4: complete (healthCheck 10项+API+startBefore 闸门; tests 4/4 ok)
Task 5: complete (EnsureLuaJitModInFile; workshop-3444078585 交叉验证; tests 3/3 ok)
Task 6: complete (GenerateIAPreset overlay|standalone; Shipwrecked 168项模板; tests 3/3 ok)
Task 7: complete (netUtils.ResolveIP 三态+FormatCConnect/SteamURL 外部端口; tests 6/6 ok)
Task 4-7: 前端 GezigeTools 页完成，vite build 通过（独立 commit 在 dst-manage-web2）
Ruling: 前端文案硬编码中文（i18n key 后补）— Global Constraints deviation — cost if wrong: 多语言用户看不到翻译
Ruling: 四功能集中 GezigeTools 页而非分散挂 ShareConfig/System/Level2 — 降低对现有复杂页面的侵入 — cost if wrong: 交互入口与 spec 的页面归属不同
Task 4-7: minor (deferred): System/LuaJIT Tab、ShareConfig 直连卡片迁回、侧栏菜单入口、i18n key
Task 8 (进行中): go build/test/vet 全绿（6 包）；前端 vite build 通过；冒烟：面板启动 /hello 正常，/api/health/check 鉴权 401 生效
Task 8: 全分支审查已派发（9 commits review diff 130KB），等待 reviewer 结果后做修复 pass

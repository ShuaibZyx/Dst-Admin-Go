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

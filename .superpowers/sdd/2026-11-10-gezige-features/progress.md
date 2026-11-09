# SDD ledger — plan: D:\New World\docs\superpowers\plans\2026-11-10-gezige-features.md

Setup: 2026-11-10. 后端 git init 基线 977af9f（main），工作分支 feat/gezige-features。前端 dst-manage-web2 同名分支。Go 1.27.3 (D:\Go) 基线编译 BUILD_OK。
Ruling: sdd-workspace/task-start 脚本在 Windows+计划文件位于非 git 目录（D:\New World\docs）时不可用（cd null directory）— 手动建 workspace 于后端仓库 .superpowers/sdd/<plan>/ — cost if wrong: ledger 路径与脚本约定不一致，仅影响恢复定位。

Pre-flight 共享接口：
- Task 2 produces SafeJoin/Quote → Task 3 consumes（B3/备份路径/注入面）— 一致，无冲突
- Task 1 produces LevelInstance 状态语义（running/lock）→ Task 4 consumes（Start 前钩子）— 一致，无冲突
- Task 3 修 Send 转义 → Task 4/5/7 指令发送共用 — 一致，无冲突

Ruling: TDD 范围 — 纯逻辑（白名单/SafeJoin/解析/Exists/断言安全函数）严格 RED→GREEN；进程时序与并发（A1/A2/B15/B16/B14）用并发压力测试与状态断言验证（-race），不模拟真实游戏进程 — cost if wrong: 进程交互边角依赖最终 review 兜底。

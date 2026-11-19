# Dst-Admin-Go

饥荒联机版（Don't Starve Together）专用服务器管理面板——后端（Go + Gin）。

## 功能

- 世界管理：启动/停止/重启、状态监控、日志实时流、启动进度
- 模组管理：订阅/校验/更新/删除、统一模组目录、LuaJIT 模组自动启用
- 多存档：创建/切换/备份/回档、上传/搬入、大小统计、批量删除、打开所在文件夹
- 世界配置：可视化定制（世界规则/世界生成/模组/端口）、世界模板、IA 模组世界（海难/火山）
- 房间设置、玩家管理（名单/封禁/传送/背包）、系统设置、定时任务

## 快速开始

```bash
go run main.go
```

面板默认监听 `8082`（端口等配置见 `config.yml`）。前端构建产物放 `dist/` 目录由后端托管。

## 配置

- `dst_config`：根目录、steamcmd、服务端、模组目录等路径配置
- `password.txt`：面板登录凭据（username=/password=/displayName=/photoURL=）

## License

MIT

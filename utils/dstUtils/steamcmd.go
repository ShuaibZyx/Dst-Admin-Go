package dstUtils

import "dst-admin-go/utils/shellUtils"

// BuildSteamCmdInstallCmd 构建 steamcmd 安装/更新 DST 的 Linux 命令（sh -c）。
// 路径经 shellUtils.Quote 单引号引用：防 shell 注入、防含空格路径断裂（B17）。
func BuildSteamCmdInstallCmd(steamCmd, installDir string) string {
	return "cd " + shellUtils.Quote(steamCmd) +
		" ; ./steamcmd.sh +login anonymous +force_install_dir " + shellUtils.Quote(installDir) +
		" +app_update 343050 validate +quit"
}

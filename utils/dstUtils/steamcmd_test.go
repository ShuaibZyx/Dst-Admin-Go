package dstUtils

import (
	"strings"
	"testing"
)

// B17: steamcmd 安装命令（Linux sh -c）路径必须引用——防注入、防含空格路径断裂。

func TestBuildSteamCmdInstallCmdQuotesPaths(t *testing.T) {
	cmd := BuildSteamCmdInstallCmd("/opt/steam cmd", "/srv/my dir")
	if !strings.Contains(cmd, "'/opt/steam cmd'") {
		t.Fatalf("steamCmd 路径(含空格)未被单引号引用: %q", cmd)
	}
	if !strings.Contains(cmd, "'/srv/my dir'") {
		t.Fatalf("installDir 路径(含空格)未被单引号引用: %q", cmd)
	}
}

func TestBuildSteamCmdInstallCmdContainsMetacharsAreQuoted(t *testing.T) {
	cmd := BuildSteamCmdInstallCmd("/x; reboot", "/y$(id)")
	// 元字符须被整体单引号包裹，作为字面量而非 shell 语法
	if !strings.Contains(cmd, `'/x; reboot'`) {
		t.Fatalf("steamCmd 注入元字符未被引用隔离: %q", cmd)
	}
	if !strings.Contains(cmd, `'/y$(id)'`) {
		t.Fatalf("installDir 注入元字符未被引用隔离: %q", cmd)
	}
}

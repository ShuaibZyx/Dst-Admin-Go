package shellUtils

import (
	"runtime"
	"strings"
	"testing"
)

// Shell 必须按平台分发到对应执行器:
// Windows 走 cmd /C(能展开 %OS%),unix 走 sh -c(能展开 $HOME)。
// 交叉验证:在任意平台,平台无关字面量不应被"对侧执行器"语义解释。
func TestShell_Dispatches(t *testing.T) {
	out, err := Shell("echo dispatch-ok")
	if err != nil {
		t.Fatalf("Shell 执行失败: %v", err)
	}
	if !strings.Contains(out, "dispatch-ok") {
		t.Fatalf("输出应含 dispatch-ok, 实际: %q", out)
	}

	if runtime.GOOS == "windows" {
		out, err := Shell("echo %OS%")
		if err != nil {
			t.Fatalf("Shell 执行失败: %v", err)
		}
		// 走 cmd 会展开 %OS%;误走 sh 则输出字面 %OS%
		if strings.Contains(out, "%OS%") {
			t.Fatalf("Windows 上 Shell 应经 cmd /C(展开 %%OS%%), 实际输出: %q", out)
		}
	}
}

package shellUtils

import (
	"strings"
	"testing"
)

// C-2: 用户输入进 screen stuff 前必须过 Lua+shell 双层转义
func TestEscapeForStuff(t *testing.T) {
	got := EscapeForStuff("x\") c_shutdown(true) `id` $(id) $HOME")
	// 转义后不得残留任何裸的 shell 元字符或未转义引号
	if strings.Contains(got, "`") && !strings.Contains(got, "\\`") {
		t.Fatalf("反引号未转义: %q", got)
	}
	if strings.Contains(got, "$") && !strings.Contains(got, "\\$") {
		t.Fatalf("$ 未转义: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("换行必须清除: %q", got)
	}
	// 输出中每个 " 都应带反斜杠
	for i := 0; i < len(got); i++ {
		if got[i] == '"' && (i == 0 || got[i-1] != '\\') {
			t.Fatalf("存在未转义引号: %q", got)
		}
	}
}

// C-2: 管理员命令仅做 shell 双引号转义（Lua 语义保留）
func TestEscapeShellDQ(t *testing.T) {
	got := EscapeShellDQ(`c_announce("hi")`)
	if strings.Contains(got, "\"") && !strings.Contains(got, "\\\"") {
		t.Fatalf("引号未转义: %q", got)
	}
	// Lua 反斜杠语义保留（\\ 仍在）
	if !strings.Contains(got, `\"`) {
		t.Fatalf("应保留可被 shell 还原的转义引号: %q", got)
	}
}

// M-6: EscapeLuaStr 逐项断言（引号/反斜杠/单引号/换行四维度）
func TestEscapeLuaStrTable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a" + string(rune(34)) + "b", "a" + string(rune(92)) + string(rune(34)) + "b"},
		{string(rune(92)), string(rune(92)) + string(rune(92))},
		{"a" + string(rune(39)) + "b", "a" + string(rune(92)) + string(rune(39)) + "b"},
		{"a" + string(rune(10)) + "b", "a b"},
	}
	for _, c := range cases {
		if got := EscapeLuaStr(c.in); got != c.want {
			t.Fatalf("EscapeLuaStr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

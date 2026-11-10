package shellUtils

import "strings"

// EscapeShellDQ 将字符串转义为可安全嵌入 sh 双引号参数的内容（C-2: 防 shell 逃逸）。
// 仅处理 shell 层，不改变命令的 Lua 语义。
func EscapeShellDQ(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// EscapeForStuff 用户输入进 screen stuff 的双层转义（C-2）：
// 先 Lua 字符串转义（EscapeLuaStr），再 shell 双引号转义。
// shell 层剥离后 Lua 收到带转义引号的内容，两层语义闭合。
func EscapeForStuff(s string) string {
	return EscapeShellDQ(EscapeLuaStr(s))
}

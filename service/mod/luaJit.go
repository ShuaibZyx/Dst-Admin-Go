// LuaJIT 模组开关（fesily/dontstarveluajit2，workshop-3444078585）
// 对应鸽子鸽 lv.123-124 行为：启用后强制写入各世界 modoverrides.lua
package mod

import (
	"os"
	"strings"
)

const LuaJitModId = "workshop-3444078585"

// findModBlock 按括号配平定位 ["id"] = { ... } 块（M-1：跨嵌套、不误匹配相邻 mod）
func findModBlock(s string) (start, end int, ok bool) {
	idx := strings.Index(s, `["`+LuaJitModId+`"]`)
	if idx < 0 {
		return 0, 0, false
	}
	brace := strings.Index(s[idx:], "{")
	if brace < 0 {
		return 0, 0, false
	}
	start = idx
	i := idx + brace
	depth := 0
	inStr := byte(0)
	for ; i < len(s); i++ {
		c := s[i]
		if inStr != 0 {
			if c == '\\' {
				i++
			} else if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return start, i + 1, true
			}
		}
	}
	return 0, 0, false
}

// EnsureLuaJitModInFile 确保 modoverrides.lua 中 luajit 模组处于指定启用状态。
// 文件不存在时创建最小模板；已存在其它模组配置时只增改目标条目。
func EnsureLuaJitModInFile(path string, enabled bool) error {
	want := "false"
	if enabled {
		want = "true"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		content := "return {\n  [\"" + LuaJitModId + "\"] = { enabled = " + want + " },\n}\n"
		return os.WriteFile(path, []byte(content), 0644)
	}
	s := string(b)
	start, end, found := findModBlock(s)
	if found {
		block := s[start:end]
		if i := strings.LastIndex(block, "enabled"); i >= 0 {
			// 块内改值（M-1：绝不碰相邻 mod）
			rest := block[i:]
			eq := strings.Index(rest, "=")
			if eq >= 0 {
				valStart := i + eq + 1
				valEnd := valStart
				for valEnd < len(block) && (block[valEnd] == ' ' || block[valEnd] == '\t') {
					valEnd++
				}
				for valEnd < len(block) && block[valEnd] != ' ' && block[valEnd] != ',' && block[valEnd] != '}' && block[valEnd] != '\n' {
					valEnd++
				}
				block = block[:valStart] + " " + want + block[valEnd:]
			}
		} else {
			inner := strings.Index(block, "{")
			block = block[:inner+1] + " enabled = " + want + "," + block[inner+1:]
		}
		s = s[:start] + block + s[end:]
	} else {
		entry := "  [\"" + LuaJitModId + "\"] = { enabled = " + want + " },\n"
		if idx := strings.Index(s, "return"); idx >= 0 {
			if brace := strings.Index(s[idx:], "{"); brace >= 0 {
				pos := idx + brace + 1
				s = s[:pos] + "\n" + entry + s[pos:]
			} else {
				s = s + "\nreturn {\n" + entry + "}\n"
			}
		} else {
			s = "return {\n" + entry + "}\n" + s
		}
	}
	return os.WriteFile(path, []byte(s), 0644)
}

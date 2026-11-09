// LuaJIT 模组开关（fesily/dontstarveluajit2，workshop-3444078585）
// 对应鸽子鸽 lv.123-124 行为：启用后强制写入各世界 modoverrides.lua
package mod

import (
	"os"
	"regexp"
	"strings"
)

const LuaJitModId = "workshop-3444078585"

var (
	reLuaJitEnabled = regexp.MustCompile(`(?s)(\["` + regexp.QuoteMeta(LuaJitModId) + `"\]\s*=\s*\{[^}]*?enabled\s*=\s*)(true|false)`)
	reLuaJitBlock   = regexp.MustCompile(`\["` + regexp.QuoteMeta(LuaJitModId) + `"\]\s*=\s*\{[^}]*?\}`)
)

// EnsureLuaJitModInFile 确保 modoverrides.lua 中 luajit 模组处于指定启用状态。
// 文件不存在时创建最小模板；已存在其它模组配置时只增改目标条目。
func EnsureLuaJitModInFile(path string, enabled bool) error {
	want := "false"
	if enabled {
		want = "true"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		// 不存在则创建
		content := "return {\n  [\"" + LuaJitModId + "\"] = { enabled = " + want + " },\n}\n"
		return os.WriteFile(path, []byte(content), 0644)
	}
	s := string(b)
	if reLuaJitBlock.MatchString(s) {
		if reLuaJitEnabled.MatchString(s) {
			s = reLuaJitEnabled.ReplaceAllString(s, "${1}"+want)
		} else {
			// 条目存在但没有 enabled 字段：在块内插入
			s = reLuaJitBlock.ReplaceAllStringFunc(s, func(m string) string {
				return strings.Replace(m, "{", "{ enabled = "+want+",", 1)
			})
		}
	} else {
		// 追加条目：插到 return { 后
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


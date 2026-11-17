package dstConfigUtils

import (
	"path/filepath"
	"testing"
)

// 新语义（不兼容旧配置）：一切输入都当作相对父级的子路径，强制落在父级文件夹下
func TestResolvePath(t *testing.T) {
	cases := []struct {
		name, parent, p, def, want string
	}{
		{"相对拼父级", `D:\DST`, "steamcmd", "steamcmd", filepath.Join(`D:\DST`, "steamcmd")},
		{"绝对路径剥成相对", `D:\DST`, `C:\other\backup`, "backup", filepath.Join(`D:\DST`, "other", "backup")},
		{"home 前缀剥掉", `D:\DST`, "~/steamcmd", "steamcmd", filepath.Join(`D:\DST`, "steamcmd")},
		{"空值取默认", `D:\DST`, "", "mods", filepath.Join(`D:\DST`, "mods")},
		{"空值空默认", `D:\DST`, "", "", ""},
		{"parent 空返回空", "", "steamcmd", "steamcmd", ""},
		{"含空格父级", "/opt/dst parent", "steamcmd", "steamcmd", filepath.Join("/opt/dst parent", "steamcmd")},
		{"多级相对", "/p", "a/b", "x", filepath.Join("/p", "a", "b")},
		{"旧单数别名归一", `D:\DST`, "world", "worlds", filepath.Join(`D:\DST`, "worlds")},
		{"旧单数别名归一mod", `D:\DST`, "mod", "mods", filepath.Join(`D:\DST`, "mods")},
		{"旧单数别名归一backup", `D:\DST`, "backup", "backups", filepath.Join(`D:\DST`, "backups")},
	}
	for _, c := range cases {
		if got := ResolvePath(c.parent, c.p, c.def); got != c.want {
			t.Errorf("%s: ResolvePath(%q,%q,%q) = %q, want %q", c.name, c.parent, c.p, c.def, got, c.want)
		}
	}
}

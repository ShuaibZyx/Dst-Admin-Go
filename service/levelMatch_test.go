package service

import "testing"

func TestMatchLevelCmdline(t *testing.T) {
	cases := []struct {
		name    string
		cmdline []string
		cluster string
		level   string
		want    bool
	}{
		{"匹配", []string{`C:\dst\bin64\dontstarve_dedicated_server_nullrenderer_x64.exe`, "-console", "-cluster", "ROGIA", "-shard", "Shipwrecked"}, "ROGIA", "Shipwrecked", true},
		{"shard 不符", []string{`C:\dst\dontstarve_dedicated_server_nullrenderer.exe`, "-cluster", "ROGIA", "-shard", "Master"}, "ROGIA", "Volcano", false},
		{"cluster 不符", []string{`C:\dst\dontstarve_dedicated_server_nullrenderer.exe`, "-cluster", "OTHER", "-shard", "Master"}, "ROGIA", "Master", false},
		{"非游戏进程", []string{"notepad.exe", "-cluster", "ROGIA", "-shard", "Master"}, "ROGIA", "Master", false},
		// box86/box64 模拟启动:真身在第二个参数,旧实现只看首个参数恒不匹配
		{"box64 前缀", []string{"box64", "./dontstarve_dedicated_server_nullrenderer_x64", "-console", "-cluster", "ROGIA", "-shard", "Master"}, "ROGIA", "Master", true},
		{"box86 前缀", []string{"box86", "./dontstarve_dedicated_server_nullrenderer_x64", "-console", "-cluster", "ROGIA", "-shard", "Caves"}, "ROGIA", "Caves", true},
		{"box64 前缀但 shard 不符", []string{"box64", "./dontstarve_dedicated_server_nullrenderer_x64", "-cluster", "ROGIA", "-shard", "Master"}, "ROGIA", "Volcano", false},
		{"模拟器前缀但非游戏进程", []string{"box64", "some_other_program", "-cluster", "ROGIA", "-shard", "Master"}, "ROGIA", "Master", false},
	}
	for _, c := range cases {
		if got := matchLevelCmdline(c.cmdline, c.cluster, c.level); got != c.want {
			t.Errorf("%s: matchLevelCmdline = %v, want %v", c.name, got, c.want)
		}
	}
}

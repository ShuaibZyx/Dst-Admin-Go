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
	}
	for _, c := range cases {
		if got := matchLevelCmdline(c.cmdline, c.cluster, c.level); got != c.want {
			t.Errorf("%s: matchLevelCmdline = %v, want %v", c.name, got, c.want)
		}
	}
}

package dstConfigUtils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		in, want string
	}{
		{"~/steamcmd", filepath.Join(home, "steamcmd")},
		{"~", home},
		{`~\steamcmd`, filepath.Join(home, "steamcmd")},
		{"/data/x", "/data/x"},
		{"steamcmd", "steamcmd"},
		{"~other/x", "~other/x"}, // 不是 ~ / ~/ / ~\ 形式，不展开
	}
	for _, c := range cases {
		if got := ExpandHome(c.in); got != c.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolvePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, parent, p, def, want string
	}{
		{"相对路径拼父级", `D:\DST`, "steamcmd", "steamcmd", filepath.Join(`D:\DST`, "steamcmd")},
		{"绝对路径不变", `D:\DST`, `C:\other\backup`, "backup", `C:\other\backup`},
		{"home 前缀展开", `D:\DST`, "~/steamcmd", "steamcmd", filepath.Join(home, "steamcmd")},
		{"空值取默认", `D:\DST`, "", "mod", filepath.Join(`D:\DST`, "mod")},
		{"parent 空原样", "", "steamcmd", "steamcmd", "steamcmd"},
		{"parent 空且值空不取默认", "", "", "mod", ""},
		{"空值空默认", `D:\DST`, "", "", ""},
		{"含空格父级", "/opt/dst parent", "mod", "mod", filepath.Join("/opt/dst parent", "mod")},
	}
	for _, c := range cases {
		if got := ResolvePath(c.parent, c.p, c.def); got != c.want {
			t.Errorf("%s: ResolvePath(%q,%q,%q) = %q, want %q", c.name, c.parent, c.p, c.def, got, c.want)
		}
	}
}

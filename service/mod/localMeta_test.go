package mod

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func TestParseModinfoName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "modinfo.lua")
	content := "name = \"Generic Name\"\nname.zh = \"中文名字\"\nversion = \"1.0\"\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if got := parseModinfoName(p); got != "中文名字" {
		t.Fatalf("name = %q, want 中文名字", got)
	}

	p2 := filepath.Join(t.TempDir(), "modinfo.lua")
	if err := os.WriteFile(p2, []byte("name = 'Only English'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := parseModinfoName(p2); got != "Only English" {
		t.Fatalf("name = %q, want Only English", got)
	}

	if got := parseModinfoName(filepath.Join(t.TempDir(), "missing.lua")); got != "" {
		t.Fatalf("missing file name = %q, want empty", got)
	}
}

func TestLocalModMetaFromUgc(t *testing.T) {
	parent := t.TempDir()
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\nforce_install_dir=dst-dedicated-server\ncluster=MyDediServer\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	// UGC 布局：<server>/ugc_mods/MyDediServer/Master/content/322330/<id>/
	modDir := filepath.Join(parent, "dst-dedicated-server", "ugc_mods", "MyDediServer", "Master", "content", "322330", "12345")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "modinfo.lua"), []byte("name = \"Hello Mod\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "icon.png"), []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	name, icon := LocalModMeta("12345")
	if name != "Hello Mod" {
		t.Fatalf("name = %q, want Hello Mod", name)
	}
	if icon == "" {
		t.Fatal("图标路径应找到")
	}
}

package levelConfigUtils

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func TestGetLevelConfigRegistersAllShardDirs(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "worlds", "ROGIA")
	// 磁盘上四个世界目录（Master/Caves + 两个自定义）
	for _, name := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name, "server.ini"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\npersistent_storage_root=worlds\ncluster=ROGIA\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	config, err := GetLevelConfig("ROGIA")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range config.LevelList {
		got[it.File] = it.Name
	}
	for _, want := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
		if _, ok := got[want]; !ok {
			t.Errorf("缺世界 %s: %+v", want, config.LevelList)
		}
	}
	if len(config.LevelList) != 4 {
		t.Errorf("LevelList = %+v, want 4 项", config.LevelList)
	}
	// 显示名为中文
	if got["Shipwrecked"] != "海难" || got["Volcano"] != "火山" {
		t.Errorf("显示名未中文化: %+v", config.LevelList)
	}
	if got["Master"] != "森林" || got["Caves"] != "洞穴" {
		t.Errorf("Master/Caves 显示名 = %q/%q", got["Master"], got["Caves"])
	}
}

func TestGetLevelConfigTranslatesExistingEnglishNames(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "worlds", "ROGIA")
	for _, name := range []string{"Shipwrecked", "Volcano"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name, "server.ini"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// 存量 level.json 里是英文目录名
	if err := os.WriteFile(filepath.Join(base, "level.json"),
		[]byte(`{"levelList":[{"name":"Shipwrecked","file":"Shipwrecked"},{"name":"Volcano","file":"Volcano"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\npersistent_storage_root=worlds\ncluster=ROGIA\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	config, err := GetLevelConfig("ROGIA")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range config.LevelList {
		got[it.File] = it.Name
	}
	if got["Shipwrecked"] != "海难" || got["Volcano"] != "火山" {
		t.Fatalf("存量英文名未翻译: %+v", config.LevelList)
	}
}

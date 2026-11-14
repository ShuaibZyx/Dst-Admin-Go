package service

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func setupSaveTest(t *testing.T) (parent string) {
	t.Helper()
	parent = t.TempDir()
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\npersistent_storage_root=world\ncluster=Alpha\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)
	return parent
}

func mkCluster(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "Master"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cluster.ini"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectAndMoveSaves(t *testing.T) {
	setupSaveTest(t)
	oldRoots := saveScanRoots
	ext := t.TempDir()
	saveScanRoots = func() []string { return []string{ext} }
	t.Cleanup(func() { saveScanRoots = oldRoots })

	// 外部存档（Klei 布局）与一个非存档目录
	mkCluster(t, filepath.Join(ext, "DoNotStarveTogether", "OldSave"))
	mkCluster(t, filepath.Join(ext, "DoNotStarveTogether", "Alpha"))
	if err := os.MkdirAll(filepath.Join(ext, "DoNotStarveTogether", "backup"), 0755); err != nil {
		t.Fatal(err)
	}

	found := DetectSaves()
	names := map[string]SaveInfo{}
	for _, s := range found {
		names[s.Name] = s
	}
	if _, ok := names["OldSave"]; !ok {
		t.Fatalf("未检测到 OldSave: %+v", found)
	}
	if _, ok := names["backup"]; ok {
		t.Fatalf("杂物目录不应出现: %+v", found)
	}

	// 搬家：只搬 OldSave
	res := MoveSaves([]string{names["OldSave"].Path})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("搬家失败: %+v", res)
	}
	list := ListSaves()
	got := map[string]bool{}
	for _, s := range list {
		got[s.Name] = true
	}
	if !got["OldSave"] {
		t.Fatalf("搬入后列表缺 OldSave: %+v", list)
	}

	// 同名冲突拒绝
	mkCluster(t, filepath.Join(ext, "DoNotStarveTogether", "OldSave"))
	found2 := DetectSaves()
	var old2 string
	for _, s := range found2 {
		if s.Name == "OldSave" && !s.InPlace {
			old2 = s.Path
		}
	}
	res2 := MoveSaves([]string{old2})
	if res2[0].OK {
		t.Fatalf("同名应拒绝: %+v", res2)
	}
}

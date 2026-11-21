package service

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupPathResolvesUnderParent(t *testing.T) {
	parent := t.TempDir()
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\ncluster=Cluster1\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	want := filepath.Join(parent, "backups")
	if err := os.MkdirAll(want, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("backupPath panic: %v", r)
		}
	}()
	got := (&BackupService{}).backupPath()
	if got != want {
		t.Fatalf("backupPath() = %q, want %q", got, want)
	}
}

// sumMd5 必须纯 Go 跨平台:旧实现走 find|md5sum 管道,Windows 无 sh、macOS 无 md5sum,
// 恒返回 "" 后被"两次md5相等"误判跳过——快照备份在 Windows/macOS 上一次都建不出来。
func TestSumMd5PortableAndContentSensitive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "save")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.sav"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	h1 := sumMd5(dir)
	if h1 == "" {
		t.Fatal("sumMd5 不应返回空串(空串会让快照备份恒被判'未变化'而跳过)")
	}
	if h2 := sumMd5(dir); h2 != h1 {
		t.Fatalf("内容未变指纹应稳定: %q != %q", h2, h1)
	}

	if err := os.WriteFile(filepath.Join(sub, "a.sav"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	if h3 := sumMd5(dir); h3 == h1 {
		t.Fatal("内容变化后指纹必须变化")
	}

	if got := sumMd5(filepath.Join(dir, "no-such-dir")); got != "" {
		t.Fatalf("不存在的目录应返回空串, got %q", got)
	}
}

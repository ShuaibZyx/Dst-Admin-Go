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

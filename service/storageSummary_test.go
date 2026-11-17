package service

import (
	"os"
	"path/filepath"
	"testing"

	"dst-admin-go/vo"
)

// 存储概要:世界存档大小 + 备份数量/总大小(信息卡展示)。

func TestDirSize(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Master", "save"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, "Master", "save", "a.dat"),
		filepath.Join(root, "Master", "server_log.txt"),
	} {
		if err := os.WriteFile(p, make([]byte, 100), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := DirSize(root); got != 200 {
		t.Fatalf("DirSize 应为 200, 实际 %d", got)
	}
	if got := DirSize(filepath.Join(root, "not-exist")); got != 0 {
		t.Fatalf("不存在的目录应为 0, 实际 %d", got)
	}
}

func TestSumBackups(t *testing.T) {
	backups := []vo.BackupVo{
		{FileName: "a.zip", FileSize: 100},
		{FileName: "b.zip", FileSize: 250},
	}
	count, bytes := SumBackups(backups)
	if count != 2 || bytes != 350 {
		t.Fatalf("应为 2 个 / 350 字节, 实际 %d 个 / %d 字节", count, bytes)
	}
	count, bytes = SumBackups(nil)
	if count != 0 || bytes != 0 {
		t.Fatalf("空列表应为 0/0, 实际 %d/%d", count, bytes)
	}
}

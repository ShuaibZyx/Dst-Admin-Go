package service

import (
	"os"
	"path/filepath"
	"testing"
)

// 日志增量读：offset 之后的新行；文件截断/轮转时从头重读（offset 归 0）。
// fsnotify 事件驱动的推送全部走这里（替代每连接 2 秒轮询 stat+read）。
func TestReadLogIncrement(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server_log.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	lines, off := ReadLogIncrement(p, 0)
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Fatalf("全量读 = %v", lines)
	}

	// 追加 → 只读增量
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0644)
	_, _ = f.WriteString("line3\n")
	_ = f.Close()
	lines, off2 := ReadLogIncrement(p, off)
	if len(lines) != 1 || lines[0] != "line3" {
		t.Fatalf("增量读 = %v", lines)
	}

	// 截断（轮转）→ 从头重读
	if err := os.WriteFile(p, []byte("new1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	lines, _ = ReadLogIncrement(p, off2)
	if len(lines) != 1 || lines[0] != "new1" {
		t.Fatalf("轮转后应从头重读, got %v", lines)
	}
}

package mod

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// RunSteamcmdStream 必须在进程退出后正常返回（历史上曾因管道写端未关而悬挂）
func TestRunSteamcmdStreamReturns(t *testing.T) {
	dir := `D:\DST-Server\steamcmd`
	if _, err := os.Stat(filepath.Join(dir, "steamcmd.exe")); err != nil {
		t.Skip("本机无 steamcmd，跳过")
	}
	done := make(chan error, 1)
	go func() {
		done <- RunSteamcmdStream(dir, []string{"+quit"}, func(string) {})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunSteamcmdStream 返回错误: %v", err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("RunSteamcmdStream 悬挂未返回（管道死锁回归）")
	}
}

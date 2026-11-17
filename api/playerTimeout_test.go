package api

import (
	"testing"
	"time"
)

// 玩家查询带 2 秒兜底超时:游戏命令失败(如 attempt to call a nil value)时
// GetPlayerList 死等内部 10 秒超时,多组件轮询 × 长阻塞占满浏览器连接池,
// 一启动世界整个面板卡死。API 层 2 秒兜底先返回。
func TestPlayerListTimeoutFastReturn(t *testing.T) {
	start := time.Now()
	list := playerListWithTimeout("NoSuchCluster", "NoSuchLevel")
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("应在 2 秒内返回, got %v", elapsed)
	}
	if list == nil {
		t.Fatal("应返回空列表而非 nil")
	}
}

package api

import "testing"

// 启停异步化：API 立即返回,同世界操作防重入(启动中禁止再启动/停止,避免打架)。
func TestLevelOpMutex(t *testing.T) {
	if !beginLevelOp("cl/x", "start") {
		t.Fatal("首次应获取操作权")
	}
	if beginLevelOp("cl/x", "start") {
		t.Fatal("进行中重复启动应拒绝")
	}
	if beginLevelOp("cl/x", "stop") {
		t.Fatal("启动中应拒绝停止")
	}
	endLevelOp("cl/x")
	if !beginLevelOp("cl/x", "stop") {
		t.Fatal("释放后应可停止")
	}
	endLevelOp("cl/x")
	// 不同世界互不影响
	if !beginLevelOp("cl/y", "start") {
		t.Fatal("其他世界不受影响")
	}
	endLevelOp("cl/y")
}

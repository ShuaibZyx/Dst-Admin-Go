package service

import "testing"

// SSE 状态推送去重:同载荷不重复推送(2s 心跳帧带 Lua 全文约 20KB,重复推送纯浪费+挤渲染)。
func TestChangeSender(t *testing.T) {
	s := &ChangeSender{}
	if !s.ShouldPush("payload-a") {
		t.Fatal("首帧必须推送")
	}
	if s.ShouldPush("payload-a") {
		t.Fatal("相同载荷不得重复推送")
	}
	if !s.ShouldPush("payload-b") {
		t.Fatal("载荷变化必须推送")
	}
	if s.ShouldPush("payload-b") {
		t.Fatal("重复判定需持续生效")
	}
}

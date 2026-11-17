package service

import (
	"fmt"
	"testing"
	"time"
)

// LiveSession 的历史积压必须有上限:否则每次重连全量倾泻(数百帧/数MB),
// 挤爆前端渲染(面板顶部数值数秒才出),且内存只增不减。
func TestLiveSession_BacklogCapped(t *testing.T) {
	s := NewLiveSession()
	done := make(chan struct{})
	s.Start(func(send func(string)) {
		for i := 0; i < 300; i++ {
			send(fmt.Sprintf("line-%d", i))
		}
		close(done)
	})
	<-done
	time.Sleep(50 * time.Millisecond) // 等 defer 清理完成(通道关闭)

	// 重新开一轮再订阅,验证 backlog 上限
	s2 := NewLiveSession()
	done2 := make(chan struct{})
	s2.Start(func(send func(string)) {
		for i := 0; i < 300; i++ {
			send(fmt.Sprintf("line-%d", i))
		}
		close(done2)
	})
	<-done2

	// 立即 Attach(会话已结束则仅历史)或运行中订阅后收集:统一通过重开验证
	s3 := NewLiveSession()
	go s3.Start(func(send func(string)) {
		for i := 0; i < 300; i++ {
			send(fmt.Sprintf("line-%d", i))
		}
	})
	time.Sleep(200 * time.Millisecond)
	backlog, ch, unsub := s3.Attach()
	unsub()
	_ = ch

	if len(backlog) > 50 {
		t.Fatalf("backlog 应截断至 ≤50 条, 实际 %d 条", len(backlog))
	}
	if len(backlog) == 0 {
		t.Fatalf("backlog 不应为空")
	}
	// 必须保留最新的行(丢旧保新)
	last := backlog[len(backlog)-1]
	if last != "line-299" {
		t.Fatalf("backlog 应保留最新行, 实际末尾: %q", last)
	}
}

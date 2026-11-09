package service

import (
	"sync"
	"testing"
	"time"
)

// A1: ClusterContainer 的 map 并发读写不得崩溃（concurrent map read/write 是 fatal，不可 recover）
func TestClusterContainerConcurrentAccess(t *testing.T) {
	c := NewClusterContainer()
	key := "cl_Master"
	// 模拟 StartLevel 的写路径（直接写 map，不起真实游戏进程）
	c.container[key] = NewLevelInstance("cl", "Master", 32, "", "", "", "", "")

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(5)
		go func() { defer wg.Done(); c.Status("cl", "Master") }()
		go func() { defer wg.Done(); c.Send("cl", "Master", "c_save()") }()
		go func() { defer wg.Done(); c.MemUsage("cl", "Master") }()
		go func() { defer wg.Done(); c.CpuUsage("cl", "Master") }()
		go func() {
			defer wg.Done()
			c.Remove("cl", "Master")
			// 与 StartLevel 相同的加锁写路径（测试模拟写者）
			c.lock.Lock()
			c.container[key] = NewLevelInstance("cl", "Master", 32, "", "", "", "", "")
			c.lock.Unlock()
		}()
	}
	wg.Wait()
}

// A2: Start() 失败路径不得永久持锁（日志文件创建失败后，后续 Start/Stop 不能死锁）
func TestStartLogCreateFailureDoesNotDeadlock(t *testing.T) {
	// levelName 含非法文件名字符，使 os.Create 失败
	inst := NewLevelInstance("cl", "Mas:ter", 32, "no-such-steamcmd", "no-such-install", "", "", "")
	done := make(chan struct{})
	go func() {
		inst.Start()
		close(done)
	}()
	select {
	case <-done:
		// Start 返回即未死锁
	case <-time.After(5 * time.Second):
		t.Fatal("Start() 在日志创建失败后未返回——锁被永久持有(A2)")
	}
	if inst.Status() {
		t.Fatal("Start() 失败后 running 仍为 true(A2)")
	}
}

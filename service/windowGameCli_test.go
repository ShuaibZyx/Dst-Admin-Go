package service

import (
	"bytes"
	"dst-admin-go/utils/shellUtils"
	"strings"
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

type bufWC struct{ b *bytes.Buffer }

func (w bufWC) Write(p []byte) (int, error) { return w.b.Write(p) }
func (w bufWC) Close() error                { return nil }

// I1: Send 原样透传 + 换行
func TestSendPassthrough(t *testing.T) {
	buf := &bytes.Buffer{}
	inst := &LevelInstance{stdin: bufWC{buf}}
	if err := inst.Send("c_announce(\"hi\")"); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "c_announce(\"hi\")\n" {
		t.Fatalf("Send 应原样透传，实际 %q", buf.String())
	}
}

// I1: EscapeLuaStr 阻断注入
func TestEscapeLuaStr(t *testing.T) {
	got := shellUtils.EscapeLuaStr("a") + shellUtils.EscapeLuaStr("\"") + shellUtils.EscapeLuaStr("'") + shellUtils.EscapeLuaStr("x\ny")
	if strings.ContainsAny(got, "\n\r") {
		t.Fatalf("换行应被移除: %q", got)
	}
	if !strings.Contains(got, "\\\"") {
		t.Fatalf("引号应被转义: %q", got)
	}
}

// M-6: Send 不得吞反斜杠（Windows 路径场景），旧全局删反斜杠会破坏
func TestSendKeepsBackslashes(t *testing.T) {
	buf := &bytes.Buffer{}
	inst := &LevelInstance{stdin: bufWC{buf}}
	if err := inst.Send("c_announce(\"C:\\new\")"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "C:\\new") {
		t.Fatalf("反斜杠必须保留，实际 %q", buf.String())
	}
}

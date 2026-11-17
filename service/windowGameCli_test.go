package service

import (
	"bytes"
	"dst-admin-go/utils/shellUtils"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// CPU 使用率查询不得以秒级采样阻塞:世界运行时 status 轮询每个请求卡 1 秒,
// 前端 3 秒轮询 × 多组件 → HTTP/1.1 连接池占满 → 面板整体卡顿("一启动世界就非常卡")。
// 世界停止时进程不存在、查询立即失败返回,所以只有世界运行时才复现。
func TestCpuByPidNotBlockedBySampling(t *testing.T) {
	pid := int32(os.Getpid())
	cpuByPid(pid) // 预热内部采样基准
	start := time.Now()
	cpuByPid(pid)
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("cpuByPid 阻塞 %v, 应瞬间返回(interval=0 用差分采样)", elapsed)
	}
}

func TestGetProcessCpuInfoNotBlockedBySampling(t *testing.T) {
	inst := &LevelInstance{pid: os.Getpid()}
	inst.GetProcessCpuInfo() // 预热
	start := time.Now()
	inst.GetProcessCpuInfo()
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("GetProcessCpuInfo 阻塞 %v, 应瞬间返回", elapsed)
	}
}

// ---- 进程运行时长:前端日志面板"进程运行时长:{{elapsed}}"依赖 Ps.elapsed,
// 缺失时世界明明在跑却显示"未运行/未知" ----

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45 秒"},
		{3*time.Minute + 12*time.Second, "3 分 12 秒"},
		{2*time.Hour + 15*time.Minute, "2 小时 15 分"},
		{26*time.Hour + 5*time.Minute, "1 天 2 小时"},
	}
	for _, c := range cases {
		if got := formatElapsed(c.d); got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestElapsedByPid(t *testing.T) {
	if got := elapsedByPid(int32(os.Getpid())); got == "" {
		t.Fatal("当前进程应返回运行时长")
	}
	if got := elapsedByPid(-1); got != "" {
		t.Fatalf("无效 pid 应返回空, got %q", got)
	}
}

func TestClusterContainerElapsed(t *testing.T) {
	c := NewClusterContainer()
	c.container[levelKey("cl1", "Master")] = &LevelInstance{pid: os.Getpid()}
	if got := c.Elapsed("cl1", "Master"); got == "" {
		t.Fatal("运行中的世界应返回运行时长")
	}
	if got := c.Elapsed("cl1", "NoSuch"); got != "" {
		t.Fatalf("无进程的世界应返回空, got %q", got)
	}
}

// 面板重启后控制台句柄丢失,停止退化为进程终止(游戏日志缓冲尾部丢失):
// 必须打点告知用户,停止过程不再是黑盒。
func TestStopLevelFallbackWarnsNoHandle(t *testing.T) {
	c := NewClusterContainer()
	before := len(StartLog.lines) // 同包直读
	c.StopLevel("cl1", "NoHandleWorld")
	joined := strings.Join(StartLog.lines[before:], "\n")
	if !strings.Contains(joined, "无控制台句柄") {
		t.Fatalf("fallback 停止应打点提示句柄丢失(停止瞬间日志可能丢失), got %q", joined)
	}
}

// gopsutil Windows 的 CmdlineSlice 返回单元素整条命令行(wmic 实测):
// exe 判定必须取"第一个空格前"再 Base——否则 Base 取到命令行末尾的参数值,
// 匹配恒失败,面板重启后所有世界状态显示"停止/待开启"(世界明明在跑)。
func TestMatchLevelCmdlineSingleElement(t *testing.T) {
	cl := []string{`./dontstarve_dedicated_server_nullrenderer_x64.exe -console -cluster ROGIA -shard Master -persistent_storage_root D:/DST-Server -conf_dir worlds -ugc_directory D:\DST-Server\mods\steamapps\workshop`}
	if !matchLevelCmdline(cl, "ROGIA", "Master") {
		t.Fatal("单元素整条命令行(wmic 形态)应匹配")
	}
	if matchLevelCmdline(cl, "ROGIA", "Caves") {
		t.Fatal("shard 不符不应匹配")
	}
	// 拆分形态(参数独立元素)也要继续工作
	cl2 := []string{`C:\dst\dontstarve_dedicated_server_nullrenderer_x64.exe`, "-console", "-cluster", "ROGIA", "-shard", "Master"}
	if !matchLevelCmdline(cl2, "ROGIA", "Master") {
		t.Fatal("拆分形态应匹配")
	}
	// 非游戏进程不误报
	if matchLevelCmdline([]string{`C:\other\app.exe -cluster ROGIA -shard Master`}, "ROGIA", "Master") {
		t.Fatal("非游戏进程不应匹配")
	}
}

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

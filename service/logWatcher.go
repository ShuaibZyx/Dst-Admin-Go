package service

import (
	"bufio"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// 游戏日志文件监听：fsnotify 事件驱动推送（写入即推，0 延迟）+ 3 秒兜底轮询（补漏事件），
// 替代原来每条 SSE 连接各自的 2 秒轮询 stat+read。多连接共享一个 watcher（LiveSession 广播）。

// ReadLogIncrement 增量读日志：返回 offset 之后的新行与新 offset。
// 文件截断/轮转（size < offset）时从头重读。
func ReadLogIncrement(logPath string, offset int64) (lines []string, newOffset int64) {
	fi, err := os.Stat(logPath)
	if err != nil {
		return nil, offset
	}
	if fi.Size() < offset { // 轮转/截断：重建
		offset = 0
	}
	if fi.Size() == offset {
		return nil, offset
	}
	f, err := os.Open(logPath)
	if err != nil {
		return nil, offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	pos, _ := f.Seek(0, 1)
	return lines, pos
}

// logWatch 单个日志文件的监听状态（watcher + 消费进度 + 广播会话）
type logWatch struct {
	session *LiveSession
	mu      sync.Mutex
	offset  int64
}

var logWatchers sync.Map // logPath -> *logWatch

// LogSessionFor 获取日志文件的广播会话（首个请求创建 watcher 常驻任务）
func LogSessionFor(logPath string) *LiveSession {
	if v, ok := logWatchers.Load(logPath); ok {
		// 命中缓存也要确保 runner 活着：会话死过一次后 Attach 会返回已关通道
		w := v.(*logWatch)
		w.session.Start(func(send func(string)) {
			w.run(logPath)
		})
		return w.session
	}
	w := &logWatch{session: NewLiveSession()}
	actual, loaded := logWatchers.LoadOrStore(logPath, w)
	if loaded {
		w = actual.(*logWatch)
	}
	// Start 标记会话活跃（run 永不退出=常驻），否则 Attach 会把订阅通道立即关闭
	w.session.Start(func(send func(string)) {
		w.run(logPath)
	})
	return w.session
}

// run 常驻：fsnotify 事件驱动 + 3 秒兜底轮询（防事件合并/丢失）。
// 永不退出：fsnotify 在 Windows 高频写入下会缓冲溢出并关闭 Events/Errors 通道
// （游戏日志写入正属此列），一旦退出 LiveSession 会关闭所有订阅通道、把 SSE 长连接拉断，
// 故监听死亡时重建而不是结束。
func (w *logWatch) run(logPath string) {
	for {
		if !w.watchOnce(logPath) {
			time.Sleep(2 * time.Second) // 退避后重建监听
		}
	}
}

// watchOnce 单次监听会话；监听通道关闭（fsnotify 死亡）时返回 false
func (w *logWatch) watchOnce(logPath string) bool {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("[logwatch] 创建监听失败, 退回轮询:", err)
		w.pollLoop(logPath)
		return true // pollLoop 永不返回
	}
	defer watcher.Close()
	// 文件可能还不存在（世界未启动）：监听所在目录的创建事件来补挂
	_ = watcher.Add(logPath)
	fallback := time.NewTicker(3 * time.Second)
	defer fallback.Stop()
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return false
			}
			// 轮转（Create/Remove/Rename）后重挂并从头读；Write 读增量
			if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				w.mu.Lock()
				w.offset = 0
				w.mu.Unlock()
				_ = watcher.Remove(logPath)
				if err := watcher.Add(logPath); err != nil {
					return false // 重挂失败（文件瞬时不在）：整体重建
				}
			}
			w.push(logPath)
		case _, ok := <-watcher.Errors:
			if !ok {
				return false
			}
		case <-fallback.C:
			w.push(logPath) // 兜底：事件丢失/合并时补读
		}
	}
}

// pollLoop 无 fsnotify 时的降级（2 秒轮询，同旧语义）
func (w *logWatch) pollLoop(logPath string) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		w.push(logPath)
	}
}

// push 读增量并广播
func (w *logWatch) push(logPath string) {
	w.mu.Lock()
	lines, newOffset := ReadLogIncrement(logPath, w.offset)
	w.offset = newOffset
	w.mu.Unlock()
	for _, line := range lines {
		w.session.emit(line)
	}
}

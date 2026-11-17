package service

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// EventLog 启动过程事件日志：环形缓冲 + 实时订阅（服务器日志页展示）
type EventLog struct {
	mu    sync.Mutex
	lines []string
	subs  map[chan string]struct{}
	cap   int
}

var StartLog = NewEventLog(500)

// progressLine 生成带进度前缀的打点行(经 Logf 会再加时间戳;前端按 [进度:NN%] 子串解析)。
// 启动/停止过程的进度条数据源。
func progressLine(percent int, text string) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return fmt.Sprintf("[进度:%d%%] %s", percent, text)
}

// Progress 记一条进度打点(前端进度条数据源)。
// 必须 "%s" 包装:进度行含 "%]",直接当 format 会被 Logf 的 Sprintf 当动词污染。
func (e *EventLog) Progress(percent int, format string, args ...interface{}) {
	e.Logf("%s", progressLine(percent, fmt.Sprintf(format, args...)))
}

func NewEventLog(capacity int) *EventLog {
	return &EventLog{subs: map[chan string]struct{}{}, cap: capacity}
}

// Logf 记一行（带时间戳），同时进面板日志
func (e *EventLog) Logf(format string, args ...interface{}) {
	line := time.Now().Format("15:04:05 ") + fmt.Sprintf(format, args...)
	log.Println("[启动]", line)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lines = append(e.lines, line)
	if len(e.lines) > e.cap {
		e.lines = e.lines[len(e.lines)-e.cap:]
	}
	for ch := range e.subs {
		select {
		case ch <- line:
		default: // 慢订阅者丢行，不阻塞
		}
	}
}

// Attach 订阅：返回历史 + 实时行通道；unsub 取消订阅
func (e *EventLog) Attach() (backlog []string, ch chan string, unsub func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	backlog = append([]string{}, e.lines...)
	ch = make(chan string, 256)
	e.subs[ch] = struct{}{}
	return backlog, ch, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.subs, ch)
	}
}

package service

import (
	"log"
	"sync"
)

// LiveSession 可重连的任务会话：任务进行中再次请求=接上历史日志并继续跟随，
// 不会重复启动第二个任务（避免下载被打断/重来）。
type LiveSession struct {
	mu      sync.Mutex
	logs    []string
	running bool
	subs    map[chan string]struct{}
}

func NewLiveSession() *LiveSession {
	return &LiveSession{subs: map[chan string]struct{}{}}
}

// Start 启动任务；已在运行则返回 false（不会重复启动）
func (s *LiveSession) Start(run func(send func(string))) bool {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return false
	}
	s.running = true
	s.logs = nil
	s.mu.Unlock()

	go func() {
		defer func() {
			// runner panic 不得击穿进程:记录后走正常收尾(关闭订阅通道)
			if r := recover(); r != nil {
				log.Println("LiveSession runner panic:", r)
			}
			s.mu.Lock()
			s.running = false
			for ch := range s.subs {
				close(ch)
			}
			s.subs = map[chan string]struct{}{}
			s.mu.Unlock()
		}()
		run(s.emit)
	}()
	return true
}

// 历史积压上限:防止重连时全量倾泻(数百帧挤爆前端渲染)与内存无限增长
const liveSessionBacklogCap = 50

func (s *LiveSession) emit(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, line)
	if len(s.logs) > liveSessionBacklogCap {
		s.logs = s.logs[len(s.logs)-liveSessionBacklogCap:]
	}
	for ch := range s.subs {
		select {
		case ch <- line:
		default: // 慢消费者丢弃旧节奏行，避免卡住任务
		}
	}
}

// Attach 订阅会话：返回历史日志 + 后续行通道；任务结束时通道关闭。
// 会话空闲时返回已关闭的通道（仅历史）。
func (s *LiveSession) Attach() (backlog []string, ch chan string, unsub func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	backlog = append([]string{}, s.logs...)
	ch = make(chan string, 512)
	if !s.running {
		close(ch)
		return backlog, ch, func() {}
	}
	s.subs[ch] = struct{}{}
	return backlog, ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

// IsRunning 任务是否进行中
func (s *LiveSession) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

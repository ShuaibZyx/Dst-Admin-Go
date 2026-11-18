package api

import (
	"sync"
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// LiveStream GET /api/game/live/stream?cluster=&level= — 聚合 SSE：
// status(2s 心跳)/log(实时)/startlog(实时)/sysinfo(3s) 四类事件一条连接，
// 替代前端 4 条独立 SSE（浏览器同域 6 连接,单流把连接压力降到 1 条）。
// 事件帧：event: status|log|startlog|sysinfo + data: <JSON 或原始行>
func (g *GameConsoleApi) LiveStream(ctx *gin.Context) {
	clusterName := ctx.Query("cluster")
	if clusterName == "" {
		clusterName = clusterUtils.GetClusterFromGin(ctx).ClusterName
	}
	levelName := ctx.Query("level")
	if levelName == "" {
		levelName = "Master"
	}
	if !fileUtils.ValidName(levelName) { // F1: 防路径穿越
		ctx.AbortWithStatusJSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}
	logPath := filepath.Join(dstUtils.GetClusterBasePath(clusterName), levelName, "server_log.txt")

	// 四源订阅（共享会话,多连接广播）
	statusBacklog, statusCh, statusUnsub := statusStreamFor(clusterName).Attach()
	logBacklog, logCh, logUnsub := service.LogSessionFor(logPath).Attach()
	startBacklog, startCh, startUnsub := service.StartLog.Attach()
	sysBacklog, sysCh, sysUnsub := sysinfoStreamFor(clusterName).Attach()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	// backlog（历史）按类分帧输出；log 只取尾部 40 行（首屏上下文）。
	// startlog 历史帧用 startlog-hist：前端据此区分"历史只进文本"与"实时驱动进度条"
	for _, l := range statusBacklog {
		_, _ = fmt.Fprintf(ctx.Writer, "event: status\ndata: %s\n\n", l)
	}
	if n := len(logBacklog); n > 0 {
		start := n - 40
		if start < 0 {
			start = 0
		}
		for _, l := range logBacklog[start:] {
			_, _ = fmt.Fprintf(ctx.Writer, "event: log\ndata: %s\n\n", l)
		}
	}
	for _, l := range startBacklog {
		_, _ = fmt.Fprintf(ctx.Writer, "event: startlog-hist\ndata: %s\n\n", l)
	}
	for _, l := range sysBacklog {
			_, _ = fmt.Fprintf(ctx.Writer, "event: sysinfo\ndata: %s\n\n", l)
	}
	ctx.Writer.Flush()

	// sysinfo 3 秒采样走共享会话（多连接合并为一次系统信息轮询）

	// 退出时统一退订（reprobe 会重挂并改写 unsub 变量,故用闭包取当前值）
	defer func() {
		statusUnsub()
		logUnsub()
		startUnsub()
		sysUnsub()
	}()

	// 转发循环：任一订阅源结束只停用该源（置 nil 后 select 忽略该 case），
	// 绝不 return——旧写法 `if !ok { return }` 会把整条 SSE 长连接拉断
	// （浏览器反复重连、面板"不输出日志"的根因）。
	reprobe := time.NewTicker(3 * time.Second)
	defer reprobe.Stop()
	for {
		select {
		case l, ok := <-statusCh:
			if !ok {
				statusCh = nil
				continue
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: status\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l, ok := <-logCh:
			if !ok {
				logCh = nil
				continue
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: log\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l, ok := <-startCh:
			if !ok {
				startCh = nil
				continue
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: startlog\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l, ok := <-sysCh:
			if !ok {
				sysCh = nil
				continue
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: sysinfo\ndata: %s\n\n", l)
			ctx.Writer.Flush()
			// 停用的源在恢复后重挂（Attach/会话已确保拉起;跳过 backlog 防重复帧）
			if statusCh == nil {
				if _, c, un := statusStreamFor(clusterName).Attach(); c != nil {
					statusCh, statusUnsub = c, un
				}
			}
			if logCh == nil {
				if _, c, un := service.LogSessionFor(logPath).Attach(); c != nil {
					logCh, logUnsub = c, un
				}
			}
			if sysCh == nil {
				if _, c, un := sysinfoStreamFor(clusterName).Attach(); c != nil {
					sysCh, sysUnsub = c, un
				}
			}
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

// sysinfoStreamFor 按集群共享的系统信息推送会话（3 秒采样 + 同载荷去重）。
// 旧版每条 SSE 连接各自起 goroutine 轮询系统信息，多面板/多标签时纯浪费。
var sysinfoStreams sync.Map

func sysinfoStreamFor(clusterName string) *service.LiveSession {
	run := func(send func(string)) {
		cs := &service.ChangeSender{}
		for {
			data, err := json.Marshal(vo.Response{Code: 200, Msg: "success", Data: gameService.GetSystemInfo(clusterName)})
			if err == nil && cs.ShouldPush(string(data)) {
				send(string(data))
			}
			time.Sleep(3 * time.Second)
		}
	}
	if v, ok := sysinfoStreams.Load(clusterName); ok {
		s := v.(*service.LiveSession)
		s.Start(run)
		return s
	}
	s := service.NewLiveSession()
	s.Start(run)
	actual, _ := sysinfoStreams.LoadOrStore(clusterName, s)
	return actual.(*service.LiveSession)
}

package api

import (
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
	defer statusUnsub()
	defer logUnsub()
	defer startUnsub()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	// backlog（历史）按类分帧输出；log 只取尾部 40 行（首屏上下文）
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
		_, _ = fmt.Fprintf(ctx.Writer, "event: startlog\ndata: %s\n\n", l)
	}
	ctx.Writer.Flush()

	// sysinfo 独立 3 秒轮询（系统信息变化慢,不走事件）
	// 不 close(sysCh)：sysinfo goroutine 靠 ctx.Done 自行退出,close 后 send 会 panic 崩服务
	sysCh := make(chan string, 4)
	go func() {
		for {
			data, err := json.Marshal(vo.Response{Code: 200, Msg: "success", Data: gameService.GetSystemInfo(clusterName)})
			if err == nil {
				select {
				case sysCh <- string(data):
				default: // 慢消费者丢弃,不阻塞
				}
			}
			select {
			case <-ctx.Request.Context().Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()

	// 转发循环
	for {
		select {
		case l, ok := <-statusCh:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: status\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l, ok := <-logCh:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: log\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l, ok := <-startCh:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: startlog\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case l := <-sysCh:
			_, _ = fmt.Fprintf(ctx.Writer, "event: sysinfo\ndata: %s\n\n", l)
			ctx.Writer.Flush()
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

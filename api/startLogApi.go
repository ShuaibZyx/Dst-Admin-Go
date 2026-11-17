package api

import (
	"dst-admin-go/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

type StartLogApi struct{}

// Stream GET /api/game/startlog — SSE：启动过程事件（历史 + 实时）
func (s *StartLogApi) Stream(ctx *gin.Context) {
	backlog, ch, unsub := service.StartLog.Attach()
	defer unsub()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	// hist=历史 backlog(不驱动进度条), live=实时行(驱动进度条)——前端按 event 区分
	for _, line := range backlog {
		_, _ = fmt.Fprintf(ctx.Writer, "event: hist\ndata: %s\n\n", line)
	}
	ctx.Writer.Flush()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: live\ndata: %s\n\n", line)
			ctx.Writer.Flush()
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

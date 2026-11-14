package api

import (
	mod2 "dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ModsDownloadApi struct{}

// DownloadCurrent 下载当前存档所需模组（SSE 进度）
func (m *ModsDownloadApi) DownloadCurrent(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在系统设置中选择根目录", "data": nil})
		return
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	eventCh := make(chan string, 64)
	go func() {
		defer close(eventCh)
		n, err := mod2.EnsureClusterMods(cfg.Cluster, func(s string) { eventCh <- s })
		if err != nil {
			eventCh <- "[失败] " + err.Error()
			return
		}
		eventCh <- fmt.Sprintf("完成：本次下载 %d 个模组", n)
	}()
	for line := range eventCh {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
}

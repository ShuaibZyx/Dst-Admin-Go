package api

import (
	mod2 "dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ModsDownloadApi struct{}

var modsDlSession = NewLiveSession()

// DownloadCurrent 校验/下载当前存档模组（SSE 进度，可重连接上）。
// mode=repair：校验并修复问题项；mode=redownload：先删后下全部模组。
func (m *ModsDownloadApi) DownloadCurrent(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在系统设置中选择根目录", "data": nil})
		return
	}
	mode := ctx.Query("mode")
	if mode != "redownload" {
		mode = "repair"
	}
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	modsDlSession.Start(func(send func(string)) {
		summary, err := mod2.SyncMods(cfg.Cluster, mode, send)
		if err != nil {
			send("[失败] " + err.Error())
			return
		}
		send(summary)
	})

	backlog, ch, _ := modsDlSession.Attach()
	for _, line := range backlog {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
	for line := range ch {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
}

// DownloadStatus 模组下载任务状态
func (m *ModsDownloadApi) DownloadStatus(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success",
		"data": gin.H{"downloading": modsDlSession.IsRunning()}})
}

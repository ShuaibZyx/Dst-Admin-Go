package api

import (
	"dst-admin-go/service"
	"dst-admin-go/utils/fileUtils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"

	"github.com/gin-gonic/gin"
)

// SystemInfoStream SSE 推送系统监控（前端 useSystemInfoStream 消费 message 事件，data 为 vo.Response JSON）
func (g *GameConsoleApi) SystemInfoStream(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Stream(func(w io.Writer) bool {
		data, err := json.Marshal(vo.Response{Code: 200, Msg: "success", Data: gameService.GetSystemInfo(cluster.ClusterName)})
		if err == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		time.Sleep(3 * time.Second)
		return true
	})
}

// LogStream SSE 推送世界日志增量（前端 useLogStream 消费 event: log，data 为原始行）
func (g *GameConsoleApi) LogStream(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	levelName := ctx.Query("levelName")
	if levelName == "" {
		levelName = "Master"
	}
	if !fileUtils.ValidName(levelName) { // F1: 防路径穿越
		ctx.AbortWithStatusJSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}
	logPath := filepath.Join(dstUtils.GetClusterBasePath(cluster.ClusterName), levelName, "server_log.txt")

	// fsnotify 事件驱动（共享 watcher + 多连接广播）：写入即推（0 延迟），
	// 替代原来每条连接各自的 2 秒轮询 stat+read
	sess := service.LogSessionFor(logPath)
	backlog, ch, unsub := sess.Attach()
	defer unsub()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	// 首连推尾部 2KB 行（backlog 由 watcher 从启动起累积；此处兜底保证首屏有上下文）
	for _, line := range backlog[len(backlog)-min(len(backlog), 40):] {
		_, _ = fmt.Fprintf(ctx.Writer, "event: log\ndata: %s\n\n", line)
	}
	ctx.Writer.Flush()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "event: log\ndata: %s\n\n", line)
			ctx.Writer.Flush()
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

var _ = log.Println
var _ = http.StatusOK

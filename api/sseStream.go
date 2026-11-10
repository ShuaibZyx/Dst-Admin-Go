package api

import (
	"dst-admin-go/utils/fileUtils"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	if !fileUtils.ValidName(levelName) { // F1: 防路径穿越
		ctx.AbortWithStatusJSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}
	if !fileUtils.ValidName(levelName) { // F1: 防路径穿越
		ctx.AbortWithStatusJSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}
	logPath := filepath.Join(dstUtils.GetClusterBasePath(cluster.ClusterName), levelName, "server_log.txt")

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	var offset int64 = -1 // -1 表示首次：从尾部 2KB 开始，只推新增
	ctx.Stream(func(w io.Writer) bool {
		fi, err := os.Stat(logPath)
		if err != nil {
			time.Sleep(3 * time.Second)
			return true
		}
		if offset < 0 {
			offset = fi.Size()
			if offset > 2048 {
				offset -= 2048
			}
			// 跳过半个行，避免开头截断
			if offset > 0 {
				offset = alignToLine(logPath, offset)
			}
		}
		if fi.Size() < offset { // F3: 日志被截断/轮转时回退重读
			offset = 0
		}
		if fi.Size() < offset { // F3: 日志被截断/轮转时回退重读
			offset = 0
		}
		if fi.Size() < offset { // F3: 日志被截断/轮转时回退重读
			offset = 0
		}
		if fi.Size() > offset {
			f, err := os.Open(logPath)
			if err == nil {
				_, _ = f.Seek(offset, io.SeekStart)
				sc := bufio.NewScanner(f)
				for sc.Scan() {
					line := sc.Text()
					if line != "" {
						fmt.Fprintf(w, "event: log\ndata: %s\n\n", line)
					}
					offset, _ = f.Seek(0, io.SeekCurrent)
				}
				// scanner 缓冲可能未读到 EOF 处，offset 用文件当前消费位置
				pos, _ := f.Seek(0, io.SeekCurrent)
				if pos > offset {
					offset = pos
				}
				_ = f.Close()
			}
		}
		time.Sleep(2 * time.Second)
		return true
	})
}

// alignToLine 从给定偏移向前找到行首（避免推送半行）
func alignToLine(path string, offset int64) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	buf := make([]byte, 1)
	pos := offset
	for pos > 0 {
		if _, err := f.Seek(pos-1, io.SeekStart); err != nil {
			break
		}
		if _, err := f.Read(buf); err != nil {
			break
		}
		if buf[0] == '\n' {
			return pos
		}
		pos--
	}
	return pos
}

var _ = log.Println
var _ = http.StatusOK

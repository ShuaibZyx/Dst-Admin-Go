package api

import (
	"dst-admin-go/service"
	mod2 "dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type GameUpdateApi struct{}

var updateSession = service.NewLiveSession()

// UpdateGame GET /api/game/update?isDelete= —— SSE：停世界 → 校验/更新（可选先删服务器目录）→ 重写模组订阅
func (g *GameUpdateApi) UpdateGame(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在系统设置中选择根目录", "data": nil})
		return
	}
	clusterName := cfg.Cluster
	isDelete := ctx.Query("isDelete") == "true"

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	updateSession.Start(func(send func(string)) {
		runGameUpdate(cfg, clusterName, isDelete, send)
	})
	backlog, ch, _ := updateSession.Attach()
	for _, line := range backlog {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
	for line := range ch {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
}

func runGameUpdate(cfg dstConfigUtils.DstConfig, clusterName string, isDelete bool, send func(string)) {
	var gs service.GameService

	send("停止所有世界...")
	gs.StopGameWithoutLog(clusterName)
	send("世界已停止")

	steamcmdDir, serverDir, _ := oneclickTargets(cfg)

	if isDelete {
		send("删除现有服务器文件（" + serverDir + "）...")
		if err := os.RemoveAll(serverDir); err != nil {
			send("[失败] 删除服务器目录失败: " + err.Error())
			return
		}
		if err := os.MkdirAll(serverDir, os.ModePerm); err != nil {
			send("[失败] 重建服务器目录失败: " + err.Error())
			return
		}
	}

	send("开始校验/更新饥荒专用服务器（可能耗时）...")
	args := []string{"+force_install_dir", serverDir, "+login", "anonymous", "+app_update", "343050"}
	if cfg.Beta == 1 {
		args = append(args, "-beta", "updatebeta")
	}
	args = append(args, "validate", "+quit")
	err := mod2.RunSteamcmdStream(steamcmdDir, args, send)
	if err != nil {
		send("[提示] 未成功，10 秒后自动重试一次...")
		time.Sleep(10 * time.Second)
		err = mod2.RunSteamcmdStream(steamcmdDir, args, send)
	}
	if err != nil {
		send("[失败] 更新失败: " + err.Error())
		return
	}

	send("重写模组订阅文件...")
	gs.RewriteModSetup(clusterName)
	send("完成！服务器已更新，可到世界列表启动世界。")
}

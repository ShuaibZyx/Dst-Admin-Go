package api

import (
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/vo"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type PlayerApi struct {
}

var playerService = service.PlayerService{}

// playerListWithTimeout 玩家查询带 2 秒兜底超时。
// 游戏命令失败(如 attempt to call a nil value)时 GetPlayerList 会死等内部 10 秒超时,
// 多组件轮询 × 长阻塞占满浏览器连接池(HTTP/1.1 同域 6 连接)→ 一启动世界整个面板卡死。
// 超时先返回空列表,后台 goroutine 自会在内部超时后收尾(短时泄漏可接受)。
func playerListWithTimeout(clusterName, levelName string) []vo.PlayerVO {
	ch := make(chan []vo.PlayerVO, 1)
	go func() { ch <- playerService.GetPlayerList(clusterName, levelName) }()
	select {
	case list := <-ch:
		return list
	case <-time.After(2 * time.Second):
		return []vo.PlayerVO{}
	}
}

func (p *PlayerApi) GetDstPlayerList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: playerListWithTimeout(clusterName, "Master"),
	})
}

func (p *PlayerApi) GetDstAdminList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: playerService.GetDstAdminList(clusterName),
	})
}

func (p *PlayerApi) GetDstBlcaklistPlayerList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: playerService.GetDstBlacklistPlayerList(clusterName),
	})
}

func (p *PlayerApi) SaveDstAdminList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	adminListVO := vo.NewAdminListVO()
	ctx.BindJSON(adminListVO)
	playerService.SaveDstAdminList(clusterName, adminListVO.AdminList)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
	})
}

func (p *PlayerApi) SaveDstBlacklistPlayerList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	blacklistVO := vo.NewBlacklistVO()
	ctx.BindJSON(blacklistVO)
	playerService.SaveDstBlacklistPlayerList(clusterName, blacklistVO.Blacklist)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
	})
}

func (p *PlayerApi) DeleteDstBlacklistPlayerList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	blacklistVO := vo.NewBlacklistVO()
	ctx.BindJSON(blacklistVO)
	playerService.DeleteDstBlacklistPlayerList(clusterName, blacklistVO.Blacklist)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
	})
}

func (p *PlayerApi) DeleteDstAdminListPlayerList(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	adminlistVO := vo.NewAdminListVO()
	ctx.BindJSON(adminlistVO)
	playerService.DeleteDstAdminListPlayerList(clusterName, adminlistVO.AdminList)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
	})
}

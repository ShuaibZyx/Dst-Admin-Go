package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/pathUtils"
	"dst-admin-go/vo"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

type WebhookApi struct {
}

var (
	SearchHome = "searchHome"

	HomeInfo      = "homeInfo"
	OnlinePlayers = "onlinePlayers"

	// 玩家操作

	// 房间启动关闭

	// 更新游戏
)

type Body struct {
	MsgType string      `json:"msgtype"`
	Param   interface{} `json:"param"`
}

func verifyKey(key string, ctx *gin.Context) bool {

	keyPath := pathUtils.Data("key") // webhook 密钥文件(数据根,惰性解析)
	// 不自动建 key 文件:缺失即拒绝(旧行为 CreateFileIfNotExists(key) 是笔误,
	// 会拿密钥值当文件名在 CWD 造垃圾文件且把密钥泄露进文件名)
	content, err := fileUtils.ReadFile(keyPath)
	content = strings.Replace(content, "\n", "", -1)
	log.Println("key: ", key)
	log.Println("content: ", content)
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return false
	}

	if key != content {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return false
	}
	return true
}

func (w *WebhookApi) Webhook(ctx *gin.Context) {

	var body Body
	err := ctx.ShouldBind(&body)
	if err != nil {
		abort500f(ctx, "参数解析错误", err)
		return
	}

	key := ctx.Query("key")
	if !verifyKey(key, ctx) {
		return
	}

	config := dstConfigUtils.GetDstConfig()
	clusterName := config.Cluster

	switch body.MsgType {
	case SearchHome:
		break
	case HomeInfo:
		gameArchive := gameArchiveService.GetGameArchive(clusterName)
		ctx.JSON(http.StatusOK, vo.Response{
			Code: 200,
			Msg:  "success",
			Data: gameArchive,
		})
		break
	case OnlinePlayers:
		playerList := playerListWithTimeout(clusterName, "Master")
		ctx.JSON(http.StatusOK, vo.Response{
			Code: 200,
			Msg:  "success",
			Data: playerList,
		})
		break
	default:
		ctx.JSON(http.StatusOK, vo.Response{
			Code: 200,
			Msg:  "success",
			Data: nil,
		})
	}
}

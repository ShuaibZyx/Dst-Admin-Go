package api

import (
	"dst-admin-go/model"
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/levelConfigUtils"
	"dst-admin-go/vo"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type GameConsoleApi struct {
}

// validKuId I-4: 玩家标识白名单（补反引号），所有 kuId 入口复用
func validKuId(kuId string) bool {
	if kuId == "" {
		return false
	}
	for _, r := range kuId {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

var consoleService = service.GameConsoleService{}
var gameArchiveService = service.GameArchive{}
var announceService = service.AnnounceService{}
var gameService = service.GameService{}

func (g *GameConsoleApi) UpdateGame(ctx *gin.Context) {

	log.Println("正在更新游戏。。。。。。")
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	err := gameService.UpdateGame(clusterName)
	if err != nil {
		abort500f(ctx, "更新游戏失败: ", err)
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "update dst success",
		Data: nil,
	})
}

func (g *GameConsoleApi) GetSystemInfo(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gameService.GetSystemInfo(clusterName),
	})
}

func (g *GameConsoleApi) SentBroadcast(ctx *gin.Context) {
	message := ctx.Query("message")
	if strings.ContainsAny(message, "\r\n") || len(message) > 200 { // I-4: 入口清理
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal message"})
		return
	}
	log.Println("发送公告信息：" + message)
	cluster := clusterUtils.GetClusterFromGin(ctx)

	consoleService.SentBroadcast(cluster.ClusterName, message)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) KickPlayer(ctx *gin.Context) {

	kuId := ctx.Query("kuId")
	if !validKuId(kuId) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal kuId"})
		return
	}
	log.Println("踢出玩家：" + kuId)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.KickPlayer(clusterName, kuId)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) KillPlayer(ctx *gin.Context) {

	kuId := ctx.Query("kuId")
	if !validKuId(kuId) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal kuId"})
		return
	}
	log.Println("kill玩家：" + kuId)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.KillPlayer(clusterName, kuId)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) RespawnPlayer(ctx *gin.Context) {

	kuId := ctx.Query("kuId")
	if !validKuId(kuId) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal kuId"})
		return
	}
	log.Println("复活玩家：" + kuId)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.RespawnPlayer(clusterName, kuId)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) RollBack(ctx *gin.Context) {

	dayNums := ctx.Query("dayNums")
	days, err := strconv.Atoi(dayNums)
	if err != nil {
		abort500f(ctx, "参数解析错误："+dayNums)
		return
	}
	log.Println("回滚指定的天数：" + dayNums)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.RollBack(clusterName, days)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) Regenerateworld(ctx *gin.Context) {

	log.Println("重置世界......")

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.Regenerateworld(clusterName)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) CleanWorld(ctx *gin.Context) {

	log.Println("删除世界......")

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.CleanWorld(clusterName)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) CleanAllLevel(ctx *gin.Context) {

	defer func() {
		if r := recover(); r != nil {
			log.Println("删除世界存档失败", r) // C6: 不再重新抛出
		}
	}()

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	levelConfig, _ := levelConfigUtils.GetLevelConfig(clusterName)
	basePath := dstUtils.GetClusterBasePath(clusterName)
	for i := range levelConfig.LevelList {
		level := levelConfig.LevelList[i]
		fileUtils.DeleteDir(path.Join(basePath, level.File, "backup"))
		fileUtils.DeleteDir(path.Join(basePath, level.File, "save"))
		fileUtils.DeleteDir(path.Join(basePath, level.File, "server_chat_log.txt"))
		fileUtils.DeleteDir(path.Join(basePath, level.File, "server_log.txt"))
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) CleanLevel(ctx *gin.Context) {

	defer func() {
		if r := recover(); r != nil {
			abort500f(ctx, "删除世界存档失败", r)
			return
		}
	}()

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	basePath := dstUtils.GetClusterBasePath(clusterName)
	levels := ctx.QueryArray("level")

	for _, level := range levels {
		if !fileUtils.ValidName(level) {
			continue // A9: 非法 level 名跳过，防止穿越删除
		}
		fileUtils.DeleteDir(path.Join(basePath, level, "backup"))
		fileUtils.DeleteDir(path.Join(basePath, level, "save"))
		fileUtils.DeleteDir(path.Join(basePath, level, "server_chat_log.txt"))
		fileUtils.DeleteDir(path.Join(basePath, level, "server_log.txt"))
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) MasterConsole(ctx *gin.Context) {
	var body struct {
		Command string `json:"command"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		return
	}

	comment := body.Command

	log.Println("地面控制台: " + comment)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.MasterConsole(clusterName, comment)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) CavesConsole(ctx *gin.Context) {
	var body struct {
		Command string `json:"command"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		return
	}

	comment := body.Command

	log.Println("洞穴控制台: " + comment)

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.CavesConsole(clusterName, comment)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) OperatePlayer(ctx *gin.Context) {

	otype := ctx.Query("type") // I-4: 路由 /operate/player 无路径参数，走 query
	kuId := ctx.Query("kuId")
	if otype != "0" && otype != "1" && otype != "2" { // I1/M13
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal type"})
		return
	}
	if !validKuId(kuId) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal kuId"})
		return
	}

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.OperatePlayer(clusterName, otype, kuId)

	log.Printf("执行高级针对玩家的操作: type=%s,kuId=%s", otype, kuId)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameConsoleApi) RestoreBackup(ctx *gin.Context) {

	backupName := ctx.Query("backupName")

	if err := backupService.RestoreBackup(ctx, backupName); err != nil {
		ctx.JSON(http.StatusInternalServerError, vo.Response{Code: 500, Msg: "restore failed: " + err.Error()})
		return // I3: 恢复失败不再假报成功
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "restore backup success",
		Data: nil,
	})
}

func (g *GameConsoleApi) GetGameArchive(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gameArchiveService.GetGameArchive(clusterName),
	})
}

func (g *GameConsoleApi) GetAnnounceSetting(ctx *gin.Context) {

	//cluster := clusterUtils.GetClusterFromGin(ctx)
	//clusterName := cluster.ClusterName

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: announceService.GetAnnounceSetting(),
	})
}

func (g *GameConsoleApi) SaveAnnounceSetting(ctx *gin.Context) {

	//cluster := clusterUtils.GetClusterFromGin(ctx)
	//clusterName := cluster.ClusterName

	var announce model.Announce
	err := ctx.ShouldBind(&announce)
	log.Println(announce)
	if err != nil {
		log.Println("参数错误", err)
	}
	announceService.SaveAnnounceSetting(&announce)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: announce,
	})

}

func (g *GameConsoleApi) ReadLevelServeLog(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	lines, _ := strconv.Atoi(ctx.DefaultQuery("lines", "100"))
	levelName := ctx.Query("levelName")
	if levelName == "" {
		abort500f(ctx, "levelName 不能为空")
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: consoleService.ReadLevelServerLog(clusterName, levelName, uint(lines)),
	})
}

func (g *GameConsoleApi) ReadLevelServeChatLog(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	lines, _ := strconv.Atoi(ctx.DefaultQuery("lines", "100"))
	levelName := ctx.Query("levelName")
	if levelName == "" {
		abort500f(ctx, "levelName 不能为空")
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: consoleService.ReadLevelServerChatLog(clusterName, levelName, uint(lines)),
	})
}

func (g *GameConsoleApi) ReadServerLog(ctx *gin.Context) {

	lines, _ := strconv.Atoi(ctx.DefaultQuery("lines", "100"))
	logs, err := fileUtils.ReverseRead("./log.log", uint(lines))
	if err != nil {
		abort500f(ctx, "读取面板日志失败")
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: logs,
	})
}

func (g *GameConsoleApi) DownloadDstLogFile(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	fileName := ctx.Query("fileName")
	levelName := ctx.Query("levelName")
	if fileName == "" || levelName == "" { // M-4: 空值走 400
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "fileName/levelName 不能为空"})
		return
	}

	if !fileUtils.ValidName(levelName) || strings.ContainsAny(fileName, "/\\") || strings.Contains(fileName, "..") {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal path"})
		return
	}
	filePath := filepath.Join(dstUtils.GetClusterBasePath(clusterName), levelName, fileName)
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(dstUtils.GetClusterBasePath(clusterName))) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal path"})
		return // I2: 防路径穿越
	}
	if _, err := os.Stat(filePath); err != nil {
		log.Println("download filePath error", err)
		ctx.JSON(http.StatusNotFound, vo.Response{Code: 404, Msg: "file not found"})
		return
	}
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", "attachment; filename="+fileName)
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.File(filePath) // I2: 不再预开句柄
}

func (g *GameConsoleApi) DownloadServerLogFile(ctx *gin.Context) {

	filePath := "./log.log"
	//打开文件
	_, err := os.Open(filePath)
	//非空处理
	if err != nil {
		abort500f(ctx, "download filePath error", err)
		return
	}
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", "attachment; filename="+"log.log")
	ctx.Header("Content-Transfer-Encoding", "binary")
	// c.Header("Content-Length", strconv.FormatInt(f.Size(), 10))
	ctx.File(filePath)
}

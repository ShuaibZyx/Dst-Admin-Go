package api

import (
	"regexp"
	"dst-admin-go/constant/consts"
	"dst-admin-go/model"
	"dst-admin-go/service"
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/service/healthCheck"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/levelConfigUtils"
	"dst-admin-go/utils/shellUtils"
	"dst-admin-go/vo"
	"dst-admin-go/vo/level"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var gameLevel2Service = service.GameLevel2Service{}
var homeService = service.HomeService{}

type GameLevel2Api struct{}

type ClusterIni struct {
	Cluster *level.ClusterIni `json:"cluster"`
	Token   string            `json:"token"`
}

type LevelInfo struct {
	Ps        *vo.DstPsVo      `json:"Ps"`
	Status    bool             `json:"status"`
	LevelName string           `json:"levelName"`
	IsMaster  bool             `json:"is_master"`
	Uuid      string           `json:"uuid"`
	Location  string           `json:"location"`
	ServerIni *level.ServerIni `json:"server_ini"`
}

// levelLocation 从 leveldataoverride 文本里抠 location——状态帧只带结果不带全文
// （每帧约 20KB 的 Lua 全文是纯体脂，前端只用到 location/状态/进程信息）
var locationRe = regexp.MustCompile(`location\s*=\s*"([^"]*)"`)

func levelLocation(leveldataoverride string) string {
	if m := locationRe.FindStringSubmatch(leveldataoverride); m != nil {
		return m[1]
	}
	return ""
}

// GetStatus 获取世界状态
// queryLevelsStatus 查询全部世界状态(GetStatus 与状态推送流共用)
func queryLevelsStatus(clusterName string) []LevelInfo {
	levelList := gameLevel2Service.GetLevelList(clusterName)
	length := len(levelList)
	result := make([]LevelInfo, length)

	if runtime.GOOS == "windows" {

		var wg sync.WaitGroup
		wg.Add(length)
		for i := range levelList {
			go func(index int) {
				defer func() {
					wg.Done()
					if r := recover(); r != nil {

					}
				}()
				world := levelList[index]
				ps := gameService.PsAuxSpecified(clusterName, world.Uuid)
				status := gameService.GetLevelStatus(clusterName, world.Uuid)
				result[index] = LevelInfo{
					Ps:                ps,
					Status:            status,
					LevelName:         world.LevelName,
					IsMaster:          world.IsMaster,
					Uuid:              world.Uuid,
					Location:          levelLocation(world.Leveldataoverride),
					ServerIni:         world.ServerIni,
				}
			}(i)
		}
		wg.Wait()
		return result
	}
	for i := range levelList {
		world := levelList[i]
		result[i] = LevelInfo{
			Ps:                &vo.DstPsVo{},
			Status:            false,
			LevelName:         world.LevelName,
			IsMaster:          world.IsMaster,
			Uuid:              world.Uuid,
			Location:          levelLocation(world.Leveldataoverride),
			ServerIni:         world.ServerIni,
		}
	}

	cmd := "ps -aux | grep -v grep | grep -v tail | grep -v SCREEN | grep " + clusterName + " |awk '{print $3, $4, $5, $6,$16}'"
	info, err := shellUtils.Shell(cmd)
	if err != nil {
		log.Println(cmd + " error: " + err.Error())
		return result
	}
	lines := strings.Split(info, "\n")
	for lineIndex := range lines {
		dstPsVo := vo.NewDstPsVo()
		arr := strings.Split(lines[lineIndex], " ")
		//for i := range arr {
		//	log.Println(i, arr[i])
		//}
		if len(arr) > 4 {
			dstPsVo.CpuUage = strings.Replace(arr[0], "\n", "", -1)
			dstPsVo.MemUage = strings.Replace(arr[1], "\n", "", -1)
			dstPsVo.VSZ = strings.Replace(arr[2], "\n", "", -1)
			dstPsVo.RSS = strings.Replace(arr[3], "\n", "", -1)
			for i := range result {
				levelName := result[i].Uuid
				if strings.Contains(arr[4], levelName) {
					result[i].Ps = dstPsVo
					result[i].Status = true
				}
			}
		}

	}
	return result
}

func (g *GameLevel2Api) GetStatus(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: queryLevelsStatus(cluster.ClusterName),
	})
}

// statusStreams 按集群独立的状态推送会话（不同集群不共享广播）
var statusStreams sync.Map

func statusStreamFor(clusterName string) *service.LiveSession {
	run := func(send func(string)) {
		// 常驻推送：2 秒采样一次，但同载荷去重（无变化不推）——
		// 状态帧含各层 Lua 全文约 20KB，重复心跳纯浪费且挤前端渲染
		cs := &service.ChangeSender{}
		for {
			b, _ := json.Marshal(queryLevelsStatus(clusterName))
			if cs.ShouldPush(string(b)) {
				send(string(b))
			}
			time.Sleep(2 * time.Second)
		}
	}
	if v, ok := statusStreams.Load(clusterName); ok {
		// runner 死过（panic 恢复等）后 Attach 返回已关通道，命中缓存时补拉起
		s := v.(*service.LiveSession)
		s.Start(run)
		return s
	}
	s := service.NewLiveSession()
	s.Start(run)
	actual, _ := statusStreams.LoadOrStore(clusterName, s)
	return actual.(*service.LiveSession)
}

// StatusStream GET /api/game/8level/status/stream — SSE：世界状态推送
// cluster 优先取 query（EventSource 发不了自定义 Header），缺省回落 Header/默认集群
func (g *GameLevel2Api) StatusStream(ctx *gin.Context) {
	clusterName := ctx.Query("cluster")
	if clusterName == "" {
		clusterName = clusterUtils.GetClusterFromGin(ctx).ClusterName
	}
	s := statusStreamFor(clusterName)
	backlog, ch, _ := s.Attach()

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	for _, line := range backlog {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
	}
	ctx.Writer.Flush()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
			ctx.Writer.Flush()
		case <-ctx.Request.Context().Done():
			return
		}
	}
}

func startBefore(ctx *gin.Context) {
	// 体检闸门：仅手动开服路径（定时/自动拉起不走这里）
	cluster := clusterUtils.GetClusterFromGin(ctx)
	basePath := dstUtils.GetClusterBasePath(cluster.ClusterName)
	items := healthCheck.Run(basePath)
	saveHealthLast(items)
	force := ctx.Query("force") == "true" || ctx.Query("force") == "1"
	if HealthHasError(items) && HealthStrictEnabled() && !force {
		ctx.AbortWithStatusJSON(http.StatusConflict, vo.Response{
			Code: 409,
			Msg:  "体检未通过（严格模式）：存在 error 级问题。修复后重试，或带 force=true 强制启动",
			Data: items,
		})
		return
	}
	ApplyLuaJit(cluster.ClusterName) // LuaJIT 开关一致性
	copyOsFile()
}

func copyOsFile() {
	config := dstConfigUtils.GetDstConfig()
	// 定义路径
	srcFile := filepath.Join(dstConfigUtils.ResolvePath(config.Parent_path, config.Steamcmd, dstConfigUtils.DefaultSteamcmd), "linux32", "steamclient.so")
	dstDir := filepath.Join(dstConfigUtils.ResolvePath(config.Parent_path, config.Force_install_dir, dstConfigUtils.DefaultServerDir), "bin", "lib32")
	dstFile := filepath.Join(dstDir, "steamclient.so")
	backupFile := filepath.Join(dstDir, "steamclient.so.bak")

	// 检查目标文件是否存在
	if _, err := os.Stat(dstFile); err == nil {
		// 如果目标文件存在，先重命名为备份文件
		if err = os.Rename(dstFile, backupFile); err != nil {
			log.Println("重命名文件失败:", err)
			return
		}
		log.Println("已将", dstFile, "重命名为", backupFile)
	}

	// 复制新文件
	if err := copyFile(srcFile, dstFile); err != nil {
		log.Println("复制文件失败:", err)
		return
	}
	log.Println("已将复制到", srcFile, dstFile)

}

// copyFile 实现文件复制
func copyFile(src, dst string) error {
	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("无法打开源文件: %v", err)
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("无法创建目标文件: %v", err)
	}
	defer dstFile.Close()

	// 复制内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制内容失败: %v", err)
	}

	return nil
}

// Start 启动世界
func (g *GameLevel2Api) Start(ctx *gin.Context) {
	levelName := ctx.Query("levelName")
	if !fileUtils.ValidName(levelName) { // I1
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}
	cluster := clusterUtils.GetClusterFromGin(ctx)
	bin := cluster.Bin
	beta := cluster.Beta
	clusterName := cluster.ClusterName

	// 同世界防重入：启动/停止中拒绝新操作（异步化后必须互斥，否则停/拉交叉打架）
	opKey := clusterName + "/" + levelName
	if !beginLevelOp(opKey, "start") {
		ctx.JSON(http.StatusOK, vo.Response{Code: 409, Msg: levelName + " 正在启动/停止中，请稍候"})
		return
	}

	// 闸门同步快失败（拒绝即释放）；重活（停旧+拉起，含 3 秒停等）后台执行，API 秒回
	service.StartLog.Progress(2, "准备启动 %s", levelName)
	if err := mod.CheckIAWorldStart(clusterName, levelName); err != nil {
		endLevelOp(opKey)
		service.StartLog.Logf("启动被拒 %s/%s: %s", clusterName, levelName, err)
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	startBefore(ctx)
	if ctx.IsAborted() {
		endLevelOp(opKey)
		return
	}
	service.StartLog.Progress(4, "配置检查通过 %s", levelName)

	go func() {
		defer endLevelOp(opKey)
		gameService.StartLevel(clusterName, levelName, bin, beta)
	}()

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "start " + clusterName + " " + levelName + " success",
		Data: nil,
	})
}

// Stop 停止世界
func (g *GameLevel2Api) Stop(ctx *gin.Context) {
	levelName := ctx.Query("levelName")
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	opKey := clusterName + "/" + levelName
	if !beginLevelOp(opKey, "stop") {
		ctx.JSON(http.StatusOK, vo.Response{Code: 409, Msg: levelName + " 正在启动/停止中，请稍候"})
		return
	}
	service.StartLog.Progress(30, "正在停止 %s", levelName)
	go func() {
		defer endLevelOp(opKey)
		gameService.StopLevel(clusterName, levelName)
		service.StartLog.Progress(100, "%s 已停止", levelName)
	}()

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "start " + clusterName + " " + levelName + " success",
		Data: nil,
	})
}

// Start 启动世界
func (g *GameLevel2Api) StartAll(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	startBefore(ctx) // I8: 一键启动同走体检闸门与 LuaJIT 应用
	if ctx.IsAborted() {
		return
	}
	clusterName := cluster.ClusterName

	opKey := clusterName + "/*"
	if !beginLevelOp(opKey, "startAll") {
		ctx.JSON(http.StatusOK, vo.Response{Code: 409, Msg: "一键启动/停止进行中，请稍候"})
		return
	}

	service.StartLog.Progress(2, "准备一键启动全部世界")

	// 闸门:任一 IA 世界未订阅 IA 模组则拒绝一键启动（与单世界启动一致）
	for _, w := range gameLevel2Service.GetLevelList(clusterName) {
		if err := mod.CheckIAWorldStart(clusterName, w.Uuid); err != nil {
			endLevelOp(opKey)
			service.StartLog.Logf("一键启动被拒 %s/%s: %s", clusterName, w.Uuid, err)
			ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
			return
		}
	}

	service.StartLog.Progress(4, "配置检查通过, 逐个拉起全部世界")
	go func() {
		defer endLevelOp(opKey)
		gameService.StartGame(clusterName)
	}()

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "start all success",
		Data: nil,
	})
}

// Stop 停止世界
func (g *GameLevel2Api) StopAll(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	opKey := clusterName + "/*"
	if !beginLevelOp(opKey, "stopAll") {
		ctx.JSON(http.StatusOK, vo.Response{Code: 409, Msg: "一键启动/停止进行中，请稍候"})
		return
	}
	service.StartLog.Progress(30, "正在停止全部世界")
	go func() {
		defer endLevelOp(opKey)
		gameService.StopGame(clusterName)
		service.StartLog.Progress(100, "全部世界已停止")
	}()

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "stop all success",
		Data: nil,
	})
}

// GetClusterIni 发送房间配置
func (g *GameLevel2Api) GetClusterIni(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	clusterIni := ClusterIni{}
	clusterIni.Cluster = homeService.GetClusterIni(clusterName)
	clusterIni.Token = homeService.GetClusterToken(clusterName)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: &clusterIni,
	})
}

// SaveClusterIni 保存房间配置
func (g *GameLevel2Api) SaveClusterIni(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	clusterIni := ClusterIni{}
	err := ctx.ShouldBind(&clusterIni)
	if err != nil {
		abort500f(ctx, "参数解析错误", err)
		return
	}
	homeService.SaveClusterIni(clusterName, clusterIni.Cluster)
	homeService.SaveClusterToken(clusterName, clusterIni.Token)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: &clusterIni,
	})
}

// SendCommand 发送世界指令
func (g *GameLevel2Api) SendCommand(ctx *gin.Context) {
	var payload struct {
		LevelName string `json:"levelName"`
		Command   string `json:"command"`
	}
	err := ctx.ShouldBind(&payload)
	if err != nil {
		abort500f(ctx, err)
		return
	}
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	consoleService.SendCommand(clusterName, payload.LevelName, payload.Command)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

// GetAllOnlinePlayers 获取所有在线玩家
func (g *GameLevel2Api) GetAllOnlinePlayers(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	playerList := playerListWithTimeout(clusterName, "#ALL_LEVEL")
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: playerList,
	})
}

// GetOnlinePlayers 获取在线玩家
func (g *GameLevel2Api) GetOnlinePlayers(ctx *gin.Context) {
	levelName := ctx.Query("levelName")
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	playerList := playerListWithTimeout(clusterName, levelName)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: playerList,
	})
}

// GetAdministrators 获取管理员
func (g *GameLevel2Api) GetAdministrators(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	adminList := playerService.GetDstAdminList(clusterName)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: adminList,
	})
}

// GetWhitelist 获取白名单
func (g *GameLevel2Api) GetWhitelist(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	whitelist := playerService.GetDstWhitelistPlayerList(clusterName)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: whitelist,
	})
}

// GetBlacklist 获取黑名单
func (g *GameLevel2Api) GetBlacklist(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	blacklist := playerService.GetDstBlacklistPlayerList(clusterName)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: blacklist,
	})
}

// SaveBlacklist 保存黑名单
func (g *GameLevel2Api) SaveBlacklist(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	blacklistVO := vo.NewBlacklistVO()
	err := ctx.BindJSON(blacklistVO)
	if err != nil {
		abort500f(ctx, "参数解析错误")
		return
	}

	playerService.SaveBlacklist(clusterName, blacklistVO.Blacklist)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

// SaveWhitelist 保存白名单
func (g *GameLevel2Api) SaveWhitelist(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	whitelist := vo.NewWhitelistVO()
	err := ctx.BindJSON(whitelist)
	if err != nil {
		abort500f(ctx, "参数解析错误")
		return
	}
	playerService.SaveWhitelist(clusterName, whitelist.Whitelist)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

// SaveAdminlist 保存管理员
func (g *GameLevel2Api) SaveAdminlist(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	adminlist := vo.NewAdminListVO()
	err := ctx.BindJSON(adminlist)
	if err != nil {
		abort500f(ctx, "参数解析错误")
		return
	}

	playerService.SaveAdminlist(clusterName, adminlist.AdminList)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameLevel2Api) GetLevelList(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gameLevel2Service.GetLevelList(clusterName),
	})
}

func (g *GameLevel2Api) SaveLevelsList(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	var body struct {
		Levels []level.World `json:"levels"`
	}
	err := ctx.ShouldBind(&body)
	if err != nil {
		abort500f(ctx, "参数解析错误", err)
		return
	}

	// 闸门:海难/火山是 IA 模组世界,世界选项必须建立在已订阅基础上
	for i := range body.Levels {
		if err := mod.CheckIAWorldWrite(clusterName, body.Levels[i].Leveldataoverride); err != nil {
			ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
			return
		}
	}

	err = gameLevel2Service.UpdateLevels(clusterName, body.Levels)
	if err != nil {
		abort500f(ctx, "更新世界配置失败", err)
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameLevel2Api) DeleteLevel(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	levelName := ctx.Query("levelName")

	err := gameLevel2Service.DeleteLevel(clusterName, levelName)
	if err != nil {
		abort500f(ctx, "删除世界失败", err)
		return
	}

	autoCheck.Manager.DeleteAutoCheck(levelName)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (g *GameLevel2Api) CreateNewLevel(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	newLevel := &level.World{}
	err := ctx.ShouldBind(newLevel)
	if err != nil {
		abort500f(ctx, "参数解析错误", err)
		return
	}

	// 闸门:海难/火山是 IA 模组世界,模组未订阅/下载则世界无法加载
	if err := mod.CheckIAWorldWrite(clusterName, newLevel.Leveldataoverride); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}

	err = gameLevel2Service.CreateLevel(clusterName, newLevel)
	if err != nil {
		abort500f(ctx, "创建世界失败", err)
		return
	}

	autoCheck.Manager.AddAutoCheckTasks(model.AutoCheck{
		ClusterName: clusterName,
		LevelName:   newLevel.LevelName,
		Uuid:        newLevel.Uuid,
		Enable:      0,
		Interval:    10,
		CheckType:   consts.LEVEL_DOWN,
	})

	autoCheck.Manager.AddAutoCheckTasks(model.AutoCheck{
		ClusterName: clusterName,
		LevelName:   newLevel.LevelName,
		Uuid:        newLevel.Uuid,
		Enable:      0,
		Interval:    10,
		CheckType:   consts.LEVEL_MOD,
	})

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: newLevel,
	})
}

func (g *GameLevel2Api) GetScanUDPPorts(ctx *gin.Context) {
	ports, err := findFreeUDPPorts(10998, 11038)
	if err != nil {
		abort500f(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: ports,
	})
}

func findFreeUDPPorts(startPort, endPort int) ([]int, error) {
	var freePorts []int

	for port := startPort; port <= endPort; port++ {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
		if err == nil {
			conn.Close()
			freePorts = append(freePorts, port)
		}
	}

	return freePorts, nil
}

// CreateIAPreset 岛屿冒险(Island Adventures)世界预设
// mode=overlay: 追加 Shipwrecked+Volcano；mode=standalone: 独立 IA 存档（Shipwrecked 主分片）
func (g *GameLevel2Api) CreateIAPreset(ctx *gin.Context) {
	mode := ctx.Query("mode")
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	// 闸门:海难/火山是 IA 模组世界,模组未订阅/下载则世界无法加载
	if err := mod.CheckIAWorldWrite(clusterName, `location="shipwrecked"`); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	worlds := levelConfigUtils.GenerateIAPreset(mode)
	if worlds == nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "mode 必须是 overlay 或 standalone"})
		return
	}
	if mode == "standalone" {
		for _, l := range gameLevel2Service.GetLevelList(clusterName) {
			if err := gameLevel2Service.DeleteLevel(clusterName, l.Uuid); err != nil {
				ctx.JSON(http.StatusInternalServerError, vo.Response{Code: 500, Msg: "清理旧世界失败: " + err.Error()})
				return
			}
		}
	}
	for i := range worlds {
		if err := gameLevel2Service.CreateLevel(clusterName, &worlds[i]); err != nil {
			log.Println("创建 IA 世界失败", err)
			ctx.JSON(http.StatusInternalServerError, vo.Response{Code: 500, Msg: err.Error()})
			return
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: worlds})
}

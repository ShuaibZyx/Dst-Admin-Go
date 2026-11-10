package api

import (
	"crypto/rand"
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/levelConfigUtils"
	"dst-admin-go/utils/netUtils"
	"dst-admin-go/utils/systemUtils"
	"dst-admin-go/vo"
	"dst-admin-go/vo/level"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ShareApi struct{}

type ShareCluster struct {
	ClusterIni   string        `json:"cluster_ini"`
	ClusterToken string        `json:"cluster_token"`
	Adminlist    string        `json:"adminlist"`
	Blocklist    string        `json:"blocklist"`
	Whitelist    string        `json:"whitelist"`
	LevelJson    string        `json:"level_json"`
	Levels       []level.World `json:"levels"`
}

type KeyCer struct {
	Key    string `json:"key"`
	Enable string `json:"enable"`
	Ip     string `json:"ip"`
	Port   string `json:"port"`
}

func checkKey(key string) bool {
	// TODO 这个文件要加密 aec
	keyContent, err := fileUtils.ReadFile("./key")
	if err != nil {
		return false
	}
	if len(keyContent) < 2 {
		return false
	}
	enable := keyContent[:1]
	if enable == "0" {
		return false
	}
	key2 := strings.Replace(keyContent[1:], "\n", "", -1)
	log.Println("key2", key2, "key", key)

	return key2 == key

}

// 生成随机UUID
func generateUUID() string {
	// 生成随机字节序列
	var uuid [16]byte
	_, err := rand.Read(uuid[:])
	if err != nil {
		panic(err)
	}

	// 设置UUID版本和变体
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0xbf) | 0x80 // Variant 1

	// 将UUID转换为字符串并返回
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func ReflushKeyInfo() KeyCer {

	keyCer := KeyCer{}
	fileUtils.CreateFileIfNotExists("./key")
	ip, _ := systemUtils.GetPublicIP()
	keyCer.Ip = ip
	keyCer.Port = global.Config.Port
	keyCer.Key = generateUUID()
	keyCer.Enable = "0"

	fileUtils.WriterTXT("./key", "0"+keyCer.Key)

	return keyCer
}

func GetKeyInfo() KeyCer {

	keyCer := KeyCer{}
	fileUtils.CreateFileIfNotExists("./key")
	keyContent, err := fileUtils.ReadFile("./key")
	if err != nil {
		return KeyCer{}
	}
	if len(keyContent) < 2 {
		return KeyCer{}
	}
	enable := keyContent[:1]
	key2 := strings.Replace(keyContent[1:], "\n", "", -1)
	keyCer.Key = key2
	keyCer.Enable = enable

	ip, _ := systemUtils.GetPublicIP()
	keyCer.Ip = ip
	keyCer.Port = global.Config.Port

	return keyCer
}

func (s *ShareApi) GetKeyCerApi(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: GetKeyInfo(),
	})
}

func (s *ShareApi) ReflushKeyCerApi(ctx *gin.Context) {

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: ReflushKeyInfo(),
	})
}

func (s *ShareApi) EnableKeyCerApi(ctx *gin.Context) {
	keyInfo := GetKeyInfo()
	enable := ctx.DefaultQuery("enable", "0")

	fileUtils.WriterTXT("./key", enable+keyInfo.Key)
	keyInfo.Enable = enable
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: keyInfo,
	})
}

func (s *ShareApi) ShareClusterConfig(ctx *gin.Context) {

	key := ctx.Query("key")
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName

	if !checkKey(key) {
		log.Panicln("key 验证不通过")
	}
	if !fileUtils.Exists(dstUtils.GetClusterIniPath(clusterName)) {
		log.Panicln("存档不存在")
	}

	// 检查 key 的正确性
	shareCluster := ShareCluster{}
	clusterIni, _ := fileUtils.ReadFile(dstUtils.GetClusterIniPath(clusterName))
	clusterToken, _ := fileUtils.ReadFile(dstUtils.GetClusterTokenPath(clusterName))
	adminlist, _ := fileUtils.ReadFile(dstUtils.GetAdminlistPath(clusterName))
	blocklist, _ := fileUtils.ReadFile(dstUtils.GetBlocklistPath(clusterName))
	whitelist, _ := fileUtils.ReadFile(dstUtils.GetWhitelistPath(clusterName))
	levelJson, _ := fileUtils.ReadFile(filepath.Join(dstUtils.GetClusterBasePath(clusterName), "level.json")) // C5

	shareCluster.ClusterIni = clusterIni
	shareCluster.ClusterToken = clusterToken
	shareCluster.Adminlist = adminlist
	shareCluster.Blocklist = blocklist
	shareCluster.Whitelist = whitelist
	shareCluster.LevelJson = levelJson
	var levels []level.World
	levelConfig, _ := levelConfigUtils.GetLevelConfig(clusterName)
	for i := range levelConfig.LevelList {
		levelInfo := levelConfig.LevelList[i]
		world := homeService.GetLevel(clusterName, levelInfo.File)
		levels = append(levels, *world)
	}
	shareCluster.Levels = levels

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: shareCluster,
	})

}

func (s *ShareApi) ImportClusterConfig(ctx *gin.Context) {

	// TODO 停止之前的宕机检查等功能
	// autoCheck.Manager.ReStart()

	var payload struct {
		Url string `json:"url"`
	}
	err := ctx.ShouldBind(&payload)
	if err != nil {
		log.Println(err)
	}
	log.Println("url", payload.Url)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})

}

// DirectConnectConfig 直连配置写入口（I5）
func (s *ShareApi) DirectConnectConfig(ctx *gin.Context) {
	var body struct {
		IPMode       string `json:"ip_mode"`
		ExternalIP   string `json:"external_ip"`
		ExternalPort string `json:"external_port"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	saveKV("ip_mode", body.IPMode)
	saveKV("external_ip", body.ExternalIP)
	saveKV("external_port", body.ExternalPort)
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: body})
}

// DirectConnect 直连代码（IP 三态 x 外部端口）
// external_port（KV，可空）：NAT/FRP 映射端口，空则用世界 server_port
func (s *ShareApi) DirectConnect(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	levelName := ctx.DefaultQuery("levelName", "Master")
	if !fileUtils.ValidName(levelName) { // M-9
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal levelName"})
		return
	}

	var modeKv, ipKv model.KV
	database.DB.Where("key = ?", "ip_mode").Find(&modeKv)
	database.DB.Where("key = ?", "external_ip").Find(&ipKv)
	mode := modeKv.Value
	if mode == "" {
		mode = netUtils.ModeAutoDetect
	}
	ip, err := netUtils.ResolveIP(mode, ipKv.Value)
	if err != nil {
		ip, err = netUtils.ResolveIP(netUtils.ModeBind, "") // 自动探测失败降级
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, vo.Response{Code: 500, Msg: "IP 解析失败: " + err.Error()})
			return
		}
	}

	iniPath := filepath.Join(dstUtils.GetClusterBasePath(clusterName), levelName, "server.ini")
	ini, _ := os.ReadFile(iniPath)
	internalPort := 0
	for _, line := range strings.Split(string(ini), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "server_port") {
			fmt.Sscanf(line[strings.Index(line, "=")+1:], "%d", &internalPort)
		}
	}
	if internalPort == 0 {
		internalPort = 10999
	}
	externalPort := internalPort
	var extKv model.KV
	database.DB.Where("key = ?", "external_port").Find(&extKv)
	if extKv.Value != "" {
		fmt.Sscanf(extKv.Value, "%d", &externalPort)
	}

	cb, _ := os.ReadFile(dstUtils.GetClusterIniPath(clusterName))
	password := ""
	for _, line := range strings.Split(string(cb), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "cluster_password") {
			password = strings.TrimSpace(line[strings.Index(line, "=")+1:])
		}
	}

	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{
		"ip":           ip,
		"internalPort": internalPort,
		"externalPort": externalPort,
		"password":     password,
		"cConnectCode": netUtils.FormatCConnect(ip, externalPort, password),
		"steamUrl":     netUtils.FormatSteamURL(ip, externalPort, password),
	}})
}

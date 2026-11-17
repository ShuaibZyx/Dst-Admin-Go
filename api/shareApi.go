package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/netUtils"
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

package api

import (
	"dst-admin-go/config/global"
	"dst-admin-go/service"
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type DstConfigApi struct {
}

var initEvnService = service.InitService{}

// dstConfigResponse 原始配置 + 计算字段（§12 路径预览 / container_mode）
type dstConfigResponse struct {
	dstConfigUtils.DstConfig
	Container_mode bool              `json:"container_mode"`
	Resolved       map[string]string `json:"resolved"`
}

func (d *DstConfigApi) GetDstConfig(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	parent := cfg.Parent_path
	psr := cfg.Persistent_storage_root
	if parent != "" {
		psr = dstConfigUtils.ResolvePath(parent, psr, dstConfigUtils.DefaultWorlds)
	}
	resp := dstConfigResponse{
		DstConfig:      cfg,
		Container_mode: containerMode(),
		Resolved: map[string]string{
			"steamcmd":                dstConfigUtils.ResolvePath(parent, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd),
			"force_install_dir":       dstConfigUtils.ResolvePath(parent, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir),
			"backup":                  dstConfigUtils.ResolvePath(parent, cfg.Backup, dstConfigUtils.DefaultBackup),
			"mod_download_path":       dstConfigUtils.ResolvePath(parent, cfg.Mod_download_path, dstConfigUtils.DefaultMod),
			"persistent_storage_root": psr,
		},
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: resp,
	})
}

func (d *DstConfigApi) SaveDstConfig(ctx *gin.Context) {
	dstConfig := dstConfigUtils.NewDstConfig()
	err := ctx.Bind(dstConfig)
	if err != nil {
		log.Panicln(err)
	}
	dstConfigUtils.SaveDstConfig(dstConfig)
	global.Collect.ReCollect(filepath.Join(dstUtils.GetKleiDstPath(), dstConfig.Cluster), dstConfig.Cluster)
	initEvnService.InitBaseLevel(dstConfig, "默认初始化的世界", "", true)

	autoCheck.Manager.ReStart(dstConfig.Cluster)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "save dst_config success",
		Data: nil,
	})
}

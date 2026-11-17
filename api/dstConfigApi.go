package api

import (
	"dst-admin-go/config/global"
	"dst-admin-go/service"
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type DstConfigApi struct {
}

var initEvnService = service.InitService{}

// containerMode Docker 镜像内禁用导入等迁移功能
func containerMode() bool {
	return os.Getenv("DOCKER_CONTAINER") != ""
}

// dstConfigResponse 原始配置 + 计算字段（路径预览 / container_mode）
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
		abort500f(ctx, err)
		return
	}
	dstConfigUtils.SaveDstConfig(dstConfig)
	// 新选的根目录可能尚不存在：先递归建出存档目录，避免后续 open 失败
	if err := os.MkdirAll(dstUtils.GetKleiDstPath(), os.ModePerm); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	global.Collect.ReCollect(filepath.Join(dstUtils.GetKleiDstPath(), dstConfig.Cluster), dstConfig.Cluster)
	// 不再自动初始化空档骨架：曾导致删掉的存档在保存配置后再生、挡住宿档搬入

	autoCheck.Manager.ReStart(dstConfig.Cluster)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "save dst_config success",
		Data: nil,
	})
}

// GetClusters 列出存档目录下的所有存档（cluster）及当前存档
func (d *DstConfigApi) GetClusters(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	base := dstUtils.GetKleiDstPath()
	entries, err := os.ReadDir(base)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"clusters": []string{}, "current": cfg.Cluster}})
		return
	}
	clusters := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(base, e.Name())
		subs, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		isCluster := false
		for _, s := range subs {
			if s.Name() == "cluster.ini" || (s.IsDir() && (s.Name() == "Master" || s.Name() == "Caves")) {
				isCluster = true
				break
			}
		}
		if isCluster {
			clusters = append(clusters, e.Name())
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"clusters": clusters, "current": cfg.Cluster}})
}

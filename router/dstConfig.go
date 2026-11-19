package router

import (
	"dst-admin-go/api"
	"github.com/gin-gonic/gin"
)

func initDstConfigRouter(router *gin.RouterGroup) {

	dstConfigApi := api.DstConfigApi{}
	dstConfig := router.Group("/api/dst/config")
	{
		dstConfig.GET("", dstConfigApi.GetDstConfig)
		dstConfig.POST("", dstConfigApi.SaveDstConfig)
		dstConfig.GET("/clusters", dstConfigApi.GetClusters)
	}

	oneClick := api.OneClickInstallApi{}
	router.GET("/api/dst/install/oneclick", oneClick.Install)
	router.GET("/api/dst/install/status", oneClick.InstallStatus)

	modsDl := api.ModsDownloadApi{}
	router.GET("/api/dst/mods/download", modsDl.DownloadCurrent)
	router.GET("/api/dst/mods/status", modsDl.DownloadStatus)

	fsApi := api.FsApi{}
	router.GET("/api/fs/dirs", fsApi.ListDirs)
	router.POST("/api/fs/mkdir", fsApi.Mkdir)
	router.GET("/api/fs/open", fsApi.Open)

	kvApi := api.KvApi{}
	kv := router.Group("/api/kv")
	{
		kv.GET("", kvApi.GetKv)
		kv.POST("", kvApi.SaveKv)
	}
}

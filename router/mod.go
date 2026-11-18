package router

import (
	"dst-admin-go/api"
	"github.com/gin-gonic/gin"
)

func initModRouter(router *gin.RouterGroup) {

	modApi := api.ModApi{}
	mod := router.Group("/api/mod")
	{
		mod.GET("/search", modApi.SearchModList)
		mod.GET("/subscribed/:modId", modApi.GetModSubscribed)
		mod.GET("/:modId", modApi.GetModInfo)
		mod.PUT("/:modId", modApi.UpdateMod)
		mod.GET("", modApi.GetMyModList)
		mod.DELETE("/:modId", modApi.DeleteMod)
		mod.DELETE("/setup/workshop", modApi.DeleteSetupWorkshop)
		mod.GET("/modinfo/:modId", modApi.GetModInfoFile)
		mod.POST("/modinfo", modApi.SaveModInfoFile)
		mod.POST("/modinfo/file", modApi.AddModInfoFile)

		mod.PUT("/modinfo", modApi.UpdateAllModInfos)

		subApi := api.SubscriptionApi{}
		mod.GET("/subscription", subApi.List)
		mod.GET("/subscription/detail/:id", subApi.Detail)
		mod.POST("/subscription/sync", subApi.Sync)
		mod.POST("/subscription/add", subApi.Add)
		mod.DELETE("/subscription/:id", subApi.Delete)

		mod.GET("/ugc/acf", modApi.GetUgcModAcf)
		mod.DELETE("/ugc", modApi.DeleteUgcModFile)
	}

}

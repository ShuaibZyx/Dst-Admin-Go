package router

import (
	"dst-admin-go/api"
	"github.com/gin-gonic/gin"
)

func initDstGenMapRouter(router *gin.RouterGroup) {

	dstMapApi := api.DstMapApi{}
	dstMap := router.Group("/api/dst/map/")
	{
		dstMap.GET("/gen", dstMapApi.GenDstMap)
		dstMap.GET("image", dstMapApi.GetDstMapImage)
	}

}

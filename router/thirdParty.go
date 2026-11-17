package router

import (
	"dst-admin-go/api"
	"github.com/gin-gonic/gin"
)

func initThirdPartyRouter(router *gin.RouterGroup) {

	thirdPartyApi := api.ThirdPartyApi{}
	//第三方api转发
	router.GET("/api/dst/version", thirdPartyApi.GetDstVersion)

	// 配置路由
	router.Any("/api/dst-static/*filepath", thirdPartyApi.GiteeProxy)
}

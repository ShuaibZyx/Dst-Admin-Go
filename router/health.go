package router

import (
	"dst-admin-go/api"

	"github.com/gin-gonic/gin"
)

func initHealthRouter(router *gin.RouterGroup) {
	h := api.HealthCheckApi{}
	group := router.Group("/api/health")
	{
		group.GET("/check", h.Check)
		group.GET("/last", h.Last)
		group.POST("/strict", h.SetStrict)
	}
}

package api

import (
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/vo"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StorageApi struct{}

// GetStorageSummary 世界存档大小 + 备份数量/总大小(服务器信息卡展示)
func (s *StorageApi) GetStorageSummary(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: service.GetStorageSummary(cluster.ClusterName),
	})
}

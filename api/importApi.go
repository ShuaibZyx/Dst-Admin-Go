package api

import (
	"dst-admin-go/service/migration"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// ImportApi cluster 存档导入（旧配置唯一的兼容入口）
type ImportApi struct{}

type importClusterRequest struct {
	Source  string `json:"source"`
	Cluster string `json:"cluster"`
}

// ImportCluster POST /api/dst/config/cluster/import
func (a *ImportApi) ImportCluster(ctx *gin.Context) {
	if os.Getenv("DOCKER_CONTAINER") != "" {
		ctx.JSON(http.StatusOK, vo.Response{Code: 403, Msg: "container mode disabled"})
		return
	}
	var req importClusterRequest
	if err := ctx.Bind(&req); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	cfg := dstConfigUtils.GetDstConfig()
	target, err := migration.ImportCluster(req.Source, cfg.Parent_path, req.Cluster, cfg.Beta)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "import success",
		Data: gin.H{"target": target},
	})
}

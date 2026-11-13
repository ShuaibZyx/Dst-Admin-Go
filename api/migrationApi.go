package api

import (
	"dst-admin-go/service"
	"dst-admin-go/service/migration"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type MigrationApi struct{}

type migrationExecuteRequest struct {
	IDs []string `json:"ids"`
}

// containerMode Docker 镜像内禁用迁移功能（§12 裁断）
func containerMode() bool {
	return os.Getenv("DOCKER_CONTAINER") != ""
}

func gameIsRunning(cluster string) bool {
	var gs service.GameService
	return gs.GetLevelStatus(cluster, "Master") || gs.GetLevelStatus(cluster, "Caves")
}

func (m *MigrationApi) GetMigrationScan(ctx *gin.Context) {
	if containerMode() {
		ctx.JSON(http.StatusOK, vo.Response{Code: 403, Msg: "container mode disabled"})
		return
	}
	cfg := dstConfigUtils.GetDstConfig()
	gameRunning := gameIsRunning(cfg.Cluster)
	units, err := migration.Scan(cfg, gameRunning)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	if units == nil {
		units = []migration.MigrationUnit{}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gin.H{"units": units, "gameRunning": gameRunning},
	})
}

func (m *MigrationApi) ExecuteMigration(ctx *gin.Context) {
	if containerMode() {
		ctx.JSON(http.StatusOK, vo.Response{Code: 403, Msg: "container mode disabled"})
		return
	}
	var req migrationExecuteRequest
	if err := ctx.Bind(&req); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	cfg := dstConfigUtils.GetDstConfig()
	gameRunning := gameIsRunning(cfg.Cluster)
	units, err := migration.Scan(cfg, gameRunning)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	results := migration.Migrate(units, req.IDs, gameRunning)

	// 成功项改写配置为相对默认值并落盘（§11.4）
	done := map[string]bool{}
	for _, r := range results {
		if r.OK {
			done[r.ID] = true
		}
	}
	var succeeded []migration.MigrationUnit
	for _, u := range units {
		if done[u.ID] {
			succeeded = append(succeeded, u)
		}
	}
	if len(succeeded) > 0 {
		newCfg := cfg
		migration.RewriteConfigAfterMigration(&newCfg, succeeded)
		dstConfigUtils.SaveDstConfig(&newCfg)
	}
	if results == nil {
		results = []migration.MigrateResult{}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gin.H{"results": results},
	})
}

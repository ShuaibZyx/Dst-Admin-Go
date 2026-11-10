package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service/healthCheck"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	kvHealthLast   = "health_check_last"
	kvHealthStrict = "health_strict"
)

type HealthCheckApi struct{}

// Check 立即体检当前集群存档
func (h *HealthCheckApi) Check(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	basePath := dstUtils.GetClusterBasePath(cluster.ClusterName)
	items := healthCheck.Run(basePath)
	saveHealthLast(items)
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: items})
}

// Last 上次体检结果
func (h *HealthCheckApi) Last(ctx *gin.Context) {
	var kv model.KV
	database.DB.Where("key = ?", kvHealthLast).Find(&kv)
	var items []healthCheck.HealthItem
	if kv.Value != "" {
		_ = json.Unmarshal([]byte(kv.Value), &items)
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: items})
}

// GetStrict 当前严格模式（M-9: 前端回读）
func (h *HealthCheckApi) GetStrict(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"strict": HealthStrictEnabled()}})
}

// SetStrict 体检严格模式开关（仅手动开服生效）
func (h *HealthCheckApi) SetStrict(ctx *gin.Context) {
	var body struct {
		Strict bool `json:"strict"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	v := "false"
	if body.Strict {
		v = "true"
	}
	saveKV(kvHealthStrict, v)
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: body})
}

func saveHealthLast(items []healthCheck.HealthItem) {
	b, err := json.Marshal(items)
	if err != nil {
		return
	}
	saveKV(kvHealthLast, string(b))
}

func saveKV(key, value string) {
	var kv model.KV
	database.DB.Where("key = ?", key).Find(&kv)
	kv.Key = key
	kv.Value = value
	database.DB.Save(&kv)
}

// HealthStrictEnabled 严格模式（默认 false：警告不阻断）
func HealthStrictEnabled() bool {
	var kv model.KV
	database.DB.Where("key = ?", kvHealthStrict).Find(&kv)
	return kv.Value == "true"
}

// HealthHasError 判断体检结果是否含 error 级问题
func HealthHasError(items []healthCheck.HealthItem) bool {
	for _, it := range items {
		if it.Status == healthCheck.StatusError {
			return true
		}
	}
	return false
}

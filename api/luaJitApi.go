package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

const kvLuaJit = "lua_jit"

// isModSubscribedFn 订阅检查入口(可替换,便于测试)
var isModSubscribedFn = mod.IsModSubscribed

type LuaJitApi struct{}

// Get 返回当前 LuaJIT 模组开关状态
func (l *LuaJitApi) Get(ctx *gin.Context) {
	var kv model.KV
	database.DB.Where("key = ?", kvLuaJit).Find(&kv)
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"enabled": kv.Value == "true"}})
}

// Set 开关 LuaJIT 模组：写 KV 并对所有世界 modoverrides.lua 生效
func (l *LuaJitApi) Set(ctx *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}

	cluster := clusterUtils.GetClusterFromGin(ctx)

	// 闸门:启用 LuaJIT 需先订阅 workshop-3444078585,未订阅则拦截;关闭不设限(退订后仍可关闭)
	subscribed := isModSubscribedFn(cluster.ClusterName, mod.LuaJitModId)
	if err := mod.CheckLuaJitToggle(body.Enabled, subscribed); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}

	v := "false"
	if body.Enabled {
		v = "true"
	}
	saveKV(kvLuaJit, v)

	basePath := dstUtils.GetClusterBasePath(cluster.ClusterName)
	entries, err := filepath.Glob(filepath.Join(basePath, "*", "modoverrides.lua"))
	if err == nil {
		for _, p := range entries {
			_ = mod.EnsureLuaJitModInFile(p, body.Enabled)
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"enabled": body.Enabled}})
}

// LuaJitEnabled 供开服前检查
func LuaJitEnabled() bool {
	var kv model.KV
	database.DB.Where("key = ?", kvLuaJit).Find(&kv)
	return kv.Value == "true"
}

// ApplyLuaJit 开服前确保各世界 modoverrides 与开关一致
func ApplyLuaJit(clusterName string) {
	basePath := dstUtils.GetClusterBasePath(clusterName)
	entries, err := filepath.Glob(filepath.Join(basePath, "*", "modoverrides.lua"))
	if err != nil {
		return
	}
	for _, p := range entries {
		_ = mod.EnsureLuaJitModInFile(p, LuaJitEnabled())
	}
}

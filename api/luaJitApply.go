package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/dstUtils"
	"path/filepath"
)

const kvLuaJit = "lua_jit"

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

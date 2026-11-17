package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// LuaJIT 开关闸门 API 层：未订阅时启用须 400 且不落库；已订阅放行；关闭不设限(退订后仍可关)。

func setupLuaJitTest(t *testing.T) *gin.Engine {
	t.Helper()
	// 隔离副作用：切到临时目录,./dst_config 的创建与 modoverrides 写入都不污染仓库/存档
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	db, err := gorm.Open(sqlite.Open(filepath.Join(tmp, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.KV{}); err != nil {
		t.Fatal(err)
	}
	oldDB := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = oldDB
		_ = sqlDB.Close() // Windows 上不关句柄 TempDir 清理会失败
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/mod/luajit", (&LuaJitApi{}).Set)
	return r
}

func stubSubscribed(t *testing.T, v bool) {
	t.Helper()
	old := isModSubscribedFn
	isModSubscribedFn = func(clusterName, workshopId string) bool { return v }
	t.Cleanup(func() { isModSubscribedFn = old })
}

func postLuaJit(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/mod/luajit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLuaJitSetEnableWithoutSubscriptionRejected(t *testing.T) {
	r := setupLuaJitTest(t)
	stubSubscribed(t, false)

	w := postLuaJit(r, `{"enabled": true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未订阅时启用应 400, 实际 %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "订阅") {
		t.Fatalf("响应应含订阅提示, 实际 %s", w.Body.String())
	}
	var kv model.KV
	database.DB.Where("key = ?", kvLuaJit).Find(&kv)
	if kv.Value == "true" {
		t.Fatal("被拦截后不应写入 KV enabled=true")
	}
}

func TestLuaJitSetEnableWithSubscriptionAllowed(t *testing.T) {
	r := setupLuaJitTest(t)
	stubSubscribed(t, true)

	w := postLuaJit(r, `{"enabled": true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("已订阅时启用应 200, 实际 %d body=%s", w.Code, w.Body.String())
	}
}

func TestLuaJitSetDisableWithoutSubscriptionAllowed(t *testing.T) {
	r := setupLuaJitTest(t)
	stubSubscribed(t, false)

	w := postLuaJit(r, `{"enabled": false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("未订阅时关闭仍应放行 200, 实际 %d body=%s", w.Code, w.Body.String())
	}
}

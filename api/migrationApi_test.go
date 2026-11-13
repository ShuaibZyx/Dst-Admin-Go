package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupMigrationRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + t.TempDir() + "\ncluster=Cluster1\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	app := gin.New()
	dstConfigApi := DstConfigApi{}
	migrationApi := MigrationApi{}
	g := app.Group("")
	g.GET("/api/dst/config", dstConfigApi.GetDstConfig)
	mig := g.Group("/api/dst/config/migration")
	mig.GET("/scan", migrationApi.GetMigrationScan)
	mig.POST("/execute", migrationApi.ExecuteMigration)
	return app
}

func decodeData(t *testing.T, body string) (int, map[string]interface{}) {
	t.Helper()
	var resp struct {
		Code int         `json:"code"`
		Msg  string      `json:"msg"`
		Data interface{} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, body)
	}
	data, _ := resp.Data.(map[string]interface{})
	return resp.Code, data
}

func TestScanEndpointReturnsUnits(t *testing.T) {
	app := setupMigrationRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dst/config/migration/scan", nil)
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d", w.Code)
	}
	code, data := decodeData(t, w.Body.String())
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if _, ok := data["units"].([]interface{}); !ok {
		t.Errorf("units 不是数组: %T", data["units"])
	}
	if _, ok := data["gameRunning"].(bool); !ok {
		t.Errorf("gameRunning 不是 bool: %T", data["gameRunning"])
	}
}

func TestExecuteEndpointRejectsUnknownID(t *testing.T) {
	app := setupMigrationRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/dst/config/migration/execute", strings.NewReader(`{"ids":["no-such-id"]}`))
	req.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d", w.Code)
	}
	code, data := decodeData(t, w.Body.String())
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	results, ok := data["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("results = %v", data["results"])
	}
	r := results[0].(map[string]interface{})
	if r["ok"] != false {
		t.Errorf("未知 id 应 ok=false: %+v", r)
	}
}

func TestGetDstConfigIncludesContainerModeAndResolved(t *testing.T) {
	app := setupMigrationRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dst/config", nil)
	app.ServeHTTP(w, req)

	_, data := decodeData(t, w.Body.String())
	if _, ok := data["container_mode"].(bool); !ok {
		t.Errorf("缺 container_mode: %v", data["container_mode"])
	}
	resolved, ok := data["resolved"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺 resolved: %T", data["resolved"])
	}
	for _, k := range []string{"steamcmd", "force_install_dir", "backup", "mod_download_path", "persistent_storage_root"} {
		if _, ok := resolved[k]; !ok {
			t.Errorf("resolved 缺 %s", k)
		}
	}
	if data["parent_path"] == nil {
		t.Error("响应应含 parent_path 原始值")
	}
}

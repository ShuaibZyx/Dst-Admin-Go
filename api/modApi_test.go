package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// DELETE /api/mod/:modId：先全量清扫文件，成功才删库记录；文件失败返回 code 500 且带路径、库记录保留。

func setupDeleteModTest(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	cfg := "parent_path=" + tmp + "\n" +
		"steamcmd=steamcmd\n" +
		"force_install_dir=dst-dedicated-server\n" +
		"mod_download_path=mods\n" +
		"cluster=Cluster1\n"
	if err := os.WriteFile(filepath.Join(tmp, "dst_config"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(tmp, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}, &model.Cluster{}); err != nil {
		t.Fatal(err)
	}
	oldDB := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = oldDB
		_ = sqlDB.Close()
	})

	gin.SetMode(gin.TestMode)
	return tmp
}

func deleteModRouter() *gin.Engine {
	r := gin.New()
	r.DELETE("/api/mod/:modId", (&ModApi{}).DeleteMod)
	return r
}

func TestDeleteMod_RemovesFilesAndRecord(t *testing.T) {
	tmp := setupDeleteModTest(t)

	modDir := filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667")
	steamcmdDir := filepath.Join(tmp, "steamcmd", "steamapps", "workshop", "content", "322330", "3435352667")
	serverDir := filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-3435352667")
	for _, dir := range []string{modDir, steamcmdDir, serverDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "modinfo.lua"), []byte("name='x'"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// 订阅文件：删除须摘掉目标行、保留其他行
	setupFile := filepath.Join(tmp, "dst-dedicated-server", "mods", "dedicated_server_mods_setup.lua")
	if err := os.MkdirAll(filepath.Dir(setupFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(setupFile,
		[]byte("ServerModSetup(\"3435352667\")\nServerModSetup(\"1185229307\")\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&model.ModInfo{Modid: "3435352667", Name: "Island Adventures - Core"}).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/mod/3435352667", nil)
	deleteModRouter().ServeHTTP(w, req)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 200 {
		t.Fatalf("应返回 code 200, 实际 %d msg=%s", resp.Code, resp.Msg)
	}
	if _, err := os.Stat(modDir); !os.IsNotExist(err) {
		t.Fatalf("模组目录应被删除: %v", err)
	}
	if _, err := os.Stat(steamcmdDir); !os.IsNotExist(err) {
		t.Fatalf("steamcmd 残留应被删除: %v", err)
	}
	if _, err := os.Stat(serverDir); !os.IsNotExist(err) {
		t.Fatalf("mods/workshop-<id> 应被删除: %v", err)
	}
	setupTxt, err := os.ReadFile(setupFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(setupTxt), "3435352667") {
		t.Fatalf("订阅行应被摘除, 实际: %s", string(setupTxt))
	}
	if !strings.Contains(string(setupTxt), `ServerModSetup("1185229307")`) {
		t.Fatalf("其他订阅行应保留, 实际: %s", string(setupTxt))
	}
	var n int64
	database.DB.Model(&model.ModInfo{}).Where("modid = ?", "3435352667").Count(&n)
	if n != 0 {
		t.Fatalf("库记录应被删除, 实际 %d 行", n)
	}
}

func TestDeleteMod_FileFailureKeepsRecord(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("目录占用失败语义仅在 Windows 上确定")
	}
	tmp := setupDeleteModTest(t)

	locked := filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-3452907445")
	if err := os.MkdirAll(locked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "modinfo.lua"), []byte("name='x'"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(locked, "modinfo.lua"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := database.DB.Create(&model.ModInfo{Modid: "3452907445", Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/mod/3452907445", nil)
	deleteModRouter().ServeHTTP(w, req)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 500 {
		t.Fatalf("文件删除失败应返回 code 500, 实际 %d", resp.Code)
	}
	if !strings.Contains(resp.Msg, "workshop-3452907445") {
		t.Fatalf("msg 应包含失败路径, 实际: %s", resp.Msg)
	}
	var n int64
	database.DB.Model(&model.ModInfo{}).Where("modid = ?", "3452907445").Count(&n)
	if n != 1 {
		t.Fatalf("失败时库记录应保留, 实际 %d 行", n)
	}
}

// PUT /api/mod/:modId(逐条更新):更新前先全量清扫旧文件(与 DeleteMod 一致),
// 失败则整体不动、库记录保留可重试。回归背景:旧实现先删库记录再删文件,删除失败时
// 记录已丢、状态不一致;且只删统一目录,服务端 mods/workshop-<id> 旧副本残留会被游戏加载旧版本。

func updateModRouter() *gin.Engine {
	r := gin.New()
	r.PUT("/api/mod/:modId", (&ModApi{}).UpdateMod)
	return r
}

func TestUpdateMod_CleanFailureKeepsRecord(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("目录占用失败语义仅在 Windows 上确定")
	}
	tmp := setupDeleteModTest(t)
	modDir := filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(modDir, "modinfo.lua")
	if err := os.WriteFile(victim, []byte("name='x'"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(victim)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := database.DB.Create(&model.ModInfo{Modid: "3435352667", Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/mod/3435352667", nil)
	updateModRouter().ServeHTTP(w, req)

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 500 {
		t.Fatalf("清扫失败应返回 code 500, 实际 %d msg=%s", resp.Code, resp.Msg)
	}
	var n int64
	database.DB.Model(&model.ModInfo{}).Where("modid = ?", "3435352667").Count(&n)
	if n != 1 {
		t.Fatalf("清扫失败时库记录必须保留(可重试), 实际 %d 行", n)
	}
}

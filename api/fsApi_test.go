package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListDirsReturnsSubdirs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	app := gin.New()
	fsApi := FsApi{}
	app.GET("/api/fs/dirs", fsApi.ListDirs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/dirs?path="+root, nil)
	app.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Path string `json:"path"`
			Dirs []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"dirs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if resp.Code != 200 {
		t.Fatalf("code = %d, body=%s", resp.Code, w.Body.String())
	}
	if len(resp.Data.Dirs) != 1 || resp.Data.Dirs[0].Name != "alpha" {
		t.Fatalf("dirs = %+v, want [alpha]", resp.Data.Dirs)
	}
}

func TestListDirsEmptyPathUsesHome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	fsApi := FsApi{}
	app.GET("/api/fs/dirs", fsApi.ListDirs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/dirs?path=", nil)
	app.ServeHTTP(w, req)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Path string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if resp.Code != 200 || resp.Data.Path == "" {
		t.Fatalf("code=%d path=%q body=%s", resp.Code, resp.Data.Path, w.Body.String())
	}
}

func TestListDirsRootsSentinelListsDrives(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	fsApi := FsApi{}
	app.GET("/api/fs/dirs", fsApi.ListDirs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/dirs?path=__roots__", nil)
	app.ServeHTTP(w, req)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Dirs []struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"dirs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if resp.Code != 200 || len(resp.Data.Dirs) == 0 {
		t.Fatalf("code=%d dirs=%v body=%s", resp.Code, resp.Data.Dirs, w.Body.String())
	}
}

func TestMkdirCreatesNested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	fsApi := FsApi{}
	app.POST("/api/fs/mkdir", fsApi.Mkdir)

	target := filepath.Join(t.TempDir(), "DST-Server", "world")
	body, _ := json.Marshal(map[string]string{"path": target})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/fs/mkdir", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !filepath.IsAbs(target) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("目录未创建: %v", err)
	}
}

func TestOpenRejectsUnknownKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	fsApi := FsApi{}
	app.GET("/api/fs/open", fsApi.Open)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/open?kind=/etc", nil)
	app.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err, w.Body.String())
	}
	if resp.Code != 400 {
		t.Fatalf("非白名单 kind 应拒绝: code=%d body=%s", resp.Code, w.Body.String())
	}
}

func TestResolveOpenEntry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backup")

	// 根内条目（相对/绝对/嵌套）均放行
	if got, err := resolveOpenEntry(root, "a.zip"); err != nil || got != filepath.Join(root, "a.zip") {
		t.Fatalf("相对条目: got=%q err=%v", got, err)
	}
	want := filepath.Join(root, "DoNotStarveTogether", "cluster1")
	if got, err := resolveOpenEntry(root, filepath.Join("DoNotStarveTogether", "cluster1")); err != nil || got != want {
		t.Fatalf("嵌套条目: got=%q err=%v", got, err)
	}
	if got, err := resolveOpenEntry(root, want); err != nil || got != want {
		t.Fatalf("绝对路径条目: got=%q err=%v", got, err)
	}

	// 越界/根本身必须拒绝
	for _, p := range []string{
		"..",
		"../x.zip",
		filepath.Join(root, "..", "sibling.zip"),
		filepath.Dir(root),               // 根目录的父级
		root + "2",                       // 兄弟目录（前缀混淆）
		root,                             // 根本身
	} {
		if got, err := resolveOpenEntry(root, p); err == nil {
			t.Fatalf("应拒绝 %q, 却解析为 %q", p, got)
		}
	}
}

func TestOpenTarget(t *testing.T) {
	base := t.TempDir()

	// 目录条目（存档）：进入其内部
	save := filepath.Join(base, "cluster1")
	if err := os.MkdirAll(save, 0755); err != nil {
		t.Fatal(err)
	}
	if dir, sel := openTarget(save); dir != save || sel != "" {
		t.Fatalf("存档应进入内部: dir=%q sel=%q", dir, sel)
	}

	// 文件条目（备份 zip）：打开所在目录并选中
	zip := filepath.Join(base, "backup.zip")
	if err := os.WriteFile(zip, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if dir, sel := openTarget(zip); dir != base || sel != zip {
		t.Fatalf("备份应定位所在目录: dir=%q sel=%q", dir, sel)
	}
}

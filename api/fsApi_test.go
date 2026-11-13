package api

import (
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
			Path  string `json:"path"`
			Dirs  []struct {
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

package router

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// 缓存策略回归(2026-11-11 海难/火山图标事故):
// /misc/customization_shipwrecked.png 曾缺失,404 响应被 max-age=30672000 缓存一年;
// 文件补齐后用户浏览器仍用缓存 404,图标空白。要求:
//   - 仅带 hash 的静态资源(/assets/、/static/)长缓存
//   - 固定文件名资源(/misc/ 图片)与任何 404 一律 no-cache
func setupStaticApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "dist", "misc"), 0o755))
	must(os.MkdirAll(filepath.Join(dir, "dist", "assets"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "dist", "index.html"), []byte("<html></html>"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "dist", "misc", "icon.png"), []byte("png"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "dist", "misc", "data.json"), []byte("{}"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "dist", "assets", "index-hash.js"), []byte("js"), 0o644))
	t.Chdir(dir)
	app := gin.New()
	initStaticFile(app)
	return app
}

func doReq(app *gin.Engine, path string) (int, string) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	app.ServeHTTP(w, req)
	return w.Code, w.Header().Get("Cache-Control")
}

func TestStaticCacheMiscImageNotLongCached(t *testing.T) {
	app := setupStaticApp(t)
	code, cc := doReq(app, "/misc/icon.png")
	if code != 200 {
		t.Fatalf("期望 200,实际 %d", code)
	}
	if cc == "public, max-age=30672000" {
		t.Fatalf("/misc/ 固定文件名图片不应长缓存(更新后用户长期看到旧图),实际 Cache-Control: %q", cc)
	}
}

func TestStaticCache404NotLongCached(t *testing.T) {
	app := setupStaticApp(t)
	for _, p := range []string{"/misc/missing.png", "/assets/missing-hash.js"} {
		code, cc := doReq(app, p)
		if code == 200 {
			t.Fatalf("%s 期望 404,实际 %d", p, code)
		}
		if cc == "public, max-age=30672000" {
			t.Fatalf("%s 404 绝不能长缓存(浏览器会记一年,文件补齐后仍看不到),实际 Cache-Control: %q", p, cc)
		}
	}
}

func TestStaticCacheHashedAssetStillLongCached(t *testing.T) {
	app := setupStaticApp(t)
	code, cc := doReq(app, "/assets/index-hash.js")
	if code != 200 {
		t.Fatalf("期望 200,实际 %d", code)
	}
	if cc != "public, max-age=30672000" {
		t.Fatalf("带 hash 的资源应长缓存,实际 Cache-Control: %q", cc)
	}
}

func TestStaticCacheJsonNeverLongCached(t *testing.T) {
	app := setupStaticApp(t)
	code, cc := doReq(app, "/misc/data.json")
	if code != 200 {
		t.Fatalf("期望 200,实际 %d", code)
	}
	if cc == "public, max-age=30672000" {
		t.Fatalf("JSON 数据不应长缓存(F6),实际 Cache-Control: %q", cc)
	}
}

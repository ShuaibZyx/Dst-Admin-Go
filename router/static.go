package router

import (
	"dst-admin-go/utils/pathUtils"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// distStaticExists 判断 URL 对应的 dist 静态文件是否真实存在(用于避免给 404 发长缓存头)。
func distStaticExists(urlPath string) bool {
	// URL 已由 gin 路由到 /assets/ /static/ 前缀,取其相对文件名做存在性检查
	rel := filepath.FromSlash(urlPath)
	if len(rel) > 0 && (rel[0] == '/' || rel[0] == '\\') {
		rel = rel[1:]
	}
	_, err := os.Stat(pathUtils.Res("dist", filepath.Clean(rel)))
	return err == nil
}

func initStaticFile(app *gin.Engine) {

	defer func() {
		if r := recover(); r != nil {
		}
	}()
	app.Use(func(context *gin.Context) {
		// 仅带 hash 的静态资源长缓存；HTML/API/SSE 不缓存，避免更新后看到旧界面
		p := context.Request.URL.Path
		isHashedAsset := (len(p) > 8 && (p[:8] == "/assets/" || p[:8] == "/static/"))
		// F6/2026-11-11 事故修正:无 hash 的 /misc/ 固定名资源与一切 404 不长缓存——
		// 否则同 URL 换内容用户长期看旧图;缺文件时 404 被浏览器缓存一年,文件补齐后仍空白
		if isHashedAsset && distStaticExists(p) {
			context.Writer.Header().Set("Cache-Control", "public, max-age=30672000")
		} else {
			context.Writer.Header().Set("Cache-Control", "no-cache")
		}
	})
	// 注意：不要在此函数前部使用 LoadHTMLGlob 等会 panic 的注册——
	// dist 缺失时 panic 被上方 recover 吞掉，会导致后续路由全部丢失（/ 404）
	// 入口 index.html 由下方 StaticFile 原样下发，无需模板引擎
	app.Static("/assets", pathUtils.Res("dist", "assets"))
	app.Static("/misc", pathUtils.Res("dist", "misc"))
	app.Static("/static/js", pathUtils.Res("dist", "static", "js"))                         // 添加资源路径
	app.Static("/static/css", pathUtils.Res("dist", "static", "css"))                       // 添加资源路径
	app.Static("/static/img", pathUtils.Res("dist", "static", "img"))                       // 添加资源路径
	app.Static("/static/fonts", pathUtils.Res("dist", "static", "fonts"))                   // 添加资源路径
	app.Static("/static/media", pathUtils.Res("dist", "static", "media"))                   // 添加资源路径
	app.StaticFile("/favicon.ico", pathUtils.Res("dist", "favicon.ico"))                    // 添加资源路径
	app.StaticFile("/asset-manifest.json", pathUtils.Res("dist", "asset-manifest.json"))    // 添加资源路径
	app.StaticFile("/", pathUtils.Res("dist", "index.html"))
}

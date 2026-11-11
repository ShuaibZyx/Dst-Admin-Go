package router

import (
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
	_, err := os.Stat(filepath.Join("dist", filepath.Clean(rel)))
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
	app.LoadHTMLGlob("dist/index.html") // 添加入口index.html
	//r.LoadHTMLFiles("dist//*") // 添加资源路径
	app.Static("/assets", "./dist/assets")
	app.Static("/misc", "./dist/misc")
	app.Static("/static/js", "./dist/static/js")                         // 添加资源路径
	app.Static("/static/css", "./dist/static/css")                       // 添加资源路径
	app.Static("/static/img", "./dist/static/img")                       // 添加资源路径
	app.Static("/static/fonts", "./dist/static/fonts")                   // 添加资源路径
	app.Static("/static/media", "./dist/static/media")                   // 添加资源路径
	app.StaticFile("/favicon.ico", "./dist/favicon.ico")                 // 添加资源路径
	app.StaticFile("/asset-manifest.json", "./dist/asset-manifest.json") // 添加资源路径
	app.StaticFile("/", "./dist/index.html")
}

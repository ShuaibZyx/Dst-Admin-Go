package router

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func initStaticFile(app *gin.Engine) {

	defer func() {
		if r := recover(); r != nil {
		}
	}()
	app.Use(func(context *gin.Context) {
		// 仅带 hash 的静态资源长缓存；HTML/API/SSE 不缓存，避免更新后看到旧界面
		p := context.Request.URL.Path
		isHashedAsset := (len(p) > 8 && (p[:8] == "/assets/" || p[:8] == "/static/"))
		isMiscData := (len(p) > 6 && p[:6] == "/misc/" && !strings.HasSuffix(p, ".json"))
		if isHashedAsset || isMiscData { // F6: 无 hash 的 JSON 数据不长缓存
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

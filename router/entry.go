package router

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"dst-admin-go/middleware"
	"dst-admin-go/utils/pathUtils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"time"
)

// newSessionStore 登录态存储:session 数据随签名 cookie 走(而非进程内存),
// 服务重启不失效——否则每次重启面板都把已登录用户踢成 401。
func newSessionStore(secret []byte) sessions.Store {
	store := cookie.NewStore(secret)
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   int(60 * 24 * 7 * time.Minute.Seconds()),
		HttpOnly: true,
	})
	return store
}

func NewRoute() *gin.Engine {

	app := gin.Default()

	app.Use(sessions.Sessions("token", newSessionStore(loadSessionSecret()))) // A5: 不再硬编码签名密钥

	app.Use(middleware.Recover)
	// app.Use(middleware.ShellInjectionInterceptor())
	app.Use(middleware.Authentication())

	// app.Use(middleware.CheckDstHandler())

	app.GET("/hello", func(ctx *gin.Context) {
		ctx.String(200, "Hello! Dont starve together")
	})
	router := app.Group("")
	initBackupRouter(router)
	initClusterRouter(router)
	initDstConfigRouter(router)
	initFirstRouter(router)
	InitGameRouter(router)
	initLoginRouter(router)
	initModRouter(router)
	initPlayerRouter(router)
	initThirdPartyRouter(router)
	initWsRouter(router)
	initSteamRouter(router)
	initTimedTaskRouter(router)

	initAutoCheck(router)

	initWebLinkRouter(router)
	initWebhookRouter(router)

	initLevel2(router)
	initHealthRouter(router)

	initFile(router)
	initDstGenMapRouter(router)

	initStaticFile(app)

	return app
}


// A5: 首次启动生成随机 session 签名密钥并持久化(数据根,惰性解析)
func loadSessionSecret() []byte {
	secretFile := pathUtils.Data("session.secret")
	if b, err := os.ReadFile(secretFile); err == nil && len(b) >= 32 {
		return b
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Println("session secret rand error, fallback to time-based")
		b = []byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid()))
	}
	secret := hex.EncodeToString(b)
	if err := os.WriteFile(secretFile, []byte(secret), 0600); err != nil {
		log.Println("session secret save error", err)
	}
	return []byte(secret)
}

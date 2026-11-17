package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// 登录态必须挺过服务重启:session 数据随 cookie 走,重建引擎后旧 cookie 仍有效。
// 回归背景:memstore 把 session 存进程内存,重启即全部失效——用户浏览器全 401,
// 面板报"获取游戏状态异常"、更新游戏显示"失败",被迫重新登录。

func TestSessionSurvivesProcessRestart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("test-secret-for-session-store-01")

	// 第一"进程":登录发 session cookie
	app1 := gin.New()
	app1.Use(sessions.Sessions("token", newSessionStore(secret)))
	app1.GET("/login", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("username", "tester")
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
		c.String(http.StatusOK, "ok")
	})
	w1 := httptest.NewRecorder()
	app1.ServeHTTP(w1, httptest.NewRequest("GET", "/login", nil))
	setCookie := w1.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("登录未下发 session cookie")
	}

	// 第二"进程":同 secret 重建引擎,旧 cookie 必须仍然有效
	app2 := gin.New()
	app2.Use(sessions.Sessions("token", newSessionStore(secret)))
	app2.GET("/me", func(c *gin.Context) {
		s := sessions.Default(c)
		if s.Get("username") == nil {
			c.String(http.StatusUnauthorized, "unauthorized")
			return
		}
		c.String(http.StatusOK, "signed-in")
	})
	req2 := httptest.NewRequest("GET", "/me", nil)
	req2.Header.Set("Cookie", setCookie)
	w2 := httptest.NewRecorder()
	app2.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("重启后旧 cookie 应仍有效(session 随 cookie 走), got %d", w2.Code)
	}
}

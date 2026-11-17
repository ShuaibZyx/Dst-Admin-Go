package router

import (
	"net/http/httptest"
	"testing"
)

func TestFsDirsRouteRegistered(t *testing.T) {
	app := NewRoute()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/fs/dirs?path=", nil)
	app.ServeHTTP(w, req)
	t.Logf("status=%d body=%s", w.Code, w.Body.String())
	if w.Code == 404 {
		t.Fatal("/api/fs/dirs 路由未注册（404）")
	}
}

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"dst-admin-go/service/mod"

	"github.com/gin-gonic/gin"
)

// 海难/火山世界闸门 API 层:未订阅 IA 模组时,创建/保存/手动启动一律 400。

func stubIASubscribed(t *testing.T, v bool) {
	t.Helper()
	old := mod.IsSubscribedFn
	mod.IsSubscribedFn = func(clusterName, workshopId string) bool { return v }
	t.Cleanup(func() { mod.IsSubscribedFn = old })
}

func stubIALevelOverride(t *testing.T, content string) {
	t.Helper()
	old := mod.ReadLevelOverride
	mod.ReadLevelOverride = func(clusterName, levelName string) string { return content }
	t.Cleanup(func() { mod.ReadLevelOverride = old })
}

func setupLevel2Test(t *testing.T) *gin.Engine {
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

	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &GameLevel2Api{}
	r.PUT("/api/cluster/level", g.SaveLevelsList)
	r.POST("/api/cluster/level", g.CreateNewLevel)
	r.GET("/api/game/8level/start", g.Start)
	return r
}

func doReq(r *gin.Engine, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const iaLevelBody = `{"levelName":"海滩","uuid":"Shipwrecked","leveldataoverride":"return { location=\"shipwrecked\" }","modoverrides":"return {}"}`

func TestSaveLevelsListBlockedForIAWorldWithoutSubscription(t *testing.T) {
	r := setupLevel2Test(t)
	stubIASubscribed(t, false)
	w := doReq(r, "PUT", "/api/cluster/level", `{"levels":[`+iaLevelBody+`]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未订阅时保存海难世界应 400, 实际 %d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateLevelBlockedForIAWorldWithoutSubscription(t *testing.T) {
	r := setupLevel2Test(t)
	stubIASubscribed(t, false)
	w := doReq(r, "POST", "/api/cluster/level", iaLevelBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未订阅时创建海难世界应 400, 实际 %d body=%s", w.Code, w.Body.String())
	}
}

func TestStartBlockedForIAWorldWithoutSubscription(t *testing.T) {
	r := setupLevel2Test(t)
	stubIASubscribed(t, false)
	stubIALevelOverride(t, `location="shipwrecked"`)
	w := doReq(r, "GET", "/api/game/8level/start?levelName=Shipwrecked", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("未订阅时手动启动海难世界应 400, 实际 %d body=%s", w.Code, w.Body.String())
	}
}

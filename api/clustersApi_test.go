package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetClustersListsSaves(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parent := t.TempDir()
	base := filepath.Join(parent, "world", "DoNotStarveTogether")
	for name, marker := range map[string]string{"Alpha": "cluster.ini", "Beta": "Master", "bak": ""} {
		dir := filepath.Join(base, name)
		if err := os.MkdirAll(filepath.Join(dir, marker), 0755); err != nil && marker != "" {
			t.Fatal(err)
		}
		if marker == "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\npersistent_storage_root=world\ncluster=Alpha\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	app := gin.New()
	dstConfigApi := DstConfigApi{}
	app.GET("/api/dst/config/clusters", dstConfigApi.GetClusters)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/dst/config/clusters", nil)
	app.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Clusters []string `json:"clusters"`
			Current  string   `json:"current"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err, w.Body.String())
	}
	got := map[string]bool{}
	for _, c := range resp.Data.Clusters {
		got[c] = true
	}
	if !got["Alpha"] || !got["Beta"] {
		t.Fatalf("clusters = %v, want Alpha+Beta", resp.Data.Clusters)
	}
	if got["bak"] {
		t.Fatalf("杂物目录不应列出: %v", resp.Data.Clusters)
	}
	if resp.Data.Current != "Alpha" {
		t.Fatalf("current = %q, want Alpha", resp.Data.Current)
	}
}

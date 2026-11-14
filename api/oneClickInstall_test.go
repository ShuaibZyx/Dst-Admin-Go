package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"path/filepath"
	"testing"
)

func TestOneclickTargets(t *testing.T) {
	cfg := dstConfigUtils.DstConfig{
		Parent_path:       "/data/dst",
		Steamcmd:          "steamcmd",
		Force_install_dir: "dst-dedicated-server",
		Mod_download_path: "mod",
		Beta:              0,
	}
	sc, srv, mod := oneclickTargets(cfg)
	if sc != filepath.Join("/data/dst", "steamcmd") {
		t.Errorf("steamcmd = %q", sc)
	}
	if srv != filepath.Join("/data/dst", "dst-dedicated-server") {
		t.Errorf("server = %q", srv)
	}
	if mod != filepath.Join("/data/dst", "mods") {
		t.Errorf("mod = %q, want mods（旧别名归一）", mod)
	}

	// beta=1 → 服务器目录带 -beta
	cfg.Beta = 1
	_, srv, _ = oneclickTargets(cfg)
	if srv != filepath.Join("/data/dst", "dst-dedicated-server")+"-beta" {
		t.Errorf("beta server = %q", srv)
	}
}

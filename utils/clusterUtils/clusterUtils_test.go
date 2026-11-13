package clusterUtils

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeTempDstConfig(t *testing.T, lines []string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dst_config")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)
}

func TestGetClusterResolvesRelativePaths(t *testing.T) {
	parent := `D:\DST`
	writeTempDstConfig(t, []string{
		"parent_path=" + parent,
		"steamcmd=steamcmd",
		"force_install_dir=dst-dedicated-server",
		"backup=backup",
		"mod_download_path=mod",
		"bin=32",
		"beta=0",
	})

	c := GetCluster("Cluster1")
	if c.SteamCmd != filepath.Join(parent, "steamcmd") {
		t.Errorf("SteamCmd = %q, want %q", c.SteamCmd, filepath.Join(parent, "steamcmd"))
	}
	if c.ForceInstallDir != filepath.Join(parent, "dst-dedicated-server") {
		t.Errorf("ForceInstallDir = %q", c.ForceInstallDir)
	}
	if c.Backup != filepath.Join(parent, "backup") {
		t.Errorf("Backup = %q", c.Backup)
	}
	if c.ModDownloadPath != filepath.Join(parent, "mod") {
		t.Errorf("ModDownloadPath = %q", c.ModDownloadPath)
	}
	if c.Persistent_storage_root != filepath.Join(parent, "worlds") {
		t.Errorf("Persistent_storage_root = %q, want <parent>/worlds", c.Persistent_storage_root)
	}
}

func TestGetClusterKeepsAbsolutePaths(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/data/dst",
		"steamcmd=C:\\tools\\steamcmd",
		"force_install_dir=C:\\game\\dst",
		"backup=C:\\nas\\backup",
		"mod_download_path=C:\\nas\\mod",
		"persistent_storage_root=C:\\nas\\saves",
		"bin=32",
		"beta=0",
	})

	c := GetCluster("Cluster1")
	if c.SteamCmd != `C:\tools\steamcmd` {
		t.Errorf("SteamCmd = %q", c.SteamCmd)
	}
	if c.ForceInstallDir != `C:\game\dst` {
		t.Errorf("ForceInstallDir = %q", c.ForceInstallDir)
	}
	if c.Backup != `C:\nas\backup` {
		t.Errorf("Backup = %q", c.Backup)
	}
	if c.ModDownloadPath != `C:\nas\mod` {
		t.Errorf("ModDownloadPath = %q", c.ModDownloadPath)
	}
	if c.Persistent_storage_root != `C:\nas\saves` {
		t.Errorf("Persistent_storage_root = %q", c.Persistent_storage_root)
	}
}

func TestGetClusterEmptyParentLegacy(t *testing.T) {
	writeTempDstConfig(t, []string{
		"steamcmd=steamcmd",
		"force_install_dir=/abs/dst",
		"backup=/abs/backup",
		"mod_download_path=/abs/mod",
		"bin=32",
		"beta=0",
	})

	c := GetCluster("Cluster1")
	if c.SteamCmd != "steamcmd" {
		t.Errorf("SteamCmd = %q, want raw 'steamcmd'", c.SteamCmd)
	}
	if c.ForceInstallDir != "/abs/dst" {
		t.Errorf("ForceInstallDir = %q", c.ForceInstallDir)
	}
	if c.Persistent_storage_root != "" {
		t.Errorf("Persistent_storage_root = %q, want empty in legacy mode", c.Persistent_storage_root)
	}
	_ = runtime.GOOS // 保留平台相关断言扩展点
}

package clusterUtils

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
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
		t.Errorf("SteamCmd = %q", c.SteamCmd)
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
	if c.Persistent_storage_root != parent {
		t.Errorf("Persistent_storage_root = %q, want 根目录本身", c.Persistent_storage_root)
	}
	if c.Conf_dir != "world" {
		t.Errorf("Conf_dir = %q, want world", c.Conf_dir)
	}
}

func TestGetClusterStripsAbsolute(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/data/dst",
		"steamcmd=C:\\tools\\steamcmd",
		"force_install_dir=C:\\game\\dst",
		"backup=C:\\nas\\backup",
		"mod_download_path=C:\\nas\\mod",
		"bin=32",
		"beta=0",
	})

	c := GetCluster("Cluster1")
	if c.SteamCmd != filepath.Join("/data/dst", "tools", "steamcmd") {
		t.Errorf("SteamCmd = %q, want 剥成相对后拼父级", c.SteamCmd)
	}
	if c.Backup != filepath.Join("/data/dst", "nas", "backup") {
		t.Errorf("Backup = %q", c.Backup)
	}
}

func TestGetClusterEmptyParentReturnsEmpty(t *testing.T) {
	writeTempDstConfig(t, []string{
		"steamcmd=steamcmd",
		"force_install_dir=/abs/dst",
		"backup=/abs/backup",
		"bin=32",
		"beta=0",
	})

	c := GetCluster("Cluster1")
	if c.SteamCmd != "" || c.ForceInstallDir != "" || c.Backup != "" {
		t.Errorf("parent 未配置时应为空: %+v", c)
	}
}

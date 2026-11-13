package dstConfigUtils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempConfig 把 dst_config_path 指到临时文件，测试结束恢复
func withTempConfig(t *testing.T) string {
	t.Helper()
	old := dst_config_path
	p := filepath.Join(t.TempDir(), "dst_config")
	dst_config_path = p
	t.Cleanup(func() { dst_config_path = old })
	return p
}

func writeConfigFile(t *testing.T, p string, lines []string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// newTestConfig 带上 backup/mod_download_path，避免 GetDstConfig 默认值物化创建目录的副作用
func newTestConfig(parent string) *DstConfig {
	c := NewDstConfig()
	c.Parent_path = parent
	c.Backup = "/tmp/bk"
	c.Mod_download_path = "/tmp/mod"
	return c
}

func TestSaveGetRoundTripParentPath(t *testing.T) {
	withTempConfig(t)
	SaveDstConfig(newTestConfig("/data/dst"))

	got := GetDstConfig()
	if got.Parent_path != "/data/dst" {
		t.Fatalf("Parent_path = %q, want /data/dst", got.Parent_path)
	}
	if got.Last_parent_path != "" {
		t.Fatalf("Last_parent_path = %q, want empty on first set", got.Last_parent_path)
	}
}

func TestParentChangeRecordsLastParentPath(t *testing.T) {
	withTempConfig(t)
	SaveDstConfig(newTestConfig("/data/A"))
	SaveDstConfig(newTestConfig("/data/B"))

	got := GetDstConfig()
	if got.Parent_path != "/data/B" {
		t.Fatalf("Parent_path = %q, want /data/B", got.Parent_path)
	}
	if got.Last_parent_path != "/data/A" {
		t.Fatalf("Last_parent_path = %q, want /data/A", got.Last_parent_path)
	}
}

func TestParentSameSaveKeepsLastParentPath(t *testing.T) {
	withTempConfig(t)
	SaveDstConfig(newTestConfig("/data/A"))
	SaveDstConfig(newTestConfig("/data/A"))

	got := GetDstConfig()
	if got.Parent_path != "/data/A" {
		t.Fatalf("Parent_path = %q, want /data/A", got.Parent_path)
	}
	if got.Last_parent_path != "" {
		t.Fatalf("Last_parent_path = %q, want unchanged empty", got.Last_parent_path)
	}
}

func TestGetDstConfigNoMaterializeWhenParentSet(t *testing.T) {
	withTempConfig(t)
	writeConfigFile(t, dst_config_path, []string{
		"parent_path=/data/dst",
		"steamcmd=steamcmd",
		"force_install_dir=dst-dedicated-server",
		"cluster=Cluster1",
		"bin=32",
		"beta=0",
	})

	got := GetDstConfig()
	if got.Backup != "" {
		t.Fatalf("Backup = %q, want empty (no materialize when parent set)", got.Backup)
	}
	if got.Mod_download_path != "" {
		t.Fatalf("Mod_download_path = %q, want empty (no materialize when parent set)", got.Mod_download_path)
	}
}

func TestGetDstConfigLegacyMaterializeUnchanged(t *testing.T) {
	withTempConfig(t)
	writeConfigFile(t, dst_config_path, []string{
		"steamcmd=steamcmd",
		"force_install_dir=dst-dedicated-server",
		"cluster=Cluster1",
		"bin=32",
		"beta=0",
	})

	got := GetDstConfig()
	if got.Backup != filepath.Join(KleiDstPath(), "backup") {
		t.Fatalf("Backup = %q, want legacy klei-based default", got.Backup)
	}
	if got.Mod_download_path != filepath.Join(KleiDstPath(), "mod_config_download") {
		t.Fatalf("Mod_download_path = %q, want legacy klei-based default", got.Mod_download_path)
	}
}

func TestLegacyFileWithoutParentKeys(t *testing.T) {
	p := withTempConfig(t)
	writeConfigFile(t, p, []string{
		"steamcmd=/s",
		"force_install_dir=/f",
		"cluster=C",
		"backup=/b",
		"mod_download_path=/m",
		"bin=32",
		"beta=0",
	})

	got := GetDstConfig()
	if got.Parent_path != "" {
		t.Fatalf("Parent_path = %q, want empty for legacy file", got.Parent_path)
	}
	if got.Last_parent_path != "" {
		t.Fatalf("Last_parent_path = %q, want empty for legacy file", got.Last_parent_path)
	}
}

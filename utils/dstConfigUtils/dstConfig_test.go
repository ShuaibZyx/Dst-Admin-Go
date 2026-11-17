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

// newTestConfig 带上 backup/mod_download_path，避免测试副作用
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
	if got.Last_parent_path != "" {
		t.Fatalf("Last_parent_path = %q, want unchanged empty", got.Last_parent_path)
	}
}

func TestGetDstConfigNoMaterialize(t *testing.T) {
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
		t.Fatalf("Backup = %q, want empty (no materialize)", got.Backup)
	}
	if got.Mod_download_path != "" {
		t.Fatalf("Mod_download_path = %q, want empty (no materialize)", got.Mod_download_path)
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

// 配置读取必须带缓存(mtime 失效):之前每请求重读磁盘+逐行解析,
// 所有接口被文件 IO 放大到 300-400ms 平均耗时(实测日志)。
func TestGetDstConfigCachedUntilFileChanges(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dst_config")
	if err := os.WriteFile(p, []byte("parent_path=/a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	restore := SetDstConfigPathForTest(p)
	defer restore()

	if got := GetDstConfig(); got.Parent_path != "/a" {
		t.Fatalf("Parent_path = %q, want /a", got.Parent_path)
	}

	// 写新内容但把 mtime 改回原值:缓存命中应返回旧值(证明没有每次读盘)
	fi, _ := os.Stat(p)
	old := fi.ModTime()
	if err := os.WriteFile(p, []byte("parent_path=/b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if got := GetDstConfig(); got.Parent_path != "/a" {
		t.Fatalf("mtime 未变应命中缓存, got %q", got.Parent_path)
	}

	// mtime 变化后必须重读
	if err := os.WriteFile(p, []byte("parent_path=/c\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := GetDstConfig(); got.Parent_path != "/c" {
		t.Fatalf("文件变化后应重读, got %q", got.Parent_path)
	}
}

// dst_config 解析:①值含 = 号不得截断;②key 必须精确匹配(offline_cluster= 不得污染 cluster=)。
func TestGetDstConfig_ParseExactKeys(t *testing.T) {
	tmp := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	content := "parent_path=/opt/a=b\ncluster=ROGIA\noffline_cluster=true\nbackup=backups\nbin=64\n"
	if err := os.WriteFile(filepath.Join(tmp, "dst_config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := GetDstConfig()
	if cfg.Parent_path != "/opt/a=b" {
		t.Errorf("值含 = 不能截断: Parent_path=%q", cfg.Parent_path)
	}
	if cfg.Cluster != "ROGIA" {
		t.Errorf("cluster 不得被 offline_cluster 覆盖: Cluster=%q", cfg.Cluster)
	}
	if cfg.Backup != "backups" {
		t.Errorf("Backup 解析错误: %q", cfg.Backup)
	}
	if cfg.Bin != 64 {
		t.Errorf("Bin 解析错误: %d", cfg.Bin)
	}
}

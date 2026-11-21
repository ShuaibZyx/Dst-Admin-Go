package dstUtils

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

func TestGetKleiDstPathBetaBranch(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/P",
		"beta=1",
	})
	want := filepath.Join("/P", "worlds-beta")
	if got := GetKleiDstPath(); got != want {
		t.Fatalf("GetKleiDstPath() = %q, want %q", got, want)
	}
}

func TestGetKleiDstPathNonBeta(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/P",
		"beta=0",
	})
	want := filepath.Join("/P", "worlds")
	if got := GetKleiDstPath(); got != want {
		t.Fatalf("GetKleiDstPath() = %q, want %q", got, want)
	}
}

func TestGetKleiDstPathCustomSaveDir(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/P",
		"conf_dir=saves",
		"beta=0",
	})
	want := filepath.Join("/P", "saves")
	if got := GetKleiDstPath(); got != want {
		t.Fatalf("GetKleiDstPath() = %q, want %q", got, want)
	}
}

func TestGetKleiDstPathDefaultKlei(t *testing.T) {
	writeTempDstConfig(t, []string{
		"beta=0",
	})
	if got := GetKleiDstPath(); got != dstConfigUtils.KleiDstPath() {
		t.Fatalf("GetKleiDstPath() = %q, want KleiDstPath() = %q", got, dstConfigUtils.KleiDstPath())
	}
}

// 跨平台健壮性:含空格路径必须加引号,否则 cd 被截断启动失败(unix 分支)。
// buildDstUpdateCmd 为纯构建函数,可跨平台断言非本机分支。
func TestBuildDstUpdateCmd_QuotesPaths(t *testing.T) {
	tmp := t.TempDir()
	spaceRoot := filepath.Join(tmp, "dst server")
	if err := os.MkdirAll(spaceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	cfg := "parent_path=" + spaceRoot + "\nsteamcmd=steamcmd\nforce_install_dir=dst-dedicated-server\ncluster=ROGIA\n"
	if err := os.WriteFile(filepath.Join(tmp, "dst_config"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := buildDstUpdateCmd("linux", "ROGIA")
	// 含空格路径必须整体引用:单引号(Quote)或双引号均可,但引号必须包住整条路径
	if !strings.Contains(cmd, `cd "`) && !strings.Contains(cmd, `cd '`) {
		t.Fatalf("unix 分支的 cd 路径必须带引号, 实际命令: %s", cmd)
	}
	if !strings.Contains(cmd, `force_install_dir "`) && !strings.Contains(cmd, `force_install_dir '`) {
		t.Fatalf("unix 分支的 force_install_dir 路径必须带引号, 实际命令: %s", cmd)
	}
	// 关键回归:引号必须包住含空格的完整路径,而非把空格转义成 \  后塞进双引号
	if !strings.Contains(cmd, `'`+filepath.Join(spaceRoot, "dst-dedicated-server")+`'`) &&
		!strings.Contains(cmd, `"`+filepath.Join(spaceRoot, "dst-dedicated-server")+`"`) {
		t.Fatalf("含空格的安装目录必须作为一个完整引号单元出现, 实际命令: %s", cmd)
	}
}

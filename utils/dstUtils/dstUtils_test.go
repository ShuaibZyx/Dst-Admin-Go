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
	want := filepath.Join("/P", "world-beta")
	if got := GetKleiDstPath(); got != want {
		t.Fatalf("GetKleiDstPath() = %q, want %q", got, want)
	}
}

func TestGetKleiDstPathNonBeta(t *testing.T) {
	writeTempDstConfig(t, []string{
		"parent_path=/P",
		"beta=0",
	})
	want := filepath.Join("/P", "world")
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

package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func entriesOf(t *testing.T, names map[string]bool) []os.DirEntry {
	t.Helper()
	dir := t.TempDir()
	for name, isDir := range names {
		p := filepath.Join(dir, name)
		if isDir {
			if err := os.Mkdir(p, 0755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestIsClusterDir(t *testing.T) {
	if !IsClusterDir(entriesOf(t, map[string]bool{"cluster.ini": false})) {
		t.Error("有 cluster.ini 应为 true")
	}
	if !IsClusterDir(entriesOf(t, map[string]bool{"Master": true})) {
		t.Error("有 Master 子目录应为 true")
	}
	if !IsClusterDir(entriesOf(t, map[string]bool{"Caves": true})) {
		t.Error("有 Caves 子目录应为 true")
	}
	if IsClusterDir(entriesOf(t, map[string]bool{"save": true, "modoverrides.lua": false})) {
		t.Error("只有 save/ 应为 false")
	}
}

func TestIsJunkName(t *testing.T) {
	junk := []string{"bak", "backup", "mod_download", "mod_config_download", "x.pre-migrate-20260101-000000", ".git"}
	for _, n := range junk {
		if !IsJunkName(n) {
			t.Errorf("IsJunkName(%q) = false, want true", n)
		}
	}
	keep := []string{"MyDediServer", "Master", "Caves", "Cluster1"}
	for _, n := range keep {
		if IsJunkName(n) {
			t.Errorf("IsJunkName(%q) = true, want false", n)
		}
	}
}

func TestKnownLegacyRoots(t *testing.T) {
	home := t.TempDir()
	roots := KnownLegacyRoots(home)
	has := func(want string) bool {
		for _, r := range roots {
			if r == want {
				return true
			}
		}
		return false
	}
	if !has(filepath.Join(home, "steamcmd")) {
		t.Error("缺 ~/steamcmd")
	}
	klei := filepath.Join(home, ".klei")
	if runtime.GOOS == "windows" {
		klei = filepath.Join(home, "Documents", "klei")
	}
	if !has(filepath.Join(klei, "DoNotStarveTogether")) {
		t.Error("缺 Klei 默认存档根")
	}
	if !has(filepath.Join(klei, "DoNotStarveTogetherBetaBranch")) {
		t.Error("缺 Klei BetaBranch 存档根")
	}
	if !has("/app/steamcmd") {
		t.Error("缺 Docker /app/steamcmd")
	}
}

func TestIsNestedOrSame(t *testing.T) {
	if !IsNestedOrSame("/data/a", "/data/a") {
		t.Error("同路径应为 true")
	}
	if !IsNestedOrSame("/data/a/b", "/data/a") {
		t.Error("父含子应为 true")
	}
	if !IsNestedOrSame("/data/a", "/data/a/b/c") {
		t.Error("子含父应为 true")
	}
	if IsNestedOrSame("/data/a", "/data/b") {
		t.Error("兄弟目录应为 false")
	}
	if !IsNestedOrSame("/opt/dst parent/mod", "/opt/dst parent") {
		t.Error("含空格路径应为 true")
	}
	if runtime.GOOS == "windows" {
		if !IsNestedOrSame(`C:\Data\A`, `c:\data\a\b`) {
			t.Error("Windows 大小写差异应为 true")
		}
	}
}

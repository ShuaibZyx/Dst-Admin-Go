package migration

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	oldHome := homeDir
	oldRoots := DiscoveryRoots
	homeDir = func() (string, error) { return home, nil }
	// 发现根限定在临时 home，避免测试扫到真实机器
	DiscoveryRoots = func(h string) []string { return []string{h, filepath.Join(h, "Documents")} }
	t.Cleanup(func() {
		homeDir = oldHome
		DiscoveryRoots = oldRoots
	})
	return home
}

func mkdirAll(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func kleiRootOf(home string) string {
	klei := filepath.Join(home, ".klei")
	if runtime.GOOS == "windows" {
		klei = filepath.Join(home, "Documents", "klei")
	}
	return filepath.Join(klei, "DoNotStarveTogether")
}

// fixtureKleiClusters 搭 Klei 根：Alpha(Master/)、Beta(cluster.ini)、bak/ 杂物
func fixtureKleiClusters(t *testing.T, root string) {
	t.Helper()
	mkdirAll(t,
		filepath.Join(root, "Alpha", "Master"),
		filepath.Join(root, "Beta"),
		filepath.Join(root, "bak"),
	)
	writeFileAt(t, filepath.Join(root, "Beta", "cluster.ini"), "x")
}

func findUnit(units []MigrationUnit, kind, sourceSuffix string) *MigrationUnit {
	for i := range units {
		if units[i].Kind == kind && strings.HasSuffix(units[i].Source, sourceSuffix) {
			return &units[i]
		}
	}
	return nil
}

func countKind(units []MigrationUnit, kind string) int {
	n := 0
	for _, u := range units {
		if u.Kind == kind {
			n++
		}
	}
	return n
}

func TestScanFindsLegacyKleiClusters(t *testing.T) {
	home := withTempHome(t)
	root := kleiRootOf(home)
	fixtureKleiClusters(t, root)
	parent := t.TempDir()

	units, err := Scan(dstConfigUtils.DstConfig{Parent_path: parent}, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := countKind(units, KindWorlds); n != 2 {
		t.Fatalf("worlds 单元数 = %d, want 2", n)
	}
	if findUnit(units, KindWorlds, "Alpha") == nil {
		t.Error("缺 Alpha worlds 单元")
	}
	if findUnit(units, KindWorlds, "Beta") == nil {
		t.Error("缺 Beta worlds 单元")
	}
	for _, u := range units {
		if strings.HasSuffix(u.Source, "bak") {
			t.Errorf("bak 不应出单元: %+v", u)
		}
	}
}

func TestScanSharedRootSplitsBackupAndWorlds(t *testing.T) {
	home := withTempHome(t)
	root := kleiRootOf(home)
	fixtureKleiClusters(t, root)
	writeFileAt(t, filepath.Join(root, "snap.zip"), "zz")
	parent := t.TempDir()

	units, err := Scan(dstConfigUtils.DstConfig{
		Parent_path: parent,
		Backup:      root,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := countKind(units, KindWorlds); n != 2 {
		t.Fatalf("worlds 单元数 = %d, want 2", n)
	}
	var backupUnits []MigrationUnit
	for _, u := range units {
		if filepath.Clean(u.Source) == filepath.Clean(root) {
			backupUnits = append(backupUnits, u)
		}
	}
	if len(backupUnits) != 1 {
		t.Fatalf("源=根目录的单元数 = %d, want 1（同源去重）", len(backupUnits))
	}
	b := backupUnits[0]
	if b.Kind != KindBackup {
		t.Errorf("共享根单元 kind = %s, want backup", b.Kind)
	}
	if !b.TopLevelFilesOnly {
		t.Error("共享根 backup 单元应为 TopLevelFilesOnly")
	}
}

func TestScanTargetConflictAndInPlace(t *testing.T) {
	home := withTempHome(t)
	parent := t.TempDir()
	// 当前配置 steamcmd 默认相对值 → 源=目标 → already_in_place
	mkdirAll(t, filepath.Join(parent, "steamcmd"))
	writeFileAt(t, filepath.Join(parent, "steamcmd", "steamcmd.exe"), "x")
	// 模组多位置：配置项占主目标 <parent>/mod；发现的第二位置 → <parent>/mod/steamapps（已被非空占用）→ target_conflict
	klei := filepath.Join(home, "Documents", "klei")
	mkdirAll(t,
		filepath.Join(klei, "steamcmd", "steamapps", "workshop"),
		filepath.Join(klei, "DoNotStarveTogether", "mod_config_download"),
		filepath.Join(parent, "mod", "steamapps"),
	)
	writeFileAt(t, filepath.Join(klei, "steamcmd", "steamcmd.exe"), "x")
	writeFileAt(t, filepath.Join(parent, "mod", "steamapps", "f"), "x")

	units, err := Scan(dstConfigUtils.DstConfig{
		Parent_path:       parent,
		Mod_download_path: filepath.Join(klei, "DoNotStarveTogether", "mod_config_download"),
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	s := findUnit(units, KindSteamcmd, "steamcmd")
	if s == nil || s.Status != StatusAlreadyInPlace {
		t.Errorf("steamcmd 状态 = %+v, want already_in_place", s)
	}
	m := findUnit(units, KindMod, filepath.Join("steamcmd", "steamapps"))
	if m == nil || m.Status != StatusTargetConflict {
		t.Errorf("mod steamapps 状态 = %+v, want target_conflict", m)
	}
}

func TestScanGameRunningBlocks(t *testing.T) {
	home := withTempHome(t)
	root := kleiRootOf(home)
	fixtureKleiClusters(t, root)
	mkdirAll(t, filepath.Join(home, "steamcmd"), filepath.Join(home, "dst"))

	units, err := Scan(dstConfigUtils.DstConfig{Parent_path: t.TempDir()}, true)
	if err != nil {
		t.Fatal(err)
	}
	if w := findUnit(units, KindWorlds, "Alpha"); w == nil || w.Status != StatusGameRunning {
		t.Errorf("worlds 状态 = %+v, want game_running", w)
	}
	if srv := findUnit(units, KindServer, "dst"); srv == nil || srv.Status != StatusGameRunning {
		t.Errorf("server 状态 = %+v, want game_running", srv)
	}
	var sc *MigrationUnit
	for i := range units {
		if units[i].Kind == KindSteamcmd && units[i].SourceLabel == LabelLegacyDefault {
			sc = &units[i]
		}
	}
	if sc == nil || sc.Status != StatusMigratable {
		t.Errorf("legacy steamcmd 状态 = %+v, want migratable（不受游戏运行影响）", sc)
	}
}

func TestScanBetaSuffixServerTarget(t *testing.T) {
	withTempHome(t)
	parent := t.TempDir()
	src := filepath.Join(parent, "myserver") + "-beta"
	mkdirAll(t, src)
	// beta=1 的当前 worlds 根
	clusterDir := filepath.Join(parent, "worlds", "DoNotStarveTogetherBetaBranch", "Alpha", "Master")
	mkdirAll(t, clusterDir)

	units, err := Scan(dstConfigUtils.DstConfig{
		Parent_path:       parent,
		Force_install_dir: "myserver",
		Beta:              1,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	srv := findUnit(units, KindServer, "myserver-beta")
	if srv == nil {
		t.Fatalf("缺 -beta server 单元: %+v", units)
	}
	if !strings.HasSuffix(srv.Target, "-beta") {
		t.Errorf("server target = %q, want -beta 后缀", srv.Target)
	}
	w := findUnit(units, KindWorlds, "Alpha")
	if w == nil || !strings.Contains(w.Target, "DoNotStarveTogetherBetaBranch") {
		t.Errorf("worlds target = %+v, want BetaBranch 目录", w)
	}
}

func TestScanUniqueIDs(t *testing.T) {
	home := withTempHome(t)
	// 两个 worlds 根都含同名 cluster → id 不得撞车
	mkdirAll(t,
		filepath.Join(kleiRootOf(home), "Alpha", "Master"),
		filepath.Join(home, ".klei", "DoNotStarveTogetherBetaBranch", "Alpha", "Master"),
	)
	if runtime.GOOS == "windows" {
		mkdirAll(t,
			filepath.Join(home, "Documents", "klei", "DoNotStarveTogetherBetaBranch", "Alpha", "Master"),
		)
	}
	last := t.TempDir()
	mkdirAll(t, filepath.Join(last, "worlds", "DoNotStarveTogether", "Alpha", "Master"))

	units, err := Scan(dstConfigUtils.DstConfig{
		Parent_path:       t.TempDir(),
		Last_parent_path:  last,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, u := range units {
		if seen[u.ID] {
			t.Errorf("重复 ID: %s", u.ID)
		}
		seen[u.ID] = true
	}
}

func fixtureKleiNested(t *testing.T, home string) string {
	t.Helper()
	klei := filepath.Join(home, "Documents", "klei")
	mkdirAll(t,
		filepath.Join(klei, "steamcmd", "steamapps", "workshop", "content", "322330"),
		filepath.Join(klei, "DoNotStarveTogether", "mod_config_download"),
		filepath.Join(klei, "DoNotStarveTogether", "backup"),
		filepath.Join(klei, "DoNotStarveTogether", "Alpha", "Master"),
	)
	writeFileAt(t, filepath.Join(klei, "steamcmd", "steamcmd.exe"), "x")
	return klei
}

func TestDiscoverFindsKleiNestedLayout(t *testing.T) {
	home := withTempHome(t)
	klei := fixtureKleiNested(t, home)

	cands := DiscoverCandidates([]string{home, filepath.Join(home, "Documents")})
	kinds := map[string]string{}
	for _, c := range cands {
		kinds[c.kind+"|"+filepath.Base(c.path)] = c.path
	}
	if p, ok := kinds["steamcmd|steamcmd"]; !ok || !strings.HasPrefix(p, filepath.Join(klei, "steamcmd")) {
		t.Errorf("未发现 Klei 内嵌 steamcmd: %v", kinds)
	}
	if _, ok := kinds["mod|steamapps"]; !ok {
		t.Errorf("未发现 steamcmd workshop 缓存: %v", kinds)
	}
	if _, ok := kinds["mod|mod_config_download"]; !ok {
		t.Errorf("未发现 mod_config_download: %v", kinds)
	}
	if _, ok := kinds["backup|backup"]; !ok {
		t.Errorf("未发现 Klei 内嵌 backup: %v", kinds)
	}
}

func TestClassifyBySignature(t *testing.T) {
	base := t.TempDir()
	sc := filepath.Join(base, "whatever")
	mkdirAll(t, sc)
	writeFileAt(t, filepath.Join(sc, "steamcmd.exe"), "x")
	if k := ClassifyBySignature(sc); k != KindSteamcmd {
		t.Errorf("steamcmd.exe → %q, want steamcmd", k)
	}
	srv := filepath.Join(base, "whatever2")
	mkdirAll(t, filepath.Join(srv, "bin64"))
	writeFileAt(t, filepath.Join(srv, "bin64", "dontstarve_dedicated_server_nullrenderer_x64.exe"), "x")
	if k := ClassifyBySignature(srv); k != KindServer {
		t.Errorf("nullrenderer → %q, want server", k)
	}
	mod := filepath.Join(base, "whatever3")
	mkdirAll(t, filepath.Join(mod, "steamapps", "workshop"))
	if k := ClassifyBySignature(mod); k != KindMod {
		t.Errorf("steamapps/workshop → %q, want mod", k)
	}
}

func TestScanModUnitsGetDistinctTargets(t *testing.T) {
	home := withTempHome(t)
	fixtureKleiNested(t, home)

	units, err := Scan(dstConfigUtils.DstConfig{
		Parent_path:       t.TempDir(),
		Mod_download_path: filepath.Join(home, "Documents", "klei", "DoNotStarveTogether", "mod_config_download"),
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string]bool{}
	for _, u := range units {
		if u.Kind != KindMod {
			continue
		}
		if targets[u.Target] {
			t.Errorf("mod 目标重复: %s", u.Target)
		}
		targets[u.Target] = true
	}
	if len(targets) < 2 {
		t.Errorf("mod 单元数 = %d, want ≥2（多模组位置）", len(targets))
	}
}

func TestDirSizeTopLevelFilesOnly(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, "sub"))
	writeFileAt(t, filepath.Join(root, "a.txt"), "0123456789")
	writeFileAt(t, filepath.Join(root, "sub", "b.txt"), "01234")

	all, err := DirSize(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if all != 15 {
		t.Errorf("DirSize(recursive) = %d, want 15", all)
	}
	top, err := DirSize(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if top != 10 {
		t.Errorf("DirSize(top-level) = %d, want 10", top)
	}
}

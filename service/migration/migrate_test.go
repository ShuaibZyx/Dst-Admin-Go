package migration

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveDirRename(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "my src dir")
	dst := filepath.Join(base, "my dst dir")
	mkdirAll(t, src)
	writeFileAt(t, filepath.Join(src, "a.txt"), "hello")

	if err := moveDir(src, dst); err != nil {
		t.Fatal(err)
	}
	if dirExists(src) {
		t.Error("源应消失")
	}
	if b, err := os.ReadFile(filepath.Join(dst, "a.txt")); err != nil || string(b) != "hello" {
		t.Errorf("目标内容 = %q, err=%v", b, err)
	}
}

func TestMoveDirConflictRejected(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	dst := filepath.Join(base, "dst")
	mkdirAll(t, src, dst)
	writeFileAt(t, filepath.Join(src, "a.txt"), "src")
	writeFileAt(t, filepath.Join(dst, "b.txt"), "dst")

	if err := moveDir(src, dst); err == nil {
		t.Fatal("目标非空应报错")
	}
	if !dirExists(src) {
		t.Error("源应保持不动")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "b.txt")); string(b) != "dst" {
		t.Error("目标应保持不动")
	}
}

func TestVerifyDirEqualDetectsMismatch(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	mkdirAll(t, a, b)
	writeFileAt(t, filepath.Join(a, "f.txt"), "0123456789")
	writeFileAt(t, filepath.Join(b, "f.txt"), "0123456789")

	if err := verifyDirEqual(a, b); err != nil {
		t.Fatalf("一致树应通过: %v", err)
	}
	writeFileAt(t, filepath.Join(b, "f.txt"), "01234") // 字节数不一致
	if err := verifyDirEqual(a, b); err == nil {
		t.Fatal("字节数不一致应报错")
	}
	writeFileAt(t, filepath.Join(b, "f.txt"), "0123456789")
	writeFileAt(t, filepath.Join(a, "g.txt"), "x") // a 多一个文件
	if err := verifyDirEqual(a, b); err == nil {
		t.Fatal("缺文件应报错")
	}
}

func TestMoveTopLevelFilesOnly(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	dst := filepath.Join(base, "dst")
	mkdirAll(t, filepath.Join(src, "subdir"))
	writeFileAt(t, filepath.Join(src, "a.txt"), "aa")
	writeFileAt(t, filepath.Join(src, "b.txt"), "bb")
	writeFileAt(t, filepath.Join(src, "subdir", "c.txt"), "cc")

	if err := moveTopLevelFiles(src, dst); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Errorf("目标缺 %s: %v", f, err)
		}
	}
	if !dirExists(src) {
		t.Error("源目录本身应保留")
	}
	if !dirExists(filepath.Join(src, "subdir")) {
		t.Error("子目录应留在源")
	}
	if _, err := os.Stat(filepath.Join(src, "a.txt")); err == nil {
		t.Error("顶层文件应已离开源")
	}
}

func TestMigrateSkipsUnselectedAndBlocked(t *testing.T) {
	base := t.TempDir()

	// A: 未选中
	srcA := filepath.Join(base, "a-src")
	dstA := filepath.Join(base, "a-dst")
	mkdirAll(t, srcA)
	writeFileAt(t, filepath.Join(srcA, "f"), "1")

	// B: server + gameRunning → 拒
	srcB := filepath.Join(base, "b-src")
	mkdirAll(t, srcB)

	// C: target_conflict → 拒
	srcC := filepath.Join(base, "c-src")
	dstC := filepath.Join(base, "c-dst")
	mkdirAll(t, srcC, dstC)
	writeFileAt(t, filepath.Join(dstC, "f"), "x")

	// D: migratable 选中 → 成功
	srcD := filepath.Join(base, "d src")
	dstD := filepath.Join(base, "d dst")
	mkdirAll(t, srcD)
	writeFileAt(t, filepath.Join(srcD, "f"), "d")

	units := []MigrationUnit{
		{ID: "A", Kind: KindMod, Source: srcA, Target: dstA, Status: StatusMigratable},
		{ID: "B", Kind: KindServer, Source: srcB, Target: filepath.Join(base, "b-dst"), Status: StatusMigratable},
		{ID: "C", Kind: KindBackup, Source: srcC, Target: dstC, Status: StatusTargetConflict},
		{ID: "D", Kind: KindMod, Source: srcD, Target: dstD, Status: StatusMigratable},
	}
	results := Migrate(units, []string{"B", "C", "D"}, true)

	if !dirExists(srcA) {
		t.Error("未选中的 A 不应被动")
	}
	byID := map[string]MigrateResult{}
	for _, r := range results {
		byID[r.ID] = r
	}
	if _, ok := byID["A"]; ok {
		t.Error("未选中的 A 不应出现在结果里")
	}
	if r, ok := byID["B"]; !ok || r.OK {
		t.Errorf("B 应被游戏运行拒绝: %+v", r)
	}
	if r, ok := byID["C"]; !ok || r.OK {
		t.Errorf("C 应被目标冲突拒绝: %+v", r)
	}
	if r, ok := byID["D"]; !ok || !r.OK {
		t.Errorf("D 应成功: %+v", r)
	}
	if dirExists(srcD) || !dirExists(dstD) {
		t.Error("D 应已移动")
	}
}

func TestRewriteConfigAfterMigration(t *testing.T) {
	cfg := &dstConfigUtils.DstConfig{
		Parent_path:          "/data/dst",
		Steamcmd:             "tools/sc",
		Force_install_dir:    "games/srv",
		Backup:               "/nas/bk",
		Mod_download_path:    "mods/x",
		Persistent_storage_root: "/nas/saves",
		Conf_dir:             "MyConf",
		Beta:                 1,
	}
	succeeded := []MigrationUnit{
		{ID: "backup", Kind: KindBackup},
		{ID: "worlds:Alpha", Kind: KindWorlds},
	}
	RewriteConfigAfterMigration(cfg, succeeded)

	if cfg.Backup != "backup" {
		t.Errorf("Backup = %q, want backup", cfg.Backup)
	}
	if cfg.Persistent_storage_root != "worlds" {
		t.Errorf("Persistent_storage_root = %q, want worlds", cfg.Persistent_storage_root)
	}
	if cfg.Conf_dir != "DoNotStarveTogetherBetaBranch" {
		t.Errorf("Conf_dir = %q, want DoNotStarveTogetherBetaBranch", cfg.Conf_dir)
	}
	// 未成功 kind 原样
	if cfg.Steamcmd != "tools/sc" {
		t.Errorf("Steamcmd = %q, want 原样", cfg.Steamcmd)
	}
	if cfg.Force_install_dir != "games/srv" {
		t.Errorf("Force_install_dir = %q, want 原样", cfg.Force_install_dir)
	}
	if cfg.Mod_download_path != "mods/x" {
		t.Errorf("Mod_download_path = %q, want 原样", cfg.Mod_download_path)
	}
}

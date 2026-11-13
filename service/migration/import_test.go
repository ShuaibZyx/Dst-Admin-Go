package migration

import (
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

	if err := MoveDir(src, dst); err != nil {
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

	if err := MoveDir(src, dst); err == nil {
		t.Fatal("目标非空应报错")
	}
	if !dirExists(src) {
		t.Error("源应保持不动")
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
	writeFileAt(t, filepath.Join(b, "f.txt"), "01234")
	if err := verifyDirEqual(a, b); err == nil {
		t.Fatal("字节数不一致应报错")
	}
}

func fixtureClusterDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	mkdirAll(t, filepath.Join(dir, "Master"))
	writeFileAt(t, filepath.Join(dir, "cluster.ini"), "x")
	return dir
}

func TestImportClusterHappyPath(t *testing.T) {
	parent := t.TempDir()
	src := fixtureClusterDir(t, "MyDediServer")

	target, err := ImportCluster(src, parent, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(parent, "worlds", "DoNotStarveTogether", "MyDediServer")
	if target != want {
		t.Errorf("target = %q, want %q", target, want)
	}
	if dirExists(src) || !dirExists(filepath.Join(target, "Master")) {
		t.Error("导入后源消失、目标就位")
	}
}

func TestImportClusterBetaBranch(t *testing.T) {
	parent := t.TempDir()
	src := fixtureClusterDir(t, "Alpha")
	target, err := ImportCluster(src, parent, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join(parent, "worlds", "DoNotStarveTogetherBetaBranch", "Alpha") {
		t.Errorf("target = %q, want BetaBranch", target)
	}
}

func TestImportClusterRefusals(t *testing.T) {
	parent := t.TempDir()
	// 目标已存在
	src1 := fixtureClusterDir(t, "Dup")
	if _, err := ImportCluster(src1, parent, "", 0); err != nil {
		t.Fatal(err)
	}
	src2 := fixtureClusterDir(t, "Dup")
	if _, err := ImportCluster(src2, parent, "", 0); err == nil {
		t.Error("目标已存在应拒绝")
	}
	// 非 cluster 目录
	notCluster := t.TempDir()
	if _, err := ImportCluster(notCluster, parent, "X", 0); err == nil {
		t.Error("非 cluster 源应拒绝")
	}
	// parent 为空
	src3 := fixtureClusterDir(t, "Y")
	if _, err := ImportCluster(src3, "", "", 0); err == nil {
		t.Error("未配置父级应拒绝")
	}
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

package pathUtils

import (
	"os"
	"path/filepath"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// ① DST_ADMIN_HOME 显式指定最高优先
func TestDataRootEnvOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv("DST_ADMIN_HOME", override)
	chdir(t, t.TempDir()) // CWD 无标记也要听 env 的
	if got := DataRoot(); got != override {
		t.Fatalf("DataRoot() = %q, want DST_ADMIN_HOME %q", got, override)
	}
	if got := Data("password.txt"); got != filepath.Join(override, "password.txt") {
		t.Fatalf("Data() = %q", got)
	}
}

// ② CWD 有数据标记 → CWD(现有部署零感知)
func TestDataRootCwdMarker(t *testing.T) {
	t.Setenv("DST_ADMIN_HOME", "") // 空串=未指定,测试间自动还原
	for _, marker := range dataMarkers {
		cwd := t.TempDir()
		if err := os.WriteFile(filepath.Join(cwd, marker), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		chdir(t, cwd)
		if got := DataRoot(); got != cwd {
			t.Fatalf("标记 %s 在 CWD 时 DataRoot() = %q, want %q", marker, got, cwd)
		}
	}
}

// ③ CWD 无标记 → exe 目录(test 二进制目录可写,规则③命中)
func TestDataRootFallsToExeDir(t *testing.T) {
	t.Setenv("DST_ADMIN_HOME", "")
	chdir(t, t.TempDir()) // 空 CWD
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Dir(exe)
	if got := DataRoot(); got != want {
		t.Fatalf("空 CWD 时 DataRoot() = %q, want exe 目录 %q", got, want)
	}
}

// ResRoot:exe 目录无资源时回退 CWD(测试/go run 场景),CWD 带 dist 即用之
func TestResRootFallsBackToCwd(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	chdir(t, cwd)
	if got := ResRoot(); got != cwd {
		t.Fatalf("CWD 带 dist 时 ResRoot() = %q, want %q", got, cwd)
	}
	if got := Res("static", "template", "server.ini"); got != filepath.Join(cwd, "static", "template", "server.ini") {
		t.Fatalf("Res() = %q", got)
	}
}

// ConfigPath:数据根有 config.yml 用数据根;没有则回退资源根(随包发布)
func TestConfigPathPrefersDataRoot(t *testing.T) {
	t.Setenv("DST_ADMIN_HOME", "")
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "config.yml"), []byte("port: 1"), 0600); err != nil {
		t.Fatal(err)
	}
	chdir(t, data) // 标记命中 → DataRoot=data
	if got := ConfigPath(); got != filepath.Join(data, "config.yml") {
		t.Fatalf("ConfigPath() = %q, want 数据根 %q", got, filepath.Join(data, "config.yml"))
	}
}

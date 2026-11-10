package mod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 启用后 modoverrides 中 workshop-3444078585 enabled=true；禁用后 false 且条目保留
func TestEnsureLuaJitModInFileEnable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	if err := EnsureLuaJitModInFile(p, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, LuaJitModId) {
		t.Fatalf("应包含 %s，实际: %s", LuaJitModId, s)
	}
	if !strings.Contains(s, "enabled = true") && !strings.Contains(s, "enabled=true") {
		t.Fatalf("应 enabled=true，实际: %s", s)
	}
}

func TestEnsureLuaJitModInFileDisableKeepsEntry(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	if err := EnsureLuaJitModInFile(p, true); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLuaJitModInFile(p, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, LuaJitModId) {
		t.Fatalf("禁用后条目应保留，实际: %s", s)
	}
	if strings.Contains(s, "enabled = true") || strings.Contains(s, "enabled=true") {
		t.Fatalf("禁用后应 enabled=false，实际: %s", s)
	}
}

// 已有其它模组时不应破坏原内容
func TestEnsureLuaJitKeepsExistingMods(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	orig := "return {\n  [\"workshop-123\"] = { enabled = true },\n}\n"
	if err := os.WriteFile(p, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLuaJitModInFile(p, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "workshop-123") {
		t.Fatalf("不应破坏原有模组配置，实际: %s", s)
	}
	if !strings.Contains(s, LuaJitModId) {
		t.Fatalf("应追加 luajit 模组，实际: %s", s)
	}
}

func TestEnsureLuaJitDisableWithConfigOptionsFirst(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	orig := "return { [\"workshop-3444078585\"] = { configuration_options = { x = 1 }, enabled = true } }"
	if err := os.WriteFile(p, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLuaJitModInFile(p, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "enabled = true") || strings.Contains(string(b), "enabled=true") {
		t.Fatalf("禁用必须生效（不得残留 enabled=true），实际: %s", b)
	}
}

// M-1/M-6: 块内无 enabled 字段时插入，且不得误改相邻 mod 的 enabled
func TestEnsureLuaJitNoEnabledField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	orig := "return {\n  [\"workshop-3444078585\"] = { configuration_options = { x = 1 } },\n  [\"workshop-123\"] = { enabled = true },\n}\n"
	if err := os.WriteFile(p, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLuaJitModInFile(p, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s2 := string(b)
	if !strings.Contains(s2, "workshop-123\"] = { enabled = true }") {
		t.Fatalf("不得误改相邻 mod 的 enabled，实际: %s", s2)
	}
	if !strings.Contains(s2, "enabled = true") {
		t.Fatalf("目标块应 enabled=true，实际: %s", s2)
	}
}

// M-1/M-6: 超长 configuration_options（>1000 字符）后仍能定位 enabled
func TestEnsureLuaJitLongConfigOptions(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "modoverrides.lua")
	long := strings.Repeat("a = 1, ", 300)
	orig := "return { [\"workshop-3444078585\"] = { configuration_options = { " + long + " }, enabled = true } }"
	if err := os.WriteFile(p, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureLuaJitModInFile(p, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "enabled = true") {
		t.Fatalf("超长配置下禁用必须生效，实际: %.200s", b)
	}
}

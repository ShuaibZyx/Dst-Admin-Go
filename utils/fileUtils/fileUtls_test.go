package fileUtils

import (
	"strings"
	"testing"
)

// A6/A9: 路径穿越必须被拒绝，合法名必须保持在 base 内
func TestSafeJoinRejectsTraversal(t *testing.T) {
	bad := []string{"../x", "..\\x", "a/../../b", "a\\..\\..\\b", "/etc/passwd", "C:\\Windows", "x/y", "x\\y", "..", "."}
	for _, name := range bad {
		if _, err := SafeJoin("base", name); err == nil {
			t.Fatalf("SafeJoin 应拒绝 %q", name)
		}
	}
}

func TestSafeJoinKeepsBase(t *testing.T) {
	p, err := SafeJoin("base", "ok_name")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, "base") {
		t.Fatalf("路径 %q 应在 base 内", p)
	}
	if _, err := SafeJoin("base", ""); err == nil {
		t.Fatal("空名应拒绝")
	}
}

// A9: 白名单字符集
func TestValidName(t *testing.T) {
	for _, ok := range []string{"Master", "Caves", "Shipwrecked", "my-world_2", "123"} {
		if !ValidName(ok) {
			t.Fatalf("ValidName 应接受 %q", ok)
		}
	}
	for _, bad := range []string{"", "a;reboot", "a b", "$(x)", "a/../b", "a\\b", "a.b", "a`b"} {
		if ValidName(bad) {
			t.Fatalf("ValidName 应拒绝 %q", bad)
		}
	}
}

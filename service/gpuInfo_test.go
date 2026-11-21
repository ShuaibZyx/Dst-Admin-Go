package service

import "testing"

// parseFirstPositive 是 wmic/PowerShell 两种输出的共用解析:
// wmic 有 "AdapterRAM" 表头,PowerShell 输出纯数字行,都必须容错取到首个正数。
func TestParseFirstPositive(t *testing.T) {
	if v, ok := parseFirstPositive("AdapterRAM\n8589934592\n"); !ok || v != 8589934592 {
		t.Fatalf("wmic 形态应跳过表头取到数值, got %v %v", v, ok)
	}
	if v, ok := parseFirstPositive("  4294967296  \n"); !ok || v != 4294967296 {
		t.Fatalf("PowerShell 形态应取到数值, got %v %v", v, ok)
	}
	if v, ok := parseFirstPositive("0\n8589934592\n"); !ok || v != 8589934592 {
		t.Fatalf("0 值应跳过, got %v %v", v, ok)
	}
	if _, ok := parseFirstPositive("AdapterRAM\n"); ok {
		t.Fatal("纯表头无数据应判失败")
	}
	if _, ok := parseFirstPositive(""); ok {
		t.Fatal("空输出应判失败")
	}
}

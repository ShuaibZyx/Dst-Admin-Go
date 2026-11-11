package config

import "testing"

// C4: config.yml 缺失/无效时必须返回可用默认配置而非 nil——
// 原 initConfig 在空输入时 _config 保持 nil，随后解引用 AutoUpdateModinfo 崩溃。

func TestParseEmptyReturnsDefaultsWithoutNil(t *testing.T) {
	for _, in := range [][]byte{nil, {}} {
		cfg, err := Parse(in)
		if cfg == nil {
			t.Fatalf("Parse(%q) 返回 nil 配置（会引发 nil 解引用崩溃）", in)
		}
		if cfg.AutoUpdateModinfo.UpdateCheckInterval != 10 {
			t.Fatalf("UpdateCheckInterval 默认应 10, 得到 %d", cfg.AutoUpdateModinfo.UpdateCheckInterval)
		}
		if cfg.AutoUpdateModinfo.CheckInterval != 5 {
			t.Fatalf("CheckInterval 默认应 5, 得到 %d", cfg.AutoUpdateModinfo.CheckInterval)
		}
		_ = err
	}
}

func TestParseInvalidYAMLReturnsDefaultsWithoutPanic(t *testing.T) {
	cfg, err := Parse([]byte("::: not yaml {{{"))
	if cfg == nil {
		t.Fatal("无效 YAML 不应返回 nil")
	}
	if err == nil {
		t.Log("无效 YAML 未返回 err（可接受，只要不 panic 且非 nil）")
	}
	if cfg.AutoUpdateModinfo.UpdateCheckInterval != 10 {
		t.Fatalf("UpdateCheckInterval 默认应 10, 得到 %d", cfg.AutoUpdateModinfo.UpdateCheckInterval)
	}
}

func TestParseRespectsConfiguredValues(t *testing.T) {
	in := []byte("steamAPIKey: CUSTOM_KEY\nautoUpdateModinfo:\n  checkInterval: 3\n  updateCheckInterval: 7\n")
	cfg, err := Parse(in)
	if err != nil {
		t.Fatalf("有效 YAML 不应报错: %v", err)
	}
	if cfg.SteamAPIKey != "CUSTOM_KEY" {
		t.Fatalf("SteamAPIKey 应为 CUSTOM_KEY, 得到 %q", cfg.SteamAPIKey)
	}
	if cfg.AutoUpdateModinfo.CheckInterval != 3 {
		t.Fatalf("CheckInterval 应为 3（非默认）, 得到 %d", cfg.AutoUpdateModinfo.CheckInterval)
	}
	if cfg.AutoUpdateModinfo.UpdateCheckInterval != 7 {
		t.Fatalf("UpdateCheckInterval 应为 7（非默认）, 得到 %d", cfg.AutoUpdateModinfo.UpdateCheckInterval)
	}
}

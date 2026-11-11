package global

import (
	"dst-admin-go/config"
	"testing"
)

// Steam API key 可配置化：未配置回退内置默认，配置了用配置值（此前 4 处硬编码常量，config.SteamAPIKey 闲置）

func TestSteamAPIKeyDefaultsWhenNotConfigured(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })

	Config = nil
	if got := SteamAPIKey(); got != DefaultSteamAPIKey {
		t.Fatalf("nil 配置应回退内置默认 key，得到 %q", got)
	}
	Config = &config.Config{}
	if got := SteamAPIKey(); got != DefaultSteamAPIKey {
		t.Fatalf("未设置 steamAPIKey 时应回退内置默认，得到 %q", got)
	}
}

func TestSteamAPIKeyPrefersConfig(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })

	Config = &config.Config{SteamAPIKey: "CUSTOM_KEY_123"}
	if got := SteamAPIKey(); got != "CUSTOM_KEY_123" {
		t.Fatalf("配置了自定义 key 时应使用配置值，得到 %q", got)
	}
}

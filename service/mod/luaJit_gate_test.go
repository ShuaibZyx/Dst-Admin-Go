package mod

import "testing"

// 开关闸门：启用 LuaJIT 前提是模组已订阅；关闭不设限（退订后仍可关闭，否则开关卡死在开启态）。

func TestLuaJitToggleEnableWithoutSubscriptionBlocked(t *testing.T) {
	err := CheckLuaJitToggle(true, false)
	if err == nil {
		t.Fatal("未订阅时启用应被拦截")
	}
}

func TestLuaJitToggleEnableWithSubscriptionAllowed(t *testing.T) {
	if err := CheckLuaJitToggle(true, true); err != nil {
		t.Fatalf("已订阅时启用应放行, 实际: %v", err)
	}
}

func TestLuaJitToggleDisableAlwaysAllowed(t *testing.T) {
	if err := CheckLuaJitToggle(false, false); err != nil {
		t.Fatalf("未订阅时关闭仍应放行(退订后可关), 实际: %v", err)
	}
	if err := CheckLuaJitToggle(false, true); err != nil {
		t.Fatalf("已订阅时关闭应放行, 实际: %v", err)
	}
}

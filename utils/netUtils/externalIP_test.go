package netUtils

import "testing"

func TestResolveIPManual(t *testing.T) {
	ip, err := ResolveIP("manual", "1.2.3.4")
	if err != nil || ip != "1.2.3.4" {
		t.Fatalf("manual 应原样返回，实际 %q err=%v", ip, err)
	}
}

func TestResolveIPManualRejectsEmpty(t *testing.T) {
	if _, err := ResolveIP("manual", ""); err == nil {
		t.Fatal("manual 空 IP 应报错")
	}
}

func TestResolveIPBindReturnsSomething(t *testing.T) {
	ip, err := ResolveIP("bind", "")
	if err != nil || ip == "" {
		t.Fatalf("bind 应返回本机地址，实际 %q err=%v", ip, err)
	}
}

func TestResolveIPRejectsUnknownMode(t *testing.T) {
	if _, err := ResolveIP("bogus", ""); err == nil {
		t.Fatal("未知模式应报错")
	}
}

// 直连代码格式：外部端口优先（NAT/FRP 映射），密码可选
func TestFormatCConnect(t *testing.T) {
	got := FormatCConnect("1.2.3.4", 40999, "12345")
	want := `c_connect("1.2.3.4", 40999, "12345")`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = FormatCConnect("1.2.3.4", 10999, "")
	want = `c_connect("1.2.3.4", 10999)`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatSteamUrl(t *testing.T) {
	got := FormatSteamURL("1.2.3.4", 40999, "pw")
	want := `steam://rungameid/322330//+connect 1.2.3.4:40999 +password "pw"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = FormatSteamURL("1.2.3.4", 10999, "")
	want = `steam://rungameid/322330//+connect 1.2.3.4:10999`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

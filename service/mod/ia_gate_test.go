package mod

import (
	"os"
	"path/filepath"
	"testing"
)

// 海难/火山(IA 模组)世界闸门:世界选项必须建立在已订阅 workshop-1467214795 的基础上。
// 未订阅:写入类(创建/保存)报错,拉起类(宕机恢复/启动)拒绝;订阅后全放行。

func stubIASubscribed(t *testing.T, v bool) {
	t.Helper()
	old := IsSubscribedFn
	IsSubscribedFn = func(clusterName, workshopId string) bool { return v }
	t.Cleanup(func() { IsSubscribedFn = old })
}

func stubLevelOverride(t *testing.T, content string) {
	t.Helper()
	old := ReadLevelOverride
	ReadLevelOverride = func(clusterName, levelName string) string { return content }
	t.Cleanup(func() { ReadLevelOverride = old })
}

func TestIsIAModWorld(t *testing.T) {
	cases := []struct {
		data string
		want bool
	}{
		{`return { location="shipwrecked" }`, true},
		{`return { location="volcano" }`, true},
		{`return { location="volcanoworld" }`, true},
		{`return { location="forest" }`, false},
		{`return { location="cave" }`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := IsIAModWorld(c.data); got != c.want {
			t.Fatalf("IsIAModWorld(%q) = %v, want %v", c.data, got, c.want)
		}
	}
}

func TestCheckIAWorldWriteBlockedWithoutSubscription(t *testing.T) {
	stubIASubscribed(t, false)
	if err := CheckIAWorldWrite("cluster", `location="shipwrecked"`); err == nil {
		t.Fatal("未订阅时海难世界写入应被拦截")
	}
	if err := CheckIAWorldWrite("cluster", `location="volcano"`); err == nil {
		t.Fatal("未订阅时火山世界写入应被拦截")
	}
}

func TestCheckIAWorldWriteAllowedWithSubscription(t *testing.T) {
	stubIASubscribed(t, true)
	if err := CheckIAWorldWrite("cluster", `location="shipwrecked"`); err != nil {
		t.Fatalf("已订阅时海难世界写入应放行, 实际: %v", err)
	}
}

func TestCheckIAWorldWriteIgnoresNormalWorlds(t *testing.T) {
	stubIASubscribed(t, false)
	if err := CheckIAWorldWrite("cluster", `location="forest"`); err != nil {
		t.Fatalf("森林世界与订阅无关, 应放行, 实际: %v", err)
	}
	if err := CheckIAWorldWrite("cluster", `location="cave"`); err != nil {
		t.Fatalf("洞穴世界与订阅无关, 应放行, 实际: %v", err)
	}
}

func TestCheckIAWorldStartBlockedWithoutSubscription(t *testing.T) {
	stubIASubscribed(t, false)
	stubLevelOverride(t, `location="shipwrecked"`)
	if err := CheckIAWorldStart("cluster", "Shipwrecked"); err == nil {
		t.Fatal("未订阅时海难世界拉起应被拦截(宕机恢复/启动都走这里)")
	}
}

func TestCheckIAWorldStartAllowedWithSubscription(t *testing.T) {
	stubIASubscribed(t, true)
	stubLevelOverride(t, `location="volcano"`)
	if err := CheckIAWorldStart("cluster", "Volcano"); err != nil {
		t.Fatalf("已订阅时火山世界拉起应放行, 实际: %v", err)
	}
}

func TestCheckIAWorldStartIgnoresNormalWorlds(t *testing.T) {
	stubIASubscribed(t, false)
	stubLevelOverride(t, `location="forest"`)
	if err := CheckIAWorldStart("cluster", "Master"); err != nil {
		t.Fatalf("森林世界拉起与订阅无关, 应放行, 实际: %v", err)
	}
}

func writeLevelOverride(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leveldataoverride.lua"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIADirGateBlockedWithoutSubscription(t *testing.T) {
	stubIASubscribed(t, false)
	dir := t.TempDir()
	writeLevelOverride(t, filepath.Join(dir, "Master"), `location="forest"`)
	writeLevelOverride(t, filepath.Join(dir, "Shipwrecked"), `location="shipwrecked"`)
	if err := CheckIADirGate("cluster", dir); err == nil {
		t.Fatal("目录含海难世界且未订阅时应被拦截(备份恢复/预设替换)")
	}
}

func TestCheckIADirGateAllowedWithSubscription(t *testing.T) {
	stubIASubscribed(t, true)
	dir := t.TempDir()
	writeLevelOverride(t, filepath.Join(dir, "Shipwrecked"), `location="shipwrecked"`)
	if err := CheckIADirGate("cluster", dir); err != nil {
		t.Fatalf("已订阅时应放行, 实际: %v", err)
	}
}

func TestCheckIADirGateIgnoresNormalDir(t *testing.T) {
	stubIASubscribed(t, false)
	dir := t.TempDir()
	writeLevelOverride(t, filepath.Join(dir, "Master"), `location="forest"`)
	writeLevelOverride(t, filepath.Join(dir, "Caves"), `location="cave"`)
	if err := CheckIADirGate("cluster", dir); err != nil {
		t.Fatalf("不含 IA 世界的目录应放行, 实际: %v", err)
	}
}

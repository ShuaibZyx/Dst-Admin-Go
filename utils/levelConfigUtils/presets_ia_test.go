package levelConfigUtils

import (
	"strings"
	"testing"
)

// 叠加模式：两个 IA 世界都不抢主分片，Master/Caves 由调用方保留
func TestGenerateIAPresetOverlay(t *testing.T) {
	worlds := GenerateIAPreset("overlay")
	if len(worlds) != 2 {
		t.Fatalf("应生成 2 个世界，实际 %d", len(worlds))
	}
	sw, vol := worlds[0], worlds[1]
	if sw.IsMaster {
		t.Fatal("overlay 模式 Shipwrecked 不应为 is_master")
	}
	if vol.IsMaster {
		t.Fatal("overlay 模式 Volcano 不应为 is_master")
	}
	if !strings.Contains(sw.Leveldataoverride, `location="shipwrecked"`) {
		t.Fatalf("SW 模板缺 location，实际: %.200s", sw.Leveldataoverride)
	}
	if !strings.Contains(sw.Modoverrides, "workshop-1467214795") {
		t.Fatalf("SW 应启用 IA mod，实际: %s", sw.Modoverrides)
	}
	if !strings.Contains(sw.Modoverrides, "no_joining_islands") {
		t.Fatalf("SW 应含 no_joining_islands 开关，实际: %s", sw.Modoverrides)
	}
}

// 独立 IA 存档：Shipwrecked 为主分片
func TestGenerateIAPresetStandalone(t *testing.T) {
	worlds := GenerateIAPreset("standalone")
	if len(worlds) != 2 {
		t.Fatalf("应生成 2 个世界，实际 %d", len(worlds))
	}
	if !worlds[0].IsMaster {
		t.Fatal("standalone 模式 Shipwrecked 应为 is_master")
	}
	if worlds[1].IsMaster {
		t.Fatal("standalone 模式 Volcano 不应为 is_master")
	}
}

func TestGenerateIAPresetRejectsUnknownMode(t *testing.T) {
	if worlds := GenerateIAPreset("bogus"); worlds != nil {
		t.Fatalf("未知模式应返回 nil，实际 %+v", worlds)
	}
}

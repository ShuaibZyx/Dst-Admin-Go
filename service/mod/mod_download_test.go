package mod

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dst-admin-go/utils/dstUtils"
)

// makeZipBytes 构造包含指定文件的最小 zip
func makeZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// CDN 安装收尾三件套:解压归位 + 游戏侧 ACF 登记 + legacy bin 预置。
// 三者缺一,游戏就判"待下载"或解包失败,模组不启用(世界 prefab 缺失→启动崩溃)。
func TestInstallCdnZip(t *testing.T) {
	parent := t.TempDir()
	modsRoot := setupModsEnv(t, parent)
	modid := "1234567890"
	zipBytes := makeZipBytes(t, map[string]string{
		"modinfo.lua": `name = "test"`,
		"modmain.lua": "-- main",
	})
	fileURL := "https://cdn.steamusercontent.com/ugc/999888777/CAF2F15E/"

	// 预建游戏侧 ACF(登记写入的前提;全新环境由游戏首启创建)
	acfPath := dstUtils.GetUgcAcfPath("", "")
	if err := os.MkdirAll(filepath.Dir(acfPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(acfPath, []byte(sampleAcf), 0644); err != nil {
		t.Fatal(err)
	}

	zr, err := installCdnZip(zipBytes, modid, fileURL)
	if err != nil {
		t.Fatal(err)
	}
	if zr == nil {
		t.Fatal("应返回 zipReader 供提取 modinfo")
	}

	modDir := filepath.Join(modsRoot, modid)

	// 1. 解压归位
	if _, err := os.Stat(filepath.Join(modDir, "modinfo.lua")); err != nil {
		t.Fatal("解压归位失败:", err)
	}

	// 2. legacy bin 预置(内容=原始 zip,与磁盘模组同版本)
	b, err := os.ReadFile(filepath.Join(modDir, "999888777_legacy.bin"))
	if err != nil {
		t.Fatal("legacy bin 未预置:", err)
	}
	if !bytes.Equal(b, zipBytes) {
		t.Fatal("legacy bin 内容应与 CDN zip 一致")
	}

	// 3. 游戏侧 ACF 登记
	it, ok := ReadAcfItem(acfPath, "WorkshopItemsInstalled", modid)
	if !ok {
		t.Fatal("ACF 未登记")
	}
	if it.Ugchandle != "999888777" {
		t.Fatalf("ugchandle = %q, want 999888777", it.Ugchandle)
	}
}

// 验收不过必须整体报错:下载完成≠游戏可用的灰色状态绝不放行(改动点 4)。
func TestInstallCdnZipFailsWhenNotGameReady(t *testing.T) {
	setupModsEnv(t, t.TempDir())
	modid := "1234567890"
	zipBytes := makeZipBytes(t, map[string]string{"modinfo.lua": `name = "test"`})
	fileURL := "https://cdn.steamusercontent.com/ugc/999888777/CAF2F15E/"
	acfPath := dstUtils.GetUgcAcfPath("", "")
	if err := os.MkdirAll(filepath.Dir(acfPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(acfPath, []byte(sampleAcf), 0644); err != nil {
		t.Fatal(err)
	}

	old := verifyGameReadyFn
	verifyGameReadyFn = func(modid, modDir string) error { return errors.New("模拟:游戏不可用") }
	t.Cleanup(func() { verifyGameReadyFn = old })

	if _, err := installCdnZip(zipBytes, modid, fileURL); err == nil {
		t.Fatal("验收不过必须报错,不得留下游戏不可用的模组")
	}
}

package mod

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
)

// 一次性运维入口:对真实目录补登记 ACF + 补库布局。
// 用法: DST_LIVE_ACF=1 DST_CONFIG=<repo>/dst_config go test ./service/mod/ -run TestLiveRepairAcf -v
// 平时跳过,不影响测试套件。
func TestLiveRepairAcf(t *testing.T) {
	if os.Getenv("DST_LIVE_ACF") == "" {
		t.Skip("set DST_LIVE_ACF=1 to repair real data")
	}
	cfg := os.Getenv("DST_CONFIG")
	if cfg == "" {
		t.Fatal("need DST_CONFIG pointing at dst_config")
	}
	abs, _ := filepath.Abs(cfg)
	defer dstConfigUtils.SetDstConfigPathForTest(abs)()
	if err := EnsureSteamLibraryLayout(); err != nil {
		t.Fatal(err)
	}
	fixed := EnsureGameAcfEntries()
	t.Logf("补登记 %d 条", fixed)
}

// 一次性运维入口:对真实环境用 CDN 原始 zip 走一遍安装收尾(解压归位 + ACF 登记 + legacy bin 预置),
// 结果保留即修复生效,不回滚。对象选用户环境已装的小模组(覆盖安装同版本无害)。
// 用法: DST_LIVE_CDN=1 DST_CONFIG=<repo>/dst_config go test ./service/mod/ -run TestLiveCdnInstall -v
func TestLiveCdnInstall(t *testing.T) {
	if os.Getenv("DST_LIVE_CDN") == "" {
		t.Skip("set DST_LIVE_CDN=1 to install real data")
	}
	cfg := os.Getenv("DST_CONFIG")
	if cfg == "" {
		t.Fatal("need DST_CONFIG pointing at dst_config")
	}
	abs, _ := filepath.Abs(cfg)
	defer dstConfigUtils.SetDstConfigPathForTest(abs)()

	// Auto Tooth Trap Reset(3195 字节):CDN 可达、覆盖安装同版本无害
	const modid = "588560555"
	const fileURL = "https://cdn.steamusercontent.com/ugc/275094594963518006/66B66F6CEDC94F4498F6CDE2B31ECE774A8E5090/"

	res, err := http.Get(fileURL)
	if err != nil {
		t.Fatal("CDN 下载失败:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("CDN 状态码:", res.StatusCode)
	}
	zipBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("下载 %d 字节", len(zipBytes))

	if _, err := installCdnZip(zipBytes, modid, fileURL); err != nil {
		t.Fatal(err)
	}

	modDir := filepath.Join(dstUtils.GetUgcModPath(), "content", "322330", modid)

	// 1. 解压归位(modinfo.lua 在)
	if _, err := os.Stat(filepath.Join(modDir, "modinfo.lua")); err != nil {
		t.Fatal("解压归位失败:", err)
	}

	// 2. legacy bin 预置,内容=原始 zip
	binName := UgchandleFromURL(fileURL) + "_legacy.bin"
	b, err := os.ReadFile(filepath.Join(modDir, binName))
	if err != nil {
		t.Fatal("legacy bin 未落盘:", err)
	}
	if len(b) != len(zipBytes) {
		t.Fatalf("legacy bin 大小 %d != zip %d", len(b), len(zipBytes))
	}
	t.Log("legacy bin 已落盘:", binName)

	// 3. 游戏侧 ACF 登记
	acfPath := dstUtils.GetUgcAcfPath("", "")
	it, ok := ReadAcfItem(acfPath, "WorkshopItemsInstalled", modid)
	if !ok {
		t.Fatal("ACF 未登记")
	}
	t.Logf("ACF 登记: manifest=%s ugchandle=%s size=%d", it.Manifest, it.Ugchandle, it.Size)
	if it.Ugchandle != UgchandleFromURL(fileURL) && it.Manifest == "-1" {
		t.Fatalf("ugchandle 未取到: %+v", it)
	}

	// 相邻模组不受影响
	entries, _ := os.ReadDir(filepath.Dir(modDir))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !strings.Contains(strings.Join(names, ","), "543945797") {
		t.Fatal("相邻模组目录丢失:", names)
	}
	t.Log("相邻模组完好, 共", len(names), "个目录")
}

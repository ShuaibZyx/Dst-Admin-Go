package mod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dst-admin-go/utils/dstUtils"
)

const sampleAcf = `"AppWorkshop"
{
	"appid"		"322330"
	"SizeOnDisk"		"609697282"
	"NeedsUpdate"		"0"
	"NeedsDownload"		"1"
	"WorkshopItemsInstalled"
	{
		"378160973"
		{
			"size"		"610208"
			"timeupdated"		"1776624358"
			"manifest"		"4506020981624356616"
		}
		"466732225"
		{
			"size"		"993581"
			"timeupdated"		"1488403387"
			"manifest"		"-1"
			"ugchandle"		"156902491433527780"
		}
	}
	"WorkshopItemDetails"
	{
		"1467214795"
		{
			"manifest"		"2014494043738752988"
			"timeupdated"		"1792846979"
			"timetouched"		"1794730163"
			"BytesDownloaded"		"159529488"
			"BytesToDownload"		"159529488"
			"latest_timeupdated"		"1792846979"
			"latest_manifest"		"2014494043738752988"
		}
	}
}
`

func writeTempAcf(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "appworkshop_322330.acf")
	if err := os.WriteFile(p, []byte(sampleAcf), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseAndRoundTrip(t *testing.T) {
	root, err := ParseKV(sampleAcf)
	if err != nil {
		t.Fatal(err)
	}
	if root.key != "AppWorkshop" {
		t.Fatalf("root key = %q", root.key)
	}
	out := WriteKV(root)
	root2, err := ParseKV(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if WriteKV(root2) != out {
		t.Fatal("round trip not stable")
	}
	// 原有字段不丢失
	for _, k := range []string{"NeedsDownload", "WorkshopItemsInstalled", "WorkshopItemDetails"} {
		if kvChild(root2, k) == nil {
			t.Fatalf("lost key %s", k)
		}
	}
}

func TestReadAcfItemInstalled(t *testing.T) {
	p := writeTempAcf(t)
	it, ok := ReadAcfItem(p, "WorkshopItemsInstalled", "466732225")
	if !ok {
		t.Fatal("not found")
	}
	if it.Manifest != "-1" || it.Ugchandle != "156902491433527780" || it.TimeUpdated != 1488403387 {
		t.Fatalf("got %+v", it)
	}
	if _, ok := ReadAcfItem(p, "WorkshopItemsInstalled", "1467214795"); ok {
		t.Fatal("1467214795 should not be installed yet")
	}
}

func TestReadAcfDetailPrefersLatest(t *testing.T) {
	p := writeTempAcf(t)
	it, ok := ReadAcfDetail(p, "1467214795")
	if !ok {
		t.Fatal("not found")
	}
	if it.Manifest != "2014494043738752988" || it.TimeUpdated != 1792846979 {
		t.Fatalf("got %+v", it)
	}
}

func TestUpsertInsertsNewEntry(t *testing.T) {
	p := writeTempAcf(t)
	err := UpsertWorkshopInstalled(p, "1467214795", AcfItem{
		Size: 159529488, TimeUpdated: 1792846979, Manifest: "2014494043738752988",
	})
	if err != nil {
		t.Fatal(err)
	}
	it, ok := ReadAcfItem(p, "WorkshopItemsInstalled", "1467214795")
	if !ok || it.Manifest != "2014494043738752988" {
		t.Fatalf("got %+v ok=%v", it, ok)
	}
	// 其他条目与详情段不动
	if _, ok := ReadAcfItem(p, "WorkshopItemsInstalled", "378160973"); !ok {
		t.Fatal("existing entry lost")
	}
	if _, ok := ReadAcfDetail(p, "1467214795"); !ok {
		t.Fatal("details section lost")
	}
}

func TestUpsertUpdatesExistingEntry(t *testing.T) {
	p := writeTempAcf(t)
	err := UpsertWorkshopInstalled(p, "466732225", AcfItem{
		Size: 1, TimeUpdated: 2, Manifest: "-1", Ugchandle: "999",
	})
	if err != nil {
		t.Fatal(err)
	}
	it, _ := ReadAcfItem(p, "WorkshopItemsInstalled", "466732225")
	if it.Ugchandle != "999" || it.TimeUpdated != 2 {
		t.Fatalf("got %+v", it)
	}
	// 不应产生重复键
	data, _ := os.ReadFile(p)
	if c := strings.Count(string(data), `"466732225"`); c != 1 {
		t.Fatalf("duplicate entries: %d", c)
	}
}

func TestUgchandleFromURL(t *testing.T) {
	got := UgchandleFromURL("https://cdn.steamusercontent.com/ugc/156902491433527780/CAF2F15E/")
	if got != "156902491433527780" {
		t.Fatalf("got %q", got)
	}
	if UgchandleFromURL("https://example.com/x") != "" {
		t.Fatal("expected empty")
	}
}

func TestWriteLegacyBinWritesFile(t *testing.T) {
	dir := t.TempDir()
	zipBytes := []byte("PK\x03\x04fake-mod-zip")
	if err := WriteLegacyBin(dir, "929302284374870670", zipBytes); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "929302284374870670_legacy.bin"))
	if err != nil {
		t.Fatal("legacy bin 未写出:", err)
	}
	if string(b) != string(zipBytes) {
		t.Fatalf("legacy bin 内容 = %q, want %q", b, zipBytes)
	}
}

func TestWriteLegacyBinRejectsEmptyArgs(t *testing.T) {
	dir := t.TempDir()
	if err := WriteLegacyBin(dir, "", []byte("x")); err == nil {
		t.Error("ugchandle 为空应报错")
	}
	if err := WriteLegacyBin(dir, "929302284374870670", nil); err == nil {
		t.Error("zipBytes 为空应报错")
	}
	// 失败时不落垃圾文件
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("失败后不应留文件: %v", entries)
	}
}

// 全新环境游戏侧 ACF 尚不存在(游戏未首启):登记必须创建文件而不是静默失败,
// 否则下载"成功"但游戏无登记、模组不启用——下载即游戏可用的前置条件。
func TestUpsertCreatesAcfWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not-yet", "appworkshop_322330.acf")
	if err := UpsertWorkshopInstalled(p, "1234567890", AcfItem{
		Size: 5, TimeUpdated: 6, Manifest: "777", Ugchandle: "888",
	}); err != nil {
		t.Fatal("ACF 缺失时应创建而不是报错:", err)
	}
	it, ok := ReadAcfItem(p, "WorkshopItemsInstalled", "1234567890")
	if !ok {
		t.Fatal("创建后应可读回登记")
	}
	if it.Manifest != "777" || it.Ugchandle != "888" || it.Size != 5 || it.TimeUpdated != 6 {
		t.Fatalf("读回 %+v 与写入不符", it)
	}
}

// ---- verifyGameReady: 下载收尾验收(条件矩阵) ----

// setupVerifyEnv 配置隔离环境,返回模组目录与游戏侧 ACF 路径
func setupVerifyEnv(t *testing.T) (modDir, acfPath string) {
	t.Helper()
	modsRoot := setupModsEnv(t, t.TempDir())
	modDir = filepath.Join(modsRoot, "1234567890")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	acfPath = dstUtils.GetUgcAcfPath("", "")
	return modDir, acfPath
}

func writeVerifyAcf(t *testing.T, acfPath, entry string) {
	t.Helper()
	body := "\"AppWorkshop\"\n{\n\t\"WorkshopItemsInstalled\"\n\t{\n" + entry + "\t}\n}\n"
	if err := os.WriteFile(acfPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

const verifyRealManifestEntry = "\t\t\"1234567890\"\n\t\t{\n\t\t\t\"manifest\"\t\t\"777\"\n\t\t}\n"
const verifyLegacyEntry = "\t\t\"1234567890\"\n\t\t{\n\t\t\t\"manifest\"\t\t\"-1\"\n\t\t\t\"ugchandle\"\t\t\"999888777\"\n\t\t}\n"

func TestVerifyGameReadyAcceptsRealManifest(t *testing.T) {
	modDir, acfPath := setupVerifyEnv(t)
	writeVerifyAcf(t, acfPath, verifyRealManifestEntry)
	if err := verifyGameReady("1234567890", modDir); err != nil {
		t.Fatal("真 manifest 应通过验收:", err)
	}
}

func TestVerifyGameReadyAcceptsLegacyBin(t *testing.T) {
	modDir, acfPath := setupVerifyEnv(t)
	writeVerifyAcf(t, acfPath, verifyLegacyEntry)
	if err := os.WriteFile(filepath.Join(modDir, "999888777_legacy.bin"), []byte("PK"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyGameReady("1234567890", modDir); err != nil {
		t.Fatal("manifest=-1 且 legacy bin 在位应通过验收:", err)
	}
}

func TestVerifyGameReadyRejectsMissingLegacyBin(t *testing.T) {
	modDir, acfPath := setupVerifyEnv(t)
	writeVerifyAcf(t, acfPath, verifyLegacyEntry)
	if err := verifyGameReady("1234567890", modDir); err == nil {
		t.Error("manifest=-1 且缺 legacy bin 应验收失败(游戏解包必败)")
	}
}

func TestVerifyGameReadyRejectsNoRegistration(t *testing.T) {
	modDir, acfPath := setupVerifyEnv(t)
	writeVerifyAcf(t, acfPath, "")
	if err := verifyGameReady("1234567890", modDir); err == nil {
		t.Error("ACF 无登记应验收失败(游戏判待下载)")
	}
}

// ---- RegisterInstalled 错误语义:全部路径写失败必须报 error,不再静默吞 ----

func TestRegisterInstalledReportsTotalFailure(t *testing.T) {
	setupModsEnv(t, t.TempDir())
	acfPath := dstUtils.GetUgcAcfPath("", "")
	// 把 ACF 路径占成目录,写入必然全部失败
	if err := os.MkdirAll(acfPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := RegisterInstalled("1234567890", AcfItem{Size: 1, Manifest: "2"}); err == nil {
		t.Error("全部 ACF 写失败应返回 error")
	}
}

func TestRegisterInstalledReturnsNilOnSuccess(t *testing.T) {
	setupModsEnv(t, t.TempDir())
	if err := RegisterInstalled("1234567890", AcfItem{Size: 1, Manifest: "2"}); err != nil {
		t.Fatal("正常路径应返回 nil:", err)
	}
}

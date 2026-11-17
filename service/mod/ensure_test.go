package mod

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
)

func TestClusterModIDs(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "worlds", "MySave")
	for _, name := range []string{"Master", "Caves"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mo := `return {\n  ["workshop-1185229307"] = { enabled = true },\n  ["workshop-2929437733"] = { enabled = false },\n}`
	if err := os.WriteFile(filepath.Join(base, "Master", "modoverrides.lua"), []byte(mo), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "Caves", "modoverrides.lua"), []byte(mo), 0644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\nconf_dir=worlds\ncluster=MySave\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	ids := ClusterModIDs("MySave")
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got["1185229307"] || !got["2929437733"] || len(ids) != 2 {
		t.Fatalf("ClusterModIDs = %v, want 两个去重后的 id", ids)
	}
}

// setupModsEnv 配置 parent 为根目录的测试环境，返回统一模组目录 modsRoot
func setupModsEnv(t *testing.T, parent string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\nconf_dir=worlds\ncluster=MySave\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)
	return filepath.Join(parent, "mods", "steamapps", "workshop", "content", "322330")
}

func TestDownloadModToUnified_CompleteModSkipped(t *testing.T) {
	modsRoot := setupModsEnv(t, t.TempDir())
	modid := "1234567890"
	dir := filepath.Join(modsRoot, modid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modinfo.lua"), []byte("name = 'x'"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DownloadModToUnified(modid, nil); err != nil {
		t.Fatalf("完整模组应跳过下载，got err %v", err)
	}
	if !fileExists(filepath.Join(dir, "modinfo.lua")) {
		t.Fatal("完整模组文件不应被清理")
	}
}

func TestDownloadModToUnified_BrokenModNotSkipped(t *testing.T) {
	modsRoot := setupModsEnv(t, t.TempDir())
	modid := "1234567890"
	dir := filepath.Join(modsRoot, modid)
	// 目录存在但缺 modinfo.lua：校验判为 broken
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	err := DownloadModToUnified(modid, nil)
	if err == nil {
		t.Fatal("损坏模组（缺 modinfo.lua）不应返回成功，否则「校验-修复」永远循环报同一批模组")
	}
	if dirExists(dir) {
		t.Fatal("损坏目录应被清理后重下，不应原样保留")
	}
}

func TestWorkshopCacheCleanup(t *testing.T) {
	steamcmdDir := t.TempDir()
	modid := "1991746508"
	dl := filepath.Join(steamcmdDir, "steamapps", "workshop", "downloads")
	itemDir := filepath.Join(dl, "322330", modid)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	patch := filepath.Join(dl, "state_322330_322330_"+modid+".patch")
	if err := os.WriteFile(patch, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cleanWorkshopDownloadResidue(steamcmdDir, modid)
	if dirExists(itemDir) || fileExists(patch) {
		t.Fatal("下载残留应被清理")
	}

	acf := filepath.Join(steamcmdDir, "steamapps", "workshop", "appworkshop_322330.acf")
	if err := os.WriteFile(acf, []byte(`"AppWorkshop" {}`), 0644); err != nil {
		t.Fatal(err)
	}
	invalidateWorkshopAcf(steamcmdDir)
	if fileExists(acf) {
		t.Fatal("acf 假记录应被删除，防止 chunk 复用引用已搬走的文件")
	}
}

// ---- ensureLegacyBinFromDB: manifest=-1 兜底分支的 legacy bin 补齐 ----

func TestEnsureLegacyBinFromDBDownloadsZip(t *testing.T) {
	setupUpsertTest(t)
	zipBody := []byte("PK\x03\x04real-ish-zip-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()
	fileURL := srv.URL + "/ugc/999888777/CAF2F15E/"
	if err := UpsertModInfo(&model.ModInfo{Modid: "1234567890", FileUrl: fileURL}); err != nil {
		t.Fatal(err)
	}
	modDir := t.TempDir()
	got := ensureLegacyBinFromDB("1234567890", modDir)
	if got != "999888777" {
		t.Fatalf("ugchandle = %q, want 999888777", got)
	}
	b, err := os.ReadFile(filepath.Join(modDir, "999888777_legacy.bin"))
	if err != nil {
		t.Fatal("legacy bin 未落盘:", err)
	}
	if !bytes.Equal(b, zipBody) {
		t.Fatal("legacy bin 内容应与下载的 zip 一致")
	}
}

func TestEnsureLegacyBinFromDBEmptyWithoutRecord(t *testing.T) {
	setupUpsertTest(t)
	if got := ensureLegacyBinFromDB("nosuchmod", t.TempDir()); got != "" {
		t.Fatalf("库无记录应返回空, got %q", got)
	}
}

func TestEnsureLegacyBinFromDBEmptyOnDownloadFailure(t *testing.T) {
	setupUpsertTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	if err := UpsertModInfo(&model.ModInfo{Modid: "1234567890", FileUrl: srv.URL + "/ugc/999888777/x/"}); err != nil {
		t.Fatal(err)
	}
	if got := ensureLegacyBinFromDB("1234567890", t.TempDir()); got != "" {
		t.Fatalf("下载失败应返回空, got %q", got)
	}
}

// ---- EnsureSteamLibraryLayout: 缺库布局时游戏内下载必败(Staging library folder not found) ----

func TestEnsureSteamLibraryLayoutCreatesLayout(t *testing.T) {
	parent := t.TempDir()
	setupModsEnv(t, parent) // mod_download_path=mods → steamappsRoot=<parent>/mods/steamapps
	if err := EnsureSteamLibraryLayout(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "mods", "steamapps")
	for _, d := range []string{
		root,
		filepath.Join(root, "staging"),
		filepath.Join(root, "temp"),
		filepath.Join(root, "downloading"),
		filepath.Join(root, "workshop", "content"),
		filepath.Join(root, "workshop", "downloads"),
		filepath.Join(root, "workshop", "temp"),
	} {
		if !dirExists(d) {
			t.Error("缺目录:", d)
		}
	}
	vdf := filepath.Join(root, "libraryfolders.vdf")
	data, err := os.ReadFile(vdf)
	if err != nil {
		t.Fatal("libraryfolders.vdf 未生成:", err)
	}
	if !strings.Contains(string(data), "libraryfolders") {
		t.Error("libraryfolders.vdf 内容异常")
	}
	// 幂等:重复调用不报错、不覆盖已有 vdf
	if err := EnsureSteamLibraryLayout(); err != nil {
		t.Fatal("重复调用应幂等:", err)
	}
}

package mod

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 删除模组 = 全量清扫：统一模组目录、steamcmd 暂存残留、服务端 mods/workshop-<id>。
// 回归背景：删除只退订不清文件；modId 需防路径穿越；删除失败（文件占用）要报路径。

func setupModFilesTest(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	cfg := "parent_path=" + tmp + "\n" +
		"steamcmd=steamcmd\n" +
		"force_install_dir=dst-dedicated-server\n" +
		"mod_download_path=mods\n" +
		"cluster=Cluster1\n"
	if err := os.WriteFile(filepath.Join(tmp, "dst_config"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(tmp, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}, &model.Cluster{}); err != nil {
		t.Fatal(err)
	}
	oldDB := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = oldDB
		_ = sqlDB.Close() // Windows 上不关句柄 TempDir 清理会失败
	})
	return tmp
}

func mustMkdirFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modinfo.lua"), []byte("name='x'"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveModFiles_DeletesAllLocations(t *testing.T) {
	tmp := setupModFilesTest(t)

	mustMkdirFile(t, filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667"))
	mustMkdirFile(t, filepath.Join(tmp, "steamcmd", "steamapps", "workshop", "content", "322330", "3435352667"))
	mustMkdirFile(t, filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-3435352667"))
	// 集群自有 ForceInstallDir 下的副本（IsModSubscribed 的检查路径）
	mustMkdirFile(t, filepath.Join(tmp, "extra-server", "mods", "workshop-3435352667"))
	if err := database.DB.Create(&model.Cluster{ClusterName: "Cluster1", ForceInstallDir: filepath.Join(tmp, "extra-server")}).Error; err != nil {
		t.Fatal(err)
	}
	// 无关模组必须幸存
	mustMkdirFile(t, filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "1185229307"))
	mustMkdirFile(t, filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-2222"))

	// 候选路径必须覆盖四个清扫位置
	paths := strings.Join(modFileCandidatePaths("3435352667"), "\n")
	for _, want := range []string{
		filepath.Join("mods", "steamapps", "workshop", "content", "322330", "3435352667"),
		filepath.Join("steamcmd", "steamapps", "workshop", "content", "322330", "3435352667"),
		filepath.Join("dst-dedicated-server", "mods", "workshop-3435352667"),
		filepath.Join("extra-server", "mods", "workshop-3435352667"),
	} {
		if !strings.Contains(paths, want) {
			t.Fatalf("候选路径缺少 %q, 实际: %s", want, paths)
		}
	}

	if err := RemoveModFiles("3435352667"); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{
		filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667"),
		filepath.Join(tmp, "steamcmd", "steamapps", "workshop", "content", "322330", "3435352667"),
		filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-3435352667"),
		filepath.Join(tmp, "extra-server", "mods", "workshop-3435352667"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("应被删除但仍存在: %s", p)
		}
	}
	for _, p := range []string{
		filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "1185229307"),
		filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-2222"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("无关模组不应被删除: %s", p)
		}
	}
}

func TestRemoveModFiles_MissingPathsOk(t *testing.T) {
	setupModFilesTest(t)
	if err := RemoveModFiles("3435352667"); err != nil {
		t.Fatalf("目录缺失时应返回 nil，实际: %v", err)
	}
}

func TestRemoveModFiles_RejectsNonNumericModId(t *testing.T) {
	tmp := setupModFilesTest(t)
	keep := filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667")
	mustMkdirFile(t, keep)

	for _, bad := range []string{"../evil", "client_mods_disabled", "3435352667/../../x"} {
		if err := RemoveModFiles(bad); err != nil {
			t.Fatalf("非法 modId 应直接返回 nil 不清扫: %s -> %v", bad, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("非法 modId 不应删除任何目录: %v", err)
	}
}

func TestRemoveModSetupLines_RemovesOnlyTargetSubscription(t *testing.T) {
	tmp := setupModFilesTest(t)

	globalSetup := filepath.Join(tmp, "dst-dedicated-server", "mods", "dedicated_server_mods_setup.lua")
	if err := os.MkdirAll(filepath.Dir(globalSetup), 0755); err != nil {
		t.Fatal(err)
	}
	content := "ServerModSetup(\"1185229307\")\n" +
		"ServerModSetup(\"3452907445\")\n" +
		"ServerModSetup(\"workshop-3452907445\")\n" +
		"ServerModSetup(\"3750536829\")\n"
	if err := os.WriteFile(globalSetup, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	clusterSetup := filepath.Join(tmp, "extra-server", "mods", "dedicated_server_mods_setup.lua")
	if err := os.MkdirAll(filepath.Dir(clusterSetup), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clusterSetup, []byte("ServerModSetup(\"3452907445\")\nServerModSetup(\"588560555\")\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&model.Cluster{ClusterName: "Cluster1", ForceInstallDir: filepath.Join(tmp, "extra-server")}).Error; err != nil {
		t.Fatal(err)
	}

	if err := RemoveModSetupLines("3452907445"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(globalSetup)
	if err != nil {
		t.Fatal(err)
	}
	gotStr := string(got)
	if strings.Contains(gotStr, "3452907445") {
		t.Fatalf("目标订阅行应被移除, 实际: %s", gotStr)
	}
	if !strings.Contains(gotStr, `ServerModSetup("1185229307")`) || !strings.Contains(gotStr, `ServerModSetup("3750536829")`) {
		t.Fatalf("其他订阅行应保留, 实际: %s", gotStr)
	}

	got2, err := os.ReadFile(clusterSetup)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got2), "3452907445") || !strings.Contains(string(got2), "588560555") {
		t.Fatalf("集群 setup 文件清理异常: %s", string(got2))
	}
}
func TestRemoveModFiles_PartialFailureKeepsDbAndReportsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("目录占用失败语义仅在 Windows 上确定")
	}
	tmp := setupModFilesTest(t)

	locked := filepath.Join(tmp, "dst-dedicated-server", "mods", "workshop-3435352667")
	mustMkdirFile(t, locked)
	mustMkdirFile(t, filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667"))
	f, err := os.Open(filepath.Join(locked, "modinfo.lua"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	err = RemoveModFiles("3435352667")
	if err == nil {
		t.Fatal("占用导致删除失败时应返回 error")
	}
	if !strings.Contains(err.Error(), "workshop-3435352667") {
		t.Fatalf("error 应包含失败路径，实际: %v", err)
	}
	// 其余位置仍被尝试删除
	if _, statErr := os.Stat(filepath.Join(tmp, "mods", "steamapps", "workshop", "content", "322330", "3435352667")); !os.IsNotExist(statErr) {
		t.Fatalf("未占用的位置仍应被删除: %v", statErr)
	}
}

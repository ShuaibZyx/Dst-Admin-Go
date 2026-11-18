package mod

import (
	"path/filepath"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 模组信息写库必须按 modid 幂等：同 modid 只允许一行活跃记录。
// 回归背景：先查后插 + 下载耗时窗口内的并发请求各自 db.Create，mod_infos 出现重复行。

func setupUpsertTest(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}); err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = old
		_ = sqlDB.Close() // Windows 上不关句柄 TempDir 清理会失败
	})
}

func activeCount(t *testing.T, modid string) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Model(&model.ModInfo{}).
		Where("modid = ? AND deleted_at IS NULL", modid).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUpsertModInfo_SameModidKeepsSingleActiveRow(t *testing.T) {
	setupUpsertTest(t)

	first := model.ModInfo{Modid: "3435352667", Name: "Island Adventures - Core"}
	if err := UpsertModInfo(&first); err != nil {
		t.Fatal(err)
	}

	second := model.ModInfo{Modid: "3435352667", Name: "Island Adventures - Core (updated)"}
	if err := UpsertModInfo(&second); err != nil {
		t.Fatal(err)
	}

	if got := activeCount(t, "3435352667"); got != 1 {
		t.Fatalf("同 modid 两次写入后活跃行应为 1，实际 %d", got)
	}
	var row model.ModInfo
	if err := database.DB.Where("modid = ? AND deleted_at IS NULL", "3435352667").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != "Island Adventures - Core (updated)" {
		t.Fatalf("应更新为新数据，实际 name=%q", row.Name)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert 应复用同一行：first.ID=%d second.ID=%d", first.ID, second.ID)
	}
}

func TestUpsertModInfo_AfterSoftDeleteCreatesFreshRow(t *testing.T) {
	setupUpsertTest(t)

	info := model.ModInfo{Modid: "3452907445", Name: "v1"}
	if err := UpsertModInfo(&info); err != nil {
		t.Fatal(err)
	}
	// UpdateMod 的旧流程：软删旧行后重新订阅
	if err := database.DB.Where("modid = ?", "3452907445").Delete(&model.ModInfo{}).Error; err != nil {
		t.Fatal(err)
	}

	fresh := model.ModInfo{Modid: "3452907445", Name: "v2"}
	if err := UpsertModInfo(&fresh); err != nil {
		t.Fatal(err)
	}

	if got := activeCount(t, "3452907445"); got != 1 {
		t.Fatalf("软删重建后活跃行应为 1，实际 %d", got)
	}
	var row model.ModInfo
	if err := database.DB.Where("modid = ? AND deleted_at IS NULL", "3452907445").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != "v2" {
		t.Fatalf("应为新建的行，实际 name=%q", row.Name)
	}
}

func TestEnsureModInfoUniqueness_BlocksDuplicateActiveInsert(t *testing.T) {
	setupUpsertTest(t)

	if err := EnsureModInfoUniqueness(database.DB); err != nil {
		t.Fatal(err)
	}

	if err := database.DB.Create(&model.ModInfo{Modid: "3750536829", Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	// 模拟竞态里输掉的那路请求：直接插入同 modid 必须被数据库拒绝
	if err := database.DB.Create(&model.ModInfo{Modid: "3750536829", Name: "b"}).Error; err == nil {
		t.Fatal("活跃行唯一索引应拒绝重复插入")
	}

	// 被拒绝的请求回退走 upsert，仍收敛为一行
	if err := UpsertModInfo(&model.ModInfo{Modid: "3750536829", Name: "b"}); err != nil {
		t.Fatal(err)
	}
	if got := activeCount(t, "3750536829"); got != 1 {
		t.Fatalf("回退 upsert 后活跃行应为 1，实际 %d", got)
	}
}

func TestEnsureModInfoUniqueness_CleansExistingDuplicates(t *testing.T) {
	setupUpsertTest(t)

	for _, name := range []string{"a", "b", "c", "d", "e"} {
		if err := database.DB.Create(&model.ModInfo{Modid: "3435352667", Name: name}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.DB.Create(&model.ModInfo{Modid: "1185229307", Name: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	if err := EnsureModInfoUniqueness(database.DB); err != nil {
		t.Fatal(err)
	}

	if got := activeCount(t, "3435352667"); got != 1 {
		t.Fatalf("清理后同 modid 活跃行应为 1，实际 %d", got)
	}
	if got := activeCount(t, "1185229307"); got != 1 {
		t.Fatalf("无重复的行不应受影响，实际 %d", got)
	}
	var row model.ModInfo
	if err := database.DB.Where("modid = ? AND deleted_at IS NULL", "3435352667").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != "e" {
		t.Fatalf("应保留最新一行（e），实际 name=%q", row.Name)
	}

	// 幂等：重复执行不改变结果
	if err := EnsureModInfoUniqueness(database.DB); err != nil {
		t.Fatal(err)
	}
	if got := activeCount(t, "3435352667"); got != 1 {
		t.Fatalf("幂等执行后活跃行应为 1，实际 %d", got)
	}
}

// 回归：EnsureModInfoUniqueness 建的索引 DDL 必须能被 sqlite 迁移器解析。
// 背景：多行 `CREATE UNIQUE INDEX ...\n\t\tON ...` 会让 glebarez parseDDL 抛 "invalid DDL"，
// 进而 AutoMigrate(ModInfo) 失败、整批迁移中止（mod_subscriptions 建不出来）。
func TestEnsureModInfoUniqueness_DDLIsParseableByMigrator(t *testing.T) {
	db := setupUpsertDbOnly(t)
	if err := EnsureModInfoUniqueness(db); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}); err != nil {
		t.Fatalf("EnsureModInfoUniqueness 产生的 DDL 不可迁移: %v", err)
	}
}

// 回归：存量多行索引 SQL 需被修复（drop + 单行重建），否则迁移持续失败
func TestEnsureModInfoUniqueness_RepairsMultilineIndex(t *testing.T) {
	db := setupUpsertDbOnly(t)
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_mod_infos_modid_active
		ON mod_infos(modid) WHERE deleted_at IS NULL`).Error; err != nil {
		t.Fatal(err)
	}
	if err := EnsureModInfoUniqueness(db); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}); err != nil {
		t.Fatalf("存量多行索引未被修复: %v", err)
	}
}

// 与 setupUpsertTest 相同但不替换全局 DB（仅需 gorm 句柄）
func setupUpsertDbOnly(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModInfo{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

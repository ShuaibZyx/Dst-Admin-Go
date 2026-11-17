package service

import (
	"path/filepath"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 角色日志完善:Spawn request 行(备份日志实证格式仍在)是角色唯一来源,
// 现代追踪器按分片日志捕获并写 spawns 表,供玩家日志角色富化。
func TestApplyPlayerEvent_SpawnRequest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Spawn{}, &model.Connect{}); err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = old
		_ = sqlDB.Close()
	})

	online := map[string]bool{}
	guid := map[string]string{}
	applyPlayerEvent(online, guid, "[00:46:09]: Spawn request: wendy from 帅b", "ROGIA")
	applyPlayerEvent(online, guid, "[00:46:10]: Spawn request: wendy from 帅b", "ROGIA") // 同角色去重

	var rows []model.Spawn
	database.DB.Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("应写入 1 条 spawn(同角色去重), 实际 %d: %+v", len(rows), rows)
	}
	if rows[0].Role != "wendy" || rows[0].Name != "帅b" {
		t.Fatalf("spawn 字段错误: %+v", rows[0])
	}
}

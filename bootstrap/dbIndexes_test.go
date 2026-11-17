package bootstrap

import (
	"path/filepath"
	"testing"

	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 玩家日志/连接记录的查询列必须有索引(日志页按 ku_id/created_at 过滤分页),且迁移幂等。
func TestEnsureQueryIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	// 真实启动顺序:AutoMigrate 建表后才建索引
	if err := db.AutoMigrate(&model.PlayerLog{}, &model.Connect{}); err != nil {
		t.Fatal(err)
	}

	if err := EnsureQueryIndexes(db); err != nil {
		t.Fatal(err)
	}
	// 幂等:重复执行不得报错
	if err := EnsureQueryIndexes(db); err != nil {
		t.Fatal(err)
	}

	var names []string
	rows, err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'`).Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	want := map[string]bool{
		"idx_player_logs_ku_id":     false,
		"idx_player_logs_created_at": false,
		"idx_connects_ku_id":        false,
	}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("缺少索引 %s (实际: %v)", k, names)
		}
	}
}

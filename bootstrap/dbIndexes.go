package bootstrap

import (
	"gorm.io/gorm"
)

// EnsureQueryIndexes 高频查询列索引（玩家日志页按 ku_id/created_at 过滤分页、连接记录按 ku_id 查），
// IF NOT EXISTS 幂等，可随启动反复执行。
func EnsureQueryIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_player_logs_ku_id ON player_logs(ku_id)`,
		`CREATE INDEX IF NOT EXISTS idx_player_logs_created_at ON player_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_connects_ku_id ON connects(ku_id)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

package mod

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"strings"

	"gorm.io/gorm"
)

// UpsertModInfo 按 modid 幂等写入模组信息：存在则更新，不存在则插入。
// 配合活跃行唯一索引，并发场景下插入冲突时回退为更新，保证同 modid 只保留一行活跃记录。
// 回归背景：旧的先查后插 + get_mod_info_config 下载耗时窗口，并发请求各自 db.Create 产生重复行。
func UpsertModInfo(info *model.ModInfo) error {
	db := database.DB
	var existing model.ModInfo
	err := db.Where("modid = ?", info.Modid).First(&existing).Error
	if err == nil {
		info.ID = existing.ID
		info.CreatedAt = existing.CreatedAt
		return db.Save(info).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	cerr := db.Create(info).Error
	if cerr == nil {
		return nil
	}
	// 竞态里另一请求已插入（唯一索引拒绝重复）：回退为更新
	if err := db.Where("modid = ?", info.Modid).First(&existing).Error; err != nil {
		return cerr
	}
	info.ID = existing.ID
	info.CreatedAt = existing.CreatedAt
	return db.Save(info).Error
}

// EnsureModInfoUniqueness 清理 mod_infos 中同 modid 的重复活跃行（保留最新一条，其余软删），
// 并创建"活跃行 modid 唯一"的部分唯一索引，从数据库层面杜绝并发重复插入。
// 幂等，可在每次启动时执行。
func EnsureModInfoUniqueness(db *gorm.DB) error {
	if err := db.Exec(`UPDATE mod_infos SET deleted_at = CURRENT_TIMESTAMP
		WHERE deleted_at IS NULL AND id NOT IN (
			SELECT MAX(id) FROM mod_infos WHERE deleted_at IS NULL GROUP BY modid)`).Error; err != nil {
		return err
	}
	// DDL 必须单行：glebarez 的 DDL 解析器不接受名字与 ON 之间的换行，
	// 多行索引 SQL 会让此后每次 AutoMigrate(ModInfo) 抛 "invalid DDL"。
	var idxSQL string
	db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_mod_infos_modid_active'`).Scan(&idxSQL)
	if strings.Contains(idxSQL, "\n") {
		if err := db.Exec(`DROP INDEX IF EXISTS idx_mod_infos_modid_active`).Error; err != nil {
			return err
		}
	}
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_mod_infos_modid_active ON mod_infos(modid) WHERE deleted_at IS NULL`).Error
}

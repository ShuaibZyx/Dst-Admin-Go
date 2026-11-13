package migration

// MigrationUnit 一个可独立迁移的单元（§10）。
// kind: steamcmd|server|mod|backup|worlds
// status: migratable|target_conflict|game_running|already_in_place|not_found
type MigrationUnit struct {
	ID        string `json:"id"`         // 稳定标识，如 "worlds:MyDediServer"、"steamcmd"
	Kind      string `json:"kind"`       // steamcmd|server|mod|backup|worlds
	Source    string `json:"source"`     // 待搬目录绝对路径（worlds 为单个 cluster 目录）
	SourceLabel string `json:"sourceLabel"` // "当前配置"|"旧默认位置"|"上次父级文件夹"
	SizeBytes int64  `json:"sizeBytes"`
	Target    string `json:"target"`     // 迁移目标绝对路径
	Status    string `json:"status"`     // migratable|target_conflict|game_running|already_in_place|not_found
	Note      string `json:"note"`       // 拆分/排除说明，可为空
	TopLevelFilesOnly bool `json:"topLevelFilesOnly"` // true=只搬 Source 顶层文件，目录保留（共享源拆分，§11.5）
}

// 迁移单元类别（Global Constraints 固定值）
const (
	KindSteamcmd = "steamcmd"
	KindServer   = "server"
	KindMod      = "mod"
	KindBackup   = "backup"
	KindWorlds   = "worlds"
)

// 迁移状态（Global Constraints 固定值）
const (
	StatusMigratable     = "migratable"
	StatusTargetConflict = "target_conflict"
	StatusGameRunning    = "game_running"
	StatusAlreadyInPlace = "already_in_place"
	StatusNotFound       = "not_found"
)

// 来源标签（前端展示）
const (
	LabelCurrentConfig = "当前配置"
	LabelLegacyDefault = "旧默认位置"
	LabelLastParent    = "上次父级文件夹"
)

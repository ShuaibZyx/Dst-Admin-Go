package collect

import (
	"path/filepath"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 玩家连接采集:真实日志块(2026-11 版格式)必须解析成功。
// 回归背景:旧实现按空格固定下标取 steamid(str[4]),真实行只有 4 词 → 越界 panic → 记录丢失。

func setupCollectTest(t *testing.T) *Collect {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Connect{}, &model.PlayerLog{}, &model.Spawn{}); err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = old
		_ = sqlDB.Close()
	})
	return &Collect{clusterName: "ROGIA"}
}

// 真实块:摘自 worlds/ROGIA/Master/[Master]server_log.txt(玩家 帅b 的进服)
var realJoinBlock = []string{
	"[00:02:17]: New incoming connection 125.191.234.51|1 <5053643382807312384>",
	"[00:02:17]: Client connected from 125.191.234.51|1 <5053643382807312384>",
	"[00:02:17]: ValidateGameSessionToken GUID<5053643382807312384>",
	"[00:02:18]: Client authenticated: (KU_QSHhEdGR) 帅b",
	"[00:02:18]: [Steam] Authenticated host '76561198984266902'",
	"[00:02:20]: There is no active event to validate against.",
}

func TestParseNewIncomingLog_RealBlock(t *testing.T) {
	c := setupCollectTest(t)
	c.parseNewIncomingLog(realJoinBlock)

	var rows []model.Connect
	if err := database.DB.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("应写入 1 条连接记录, 实际 %d", len(rows))
	}
	got := rows[0]
	if got.Ip != "125.191.234.51" {
		t.Errorf("Ip 应去掉 |1 后缀, 实际 %q", got.Ip)
	}
	if got.KuId != "KU_QSHhEdGR" {
		t.Errorf("KuId 错误: %q", got.KuId)
	}
	if got.Name != "帅b" {
		t.Errorf("Name 错误: %q", got.Name)
	}
	if got.SteamId != "76561198984266902" {
		t.Errorf("SteamId 应取自 Authenticated host, 实际 %q", got.SteamId)
	}
}

func TestParseNewIncomingLog_MultiWordName(t *testing.T) {
	c := setupCollectTest(t)
	block := []string{
		"[00:02:17]: New incoming connection 1.2.3.4|1 <123>",
		"[00:02:17]: Client connected from 1.2.3.4|1 <123>",
		"[00:02:17]: ValidateGameSessionToken GUID<123>",
		"[00:02:18]: Client authenticated: (KU_ABC) Hello World 42",
		"[00:02:18]: [Steam] Authenticated host '999'",
		"[00:02:20]: x",
	}
	c.parseNewIncomingLog(block)

	var rows []model.Connect
	database.DB.Find(&rows)
	if len(rows) != 1 || rows[0].Name != "Hello World 42" {
		t.Fatalf("多段玩家名应完整保留, 实际 %+v", rows)
	}
}

func TestParseServerLogJoin_WritesPlayerLog(t *testing.T) {
	c := setupCollectTest(t)
	// 进服事件从 server_log 的 Client authenticated 行入库玩家日志
	c.parseServerLogEvent("[00:02:18]: Client authenticated: (KU_QSHhEdGR) 帅b")

	var rows []model.PlayerLog
	database.DB.Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("应写入 1 条玩家日志, 实际 %d", len(rows))
	}
	got := rows[0]
	if got.Name != "帅b" || got.KuId != "KU_QSHhEdGR" || got.Action != "[Join]" {
		t.Fatalf("玩家日志字段错误: %+v", got)
	}
	if got.Time != "00:02:18" {
		t.Fatalf("时间错误: %q", got.Time)
	}

	// 离开事件:SendUserDisconnect for '名字' → [Leave];空名(服务端自身)跳过
	c.parseServerLogEvent("[00:05:00]: [Steam] SendUserDisconnect for '帅b'")
	c.parseServerLogEvent("[00:05:01]: [Steam] SendUserDisconnect for ''")
	database.DB.Find(&rows)
	if len(rows) != 2 {
		t.Fatalf("离开事件应追加 1 条(空名跳过), 共 2 条, 实际 %d", len(rows))
	}
	if rows[1].Action != "[Leave]" || rows[1].Name != "帅b" {
		t.Fatalf("离开日志字段错误: %+v", rows[1])
	}
}

// 角色日志完善:旧 Spawn request 格式已消失,角色来源改为在线玩家观测(prefab)。
// RecordSpawn 记录名称→角色映射,同角色去重、换角色追加,getSpawnRole 取最新。
func TestRecordSpawn(t *testing.T) {
	c := setupCollectTest(t)
	RecordSpawn("帅b", "wilson", "ROGIA")
	RecordSpawn("帅b", "wilson", "ROGIA") // 同角色不重复记
	RecordSpawn("帅b", "wanda", "ROGIA")  // 换角色追加
	RecordSpawn("", "wilson", "ROGIA")    // 空名跳过
	RecordSpawn("某人", "", "ROGIA")      // 空角色跳过

	var rows []model.Spawn
	database.DB.Find(&rows)
	if len(rows) != 2 {
		t.Fatalf("应有 2 条 spawn 记录(同角色去重+空值跳过), 实际 %d: %+v", len(rows), rows)
	}
	if got := c.getSpawnRole("帅b"); got.Role != "wanda" {
		t.Fatalf("getSpawnRole 应取最新角色 wanda, 实际 %q", got.Role)
	}
}

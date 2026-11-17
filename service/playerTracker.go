package service

import (
	"dst-admin-go/collect"
	"bufio"
	"os"
	"regexp"
	"strings"
	"sync"

	"dst-admin-go/config/database"
	"dst-admin-go/model"
)

// 玩家在线表——数据源是游戏日志事件(不发 TheNet:GetClientTable 命令:
// 该 API 在 IA(岛屿冒险)世界环境是 nil,命令必报 attempt to call a nil value 并死等超时)。
//
// 事件格式(实测日志):
//
//	[ClientObject] Initialized (self/server object, locally trusted) ... userid=KU_xxx  ← 服务器自身,排除
//	[ClientObject] Initialized (remotely trusted) ... userid=KU_xxx                      ← 玩家上线
//	[ClientObject] Removed (remotely trusted) ... userid=KU_xxx                         ← 玩家离线
//
// 姓名行在日志里没有稳定格式,列表以 KU id 标识(展示层可后续接名字)。

var (
	reClientInit     = regexp.MustCompile(`\[ClientObject\]\s+Initialized.*?guid=(\d+).*?userid=(KU_\w+)`)
	reClientGoneUser = regexp.MustCompile(`\[ClientObject\]\s+(?:Removed|Finalized|Destroy(?:ed)?|Disconnect(?:ed)?)\b.*?userid=(KU_\w+)`)
	reClientGoneGuid = regexp.MustCompile(`\[ClientObject\]\s+(?:Removed|Finalized|Destroy(?:ed)?|Disconnect(?:ed)?)\b.*?guid=(\d+)`)
	reSpawnRequest   = regexp.MustCompile(`^\[[^\]]+\]:\s*Spawn request:\s*(\S+)\s+from\s+(.+?)\s*$`)
)

// applyPlayerEvent 把一行日志事件应用到在线表(增量消费时 guidToKu 需持久)。
func applyPlayerEvent(online map[string]bool, guidToKu map[string]string, line string, clusterName string) {
	// 服务器自身的 client object(local self)不算玩家
	if strings.Contains(line, "self/server object") {
		return
	}
	// 角色来源:Spawn request: <角色> from <名字>(出现在玩家所在分片日志,主世界日志看不到)
	if m := reSpawnRequest.FindStringSubmatch(line); m != nil {
		collect.RecordSpawn(strings.TrimSpace(m[2]), m[1], clusterName)
		return
	}
	if m := reClientInit.FindStringSubmatch(line); m != nil {
		guidToKu[m[1]] = m[2]
		online[m[2]] = true
		return
	}
	if m := reClientGoneUser.FindStringSubmatch(line); m != nil {
		delete(online, m[1])
		return
	}
	if m := reClientGoneGuid.FindStringSubmatch(line); m != nil {
		if kuid, ok := guidToKu[m[1]]; ok {
			delete(online, kuid)
		}
	}
}

// playerLogState 单世界日志的消费进度与在线表(query 时增量推进,无后台任务)
type playerLogState struct {
	mu       sync.Mutex
	offset   int64
	online   map[string]bool
	guidToKu map[string]string // 离线行常只带 guid——上线时建映射,持久跨增量调用
}

var playerLogStates sync.Map // logPath -> *playerLogState

func OnlinePlayersFromLog(logPath, clusterName string) []string {
	v, _ := playerLogStates.LoadOrStore(logPath, &playerLogState{
		online:   map[string]bool{},
		guidToKu: map[string]string{},
	})
	st := v.(*playerLogState)
	st.mu.Lock()
	defer st.mu.Unlock()

	fi, err := os.Stat(logPath)
	if err != nil {
		return nil
	}
	if fi.Size() < st.offset { // 轮转/截断:重建
		st.offset = 0
		st.online = map[string]bool{}
		st.guidToKu = map[string]string{}
	}
	if fi.Size() > st.offset {
		f, err := os.Open(logPath)
		if err == nil {
			_, _ = f.Seek(st.offset, 0)
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			for sc.Scan() {
				applyPlayerEvent(st.online, st.guidToKu, sc.Text(), clusterName)
			}
			pos, _ := f.Seek(0, 1)
			st.offset = pos
			_ = f.Close()
		}
	}

	out := make([]string, 0, len(st.online))
	for kuid := range st.online {
		out = append(out, kuid)
	}
	return out
}

// enrichPlayerNames 从采集的连接记录(Connect)补玩家名字:KU id → name。
// 同 KU 多条取最新;查不到的名字回退 KU id(新玩家/记录缺失)。
func enrichPlayerNames(ids []string, clusterName string) map[string]string {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		out[id] = id
	}
	if len(ids) == 0 {
		return out
	}
	var rows []model.Connect
	if err := database.DB.Where("ku_id IN ? AND cluster_name = ?", ids, clusterName).
		Order("id asc").Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		if r.Name != "" {
			out[r.KuId] = r.Name
		}
	}
	return out
}

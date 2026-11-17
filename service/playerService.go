package service

import (
	"dst-admin-go/collect"
	"dst-admin-go/utils/collectionUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

)

type PlayerService struct {
}

// rePlayerLine 匹配 player: {[id] [idx] [day] [KU] [name] [role]} 输出行。
// name 用贪婪匹配跨过其中的 ] } [ 等字符(玩家名是任意 Steam 名),role(prefab) 取最后一个 [] 组;
// 游戏 print() 每行只输出一个 {…} 块,贪婪匹配不会跨块误吞。
var rePlayerLine = regexp.MustCompile(`\{\[([^\]]*)\] \[([^\]]*)\] \[([^\]]*)\] \[([^\]]*)\] \[(.*)\] \[([^\]]*)\]\}`)

// parsePlayerVO 从日志行解析玩家信息;格式不符返回 nil(C7: 长度校验防索引 panic)。
func parsePlayerVO(line string) *vo.PlayerVO {
	m := rePlayerLine.FindStringSubmatch(line)
	if len(m) < 7 {
		return nil
	}
	return &vo.PlayerVO{Key: m[2], Day: m[3], KuId: m[4], Name: m[5], Role: m[6]}
}

func (p *PlayerService) GetPlayerList(clusterName string, levelName string) []vo.PlayerVO {
	if levelName == "#ALL_LEVEL" {
		levelName = "Master"
	}

	status := gameServe.GetLevelStatus(clusterName, levelName)
	if !status {
		return make([]vo.PlayerVO, 0)
	}

	// 数据源=游戏日志事件重放(OnlinePlayersFromLog)。
	// 旧实现给游戏发 TheNet:GetClientTable 命令等回显——该 API 在 IA(岛屿冒险)
	// 世界环境是 nil,命令必报 attempt to call a nil value 并死等 10 秒超时,
	// 玩家列表永远为空。日志是事实源,查询零延迟。
	logPath := filepath.Join(dstUtils.GetClusterBasePath(clusterName), levelName, "server_log.txt")
	ids := OnlinePlayersFromLog(logPath, clusterName)
	sort.Strings(ids)
	names := enrichPlayerNames(ids, clusterName) // 采集表补名字,查不到回退 KU id
	filteredPlayers := make([]vo.PlayerVO, 0, len(ids))
	for _, kuid := range ids {
		filteredPlayers = append(filteredPlayers, vo.PlayerVO{KuId: kuid, Name: names[kuid], Role: collect.LookupRole(names[kuid], clusterName)})
	}
	return filteredPlayers
}

func (p *PlayerService) GetDstAdminList(clusterName string) (str []string) {
	path := dstUtils.GetAdminlistPath(clusterName)
	if !fileUtils.Exists(path) {
		log.Println("路径不存在", path)
		return
	}
	str, err := fileUtils.ReadLnFile(path)
	log.Println("str:", str)
	if err != nil {
		panic("read dstUtils2 adminlist.txt error: \n" + err.Error())
	}
	return
}

func (p *PlayerService) GetDstBlacklistPlayerList(clusterName string) (str []string) {
	path := dstUtils.GetBlocklistPath(clusterName)
	if !fileUtils.Exists(path) {
		log.Println("路径不存在", path)
		return
	}
	str, err := fileUtils.ReadLnFile(path)
	log.Println("str:", str)
	if err != nil {
		panic("read dstUtils2 blocklist.txt error: \n" + err.Error())
	}
	return
}

func (p *PlayerService) GetDstWhitelistPlayerList(clusterName string) (str []string) {
	path := dstUtils.GetWhitelistPath(clusterName)
	if !fileUtils.Exists(path) {
		log.Println("路径不存在", path)
		return
	}
	str, err := fileUtils.ReadLnFile(path)
	log.Println("str:", str)
	if err != nil {
		panic("read dstUtils2 whitelist.txt error: \n" + err.Error())
	}
	return
}

func (p *PlayerService) SaveDstAdminList(clusterName string, adminlist []string) {

	path := dstUtils.GetAdminlistPath(clusterName)

	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create dstUtils2 blacklist.txt error: \n" + err.Error())
	}
	lnFile, err := fileUtils.ReadLnFile(path)
	set := collectionUtils.ToSet(append(lnFile, adminlist...))

	err = fileUtils.WriterLnFile(path, set)
	if err != nil {
		panic("write dstUtils2 blacklist.txt error: \n" + err.Error())
	}
}

func (p *PlayerService) SaveDstBlacklistPlayerList(clusterName string, blacklist []string) {

	path := dstUtils.GetBlocklistPath(clusterName)

	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create dstUtils2 blacklist.txt error: \n" + err.Error())
	}
	lnFile, err := fileUtils.ReadLnFile(path)
	set := collectionUtils.ToSet(append(lnFile, blacklist...))

	err = fileUtils.WriterLnFile(path, set)
	if err != nil {
		panic("write dstUtils2 blacklist.txt error: \n" + err.Error())
	}
}

func (p *PlayerService) DeleteDstBlacklistPlayerList(clusterName string, blacklist []string) {

	path := dstUtils.GetBlocklistPath(clusterName)
	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create dstUtils2 adminlist.txt error: \n" + err.Error())
	}
	lnFile, err := fileUtils.ReadLnFile(path)
	var result []string
	for i := range lnFile {
		isFind := false
		for j := range blacklist {
			if lnFile[i] == blacklist[j] {
				isFind = true
				break
			}
		}
		if !isFind {
			result = append(result, lnFile[i])
		}
	}

	err = fileUtils.WriterLnFile(path, result)
	if err != nil {
		panic("write dstUtils2 adminlist.txt error: \n" + err.Error())
	}
}

func (p *PlayerService) DeleteDstAdminListPlayerList(clusterName string, adminlist []string) {

	path := dstUtils.GetAdminlistPath(clusterName)
	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create dstUtils2 adminlist.txt error: \n" + err.Error())
	}
	lnFile, err := fileUtils.ReadLnFile(path)
	var result []string
	for i := range lnFile {
		isFind := false
		for j := range adminlist {
			if lnFile[i] == adminlist[j] {
				isFind = true
				break
			}
		}
		if !isFind {
			result = append(result, lnFile[i])
		}
	}

	err = fileUtils.WriterLnFile(path, result)
	if err != nil {
		panic("write dstUtils2 adminlist.txt error: \n" + err.Error())
	}

}

func (p *PlayerService) SaveBlacklist(clusterName string, list []string) {

	path := dstUtils.GetBlacklistPath(clusterName)

	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create blacklist.txt error: \n" + err.Error())
	}

	err = fileUtils.WriterLnFile(path, list)
	if err != nil {
		panic("write dstUtils2 blacklist.txt error: \n" + err.Error())
	}
}

func (p *PlayerService) SaveWhitelist(clusterName string, list []string) {

	path := dstUtils.GetWhitelistPath(clusterName)

	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create blacklist.txt error: \n" + err.Error())
	}

	err = fileUtils.WriterLnFile(path, list)
	if err != nil {
		panic("write dstUtils2 blacklist.txt error: \n" + err.Error())
	}
}

func (p *PlayerService) SaveAdminlist(clusterName string, list []string) {

	path := dstUtils.GetAdminlistPath(clusterName)

	err := fileUtils.CreateFileIfNotExists(path)
	if err != nil {
		panic("create blacklist.txt error: \n" + err.Error())
	}

	err = fileUtils.WriterLnFile(path, list)
	if err != nil {
		panic("write dstUtils2 blacklist.txt error: \n" + err.Error())
	}
}

// matchPlayerLine 判断日志行是否为本次请求的玩家数据行并解析。
// 只按"命令回显 + 请求 id + KU_ 前缀 userid"过滤;不按 "Host" 字样一刀切 —— 玩家名含 Host 会被误杀。
func matchPlayerLine(line, id string) *vo.PlayerVO {
	if strings.Contains(line, "RemoteCommandInput") {
		return nil
	}
	if !strings.Contains(line, id) || !strings.Contains(line, "KU_") {
		return nil
	}
	if playerVO := parsePlayerVO(line); playerVO != nil && strings.HasPrefix(playerVO.KuId, "KU_") {
		return playerVO
	}
	return nil
}

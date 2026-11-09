// Package healthCheck 存档体检（规格 §5.1，对应鸽子鸽体检逻辑还原）
package healthCheck

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	StatusOk    = "ok"
	StatusWarn  = "warn"
	StatusError = "error"

	ItemWorldIntegrity = "world_integrity"
	ItemPortConflict   = "port_conflict"
	ItemUdpListen      = "udp_listen"
	ItemShardLink      = "shard_link"
	ItemWorldSeed      = "world_seed"
	ItemModoverrides   = "modoverrides_syntax"
	ItemModMissing     = "mod_missing"
	ItemToken          = "cluster_token"
	ItemLogWarning     = "log_warning"
	ItemSaveWritable   = "save_writable"
)

type HealthItem struct {
	Level   string `json:"level"`
	Item    string `json:"item"`
	Status  string `json:"status"` // ok | warn | error
	Message string `json:"message"`
	FixHint string `json:"fix_hint"`
}

// Run 对一个集群存档目录（含 cluster.ini 与各 level 子目录）做全项体检。
// 为可测性直接接收 basePath；API 层包装为 Run(clusterName)。
func Run(basePath string) []HealthItem {
	var items []HealthItem
	levels := discoverLevels(basePath)

	items = append(items, checkWorldIntegrity(basePath, levels)...)
	items = append(items, checkPortConflict(levels)...)
	items = append(items, checkShardLink(basePath, levels)...)
	items = append(items, checkWorldSeed(levels)...)
	items = append(items, checkModoverrides(levels)...)
	items = append(items, checkModMissing(basePath, levels)...)
	items = append(items, checkToken(basePath)...)
	items = append(items, checkLogWarning(levels)...)
	items = append(items, checkSaveWritable(levels)...)
	items = append(items, checkUdpListenNote(levels)...)
	return items
}

type levelInfo struct {
	File      string // 目录名
	Path      string
	IsMaster  bool
	ServerIni string
	Port      string
}

func discoverLevels(basePath string) []levelInfo {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil
	}
	var levels []levelInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		iniPath := filepath.Join(basePath, e.Name(), "server.ini")
		if _, err := os.Stat(iniPath); err != nil {
			continue
		}
		content, _ := os.ReadFile(iniPath)
		s := string(content)
		levels = append(levels, levelInfo{
			File:      e.Name(),
			Path:      filepath.Join(basePath, e.Name()),
			IsMaster:  iniBool(s, "is_master"),
			ServerIni: s,
			Port:      iniValue(s, "server_port"),
		})
	}
	return levels
}

func iniValue(s, key string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "="); i > 0 {
			k := strings.TrimSpace(line[:i])
			if strings.EqualFold(k, key) {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

func iniBool(s, key string) bool {
	return strings.EqualFold(iniValue(s, key), "true")
}

func checkWorldIntegrity(basePath string, levels []levelInfo) []HealthItem {
	masters := 0
	for _, l := range levels {
		if l.IsMaster {
			masters++
		}
	}
	it := HealthItem{Level: "*", Item: ItemWorldIntegrity, Status: StatusOk, Message: "世界完整"}
	// 语义：唯一 is_master 分片（不要求目录名为 Master，IA 存档可通过）
	if len(levels) == 0 {
		it.Status = StatusError
		it.Message = "未发现任何世界分片（缺少 server.ini）"
		it.FixHint = "检查存档目录结构，或在面板创建世界"
	} else if masters == 0 {
		it.Status = StatusError
		it.Message = "缺少主分片（is_master=true）"
		it.FixHint = "指定一个世界为 is_master=true"
	} else if masters > 1 {
		it.Status = StatusError
		it.Message = "主分片不唯一"
		it.FixHint = "仅保留一个 is_master=true 的世界"
	}
	return []HealthItem{it}
}

func checkPortConflict(levels []levelInfo) []HealthItem {
	seen := map[string]string{}
	it := HealthItem{Level: "*", Item: ItemPortConflict, Status: StatusOk, Message: "端口无冲突"}
	var conflicts []string
	for _, l := range levels {
		if l.Port == "" {
			continue
		}
		if other, ok := seen[l.Port]; ok {
			conflicts = append(conflicts, other+":"+l.Port+" / "+l.File+":"+l.Port)
		} else {
			seen[l.Port] = l.File
		}
	}
	if len(conflicts) > 0 {
		it.Status = StatusError
		it.Message = "以下世界端口冲突：" + strings.Join(conflicts, "；")
		it.FixHint = "修改各世界 server.ini 的 server_port 使其互不相同"
	}
	return []HealthItem{it}
}

func checkShardLink(basePath string, levels []levelInfo) []HealthItem {
	it := HealthItem{Level: "*", Item: ItemShardLink, Status: StatusOk, Message: "世界互通配置正常"}
	content, err := os.ReadFile(filepath.Join(basePath, "cluster.ini"))
	if err != nil {
		it.Status = StatusWarn
		it.Message = "缺少 cluster.ini"
		it.FixHint = "补全集群配置"
		return []HealthItem{it}
	}
	clusterS := string(content)
	if !iniBool(clusterS, "shard_enabled") {
		it.Message = "shard_enabled=false（单世界）"
		return []HealthItem{it}
	}
	var missing []string
	for _, l := range levels {
		if l.IsMaster {
			continue
		}
		if iniValue(l.ServerIni, "master_ip") == "" {
			missing = append(missing, l.File)
		}
	}
	if len(missing) > 0 {
		it.Status = StatusError
		it.Message = "以下世界未配置 master_ip：" + strings.Join(missing, "、")
		it.FixHint = "在各世界 server.ini [SHARD] 配置 master_ip/master_port 以接入主世界"
	}
	return []HealthItem{it}
}

func checkWorldSeed(levels []levelInfo) []HealthItem {
	var missing []string
	for _, l := range levels {
		if _, err := os.Stat(filepath.Join(l.Path, "leveldataoverride.lua")); err != nil {
			missing = append(missing, l.File)
		}
	}
	it := HealthItem{Level: "*", Item: ItemWorldSeed, Status: StatusOk, Message: "世界生成配置存在"}
	if len(missing) > 0 {
		it.Status = StatusWarn
		it.Message = "以下世界缺少 leveldataoverride.lua：" + strings.Join(missing, "、")
		it.FixHint = "在世界设置中保存一次生成配置"
	}
	return []HealthItem{it}
}

func checkModoverrides(levels []levelInfo) []HealthItem {
	var items []HealthItem
	for _, l := range levels {
		p := filepath.Join(l.Path, "modoverrides.lua")
		if _, err := os.Stat(p); err != nil {
			items = append(items, HealthItem{Level: l.File, Item: ItemModoverrides, Status: StatusWarn, Message: "无 modoverrides.lua（未启用模组）"})
			continue
		}
		b, _ := os.ReadFile(p)
		if !luaBalanced(string(b)) {
			items = append(items, HealthItem{Level: l.File, Item: ItemModoverrides, Status: StatusError, Message: "modoverrides.lua 语法不完整（括号不配平）", FixHint: "检查是否被截断或手改出错，重新保存模组配置"})
		} else {
			items = append(items, HealthItem{Level: l.File, Item: ItemModoverrides, Status: StatusOk, Message: "modoverrides.lua 正常"})
		}
	}
	return items
}

// luaBalanced 粗校验括号配平（忽略字符串内的括号）
func luaBalanced(s string) bool {
	depth := 0
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr != 0 {
			if c == '\\' {
				i++
			} else if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = c
		case '{', '(':
			depth++
		case '}', ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && inStr == 0
}

func checkModMissing(basePath string, levels []levelInfo) []HealthItem {
	var items []HealthItem
	modsDir := filepath.Join(basePath, "..", "mods")
	if _, err := os.Stat(modsDir); err != nil {
		modsDir = "mods"
		if _, err := os.Stat(modsDir); err != nil {
			return []HealthItem{{Level: "*", Item: ItemModMissing, Status: StatusWarn, Message: "未找到 mods 目录，跳过本地模组存在性检查", FixHint: "配置专服 mods 路径"}}
		}
	}
	for _, l := range levels {
		p := filepath.Join(l.Path, "modoverrides.lua")
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var missing []string
		for _, id := range workshopIDs(string(b)) {
			if _, err := os.Stat(filepath.Join(modsDir, id)); err != nil {
				missing = append(missing, id)
			}
		}
		it := HealthItem{Level: l.File, Item: ItemModMissing, Status: StatusOk, Message: "模组文件齐备"}
		if len(missing) > 0 {
			it.Status = StatusWarn
			it.Message = "以下模组本地缺失：" + strings.Join(missing, "、")
			it.FixHint = "在 MOD 管理下载模组，或开服时勾选先下载模组"
		}
		items = append(items, it)
	}
	return items
}

func workshopIDs(s string) []string {
	var ids []string
	for _, seg := range strings.Split(s, "\"") {
		if strings.HasPrefix(seg, "workshop-") {
			id := strings.TrimPrefix(seg, "workshop-")
			if id != "" && !strings.ContainsAny(id, " \t{}") {
				ids = append(ids, "workshop-"+id)
			}
		}
	}
	return ids
}

func checkToken(basePath string) []HealthItem {
	it := HealthItem{Level: "*", Item: ItemToken, Status: StatusOk, Message: "cluster_token.txt 正常"}
	b, err := os.ReadFile(filepath.Join(basePath, "cluster_token.txt"))
	if err != nil {
		it.Status = StatusWarn
		it.Message = "缺少 cluster_token.txt（本地检查）"
		it.FixHint = "在 Klei 官网生成服务器令牌并填入"
	} else if len(strings.TrimSpace(string(b))) == 0 {
		it.Status = StatusWarn
		it.Message = "cluster_token.txt 为空"
		it.FixHint = "填入有效服务器令牌"
	} else if len(strings.TrimSpace(string(b))) < 10 {
		it.Status = StatusWarn
		it.Message = "cluster_token.txt 疑似格式不对（过短）"
		it.FixHint = "核对令牌是否完整"
	}
	return []HealthItem{it}
}

func checkLogWarning(levels []levelInfo) []HealthItem {
	var items []HealthItem
	for _, l := range levels {
		p := filepath.Join(l.Path, "server_log.txt")
		b, err := os.ReadFile(p)
		if err != nil {
			items = append(items, HealthItem{Level: l.File, Item: ItemLogWarning, Status: StatusOk, Message: "无日志（未启动过）"})
			continue
		}
		// 只扫尾部 8KB
		if len(b) > 8192 {
			b = b[len(b)-8192:]
		}
		s := string(b)
		it := HealthItem{Level: l.File, Item: ItemLogWarning, Status: StatusOk, Message: "日志无告警"}
		switch {
		case strings.Contains(s, "[ERROR]") || strings.Contains(s, "token"):
			it.Status = StatusWarn
			it.Message = "日志尾部存在 ERROR 或 token 相关行"
			it.FixHint = "查看 server_log.txt 尾部并按提示处理"
		case strings.Contains(s, "[WARNING]"):
			it.Status = StatusWarn
			it.Message = "日志尾部存在 WARNING"
			it.FixHint = "查看 server_log.txt 尾部告警内容"
		}
		items = append(items, it)
	}
	return items
}

func checkSaveWritable(levels []levelInfo) []HealthItem {
	var items []HealthItem
	for _, l := range levels {
		dir := filepath.Join(l.Path, "save")
		it := HealthItem{Level: l.File, Item: ItemSaveWritable, Status: StatusOk, Message: "存档目录可写"}
		if err := os.MkdirAll(dir, 0755); err != nil {
			it.Status = StatusError
			it.Message = "无法创建/写入 save 目录：" + err.Error()
			it.FixHint = "检查面板进程对存档目录的读写权限"
		} else {
			probe := filepath.Join(dir, ".healthwrite")
			if err := os.WriteFile(probe, []byte("1"), 0644); err != nil {
				it.Status = StatusError
				it.Message = "save 目录不可写：" + err.Error()
				it.FixHint = "检查磁盘空间与目录权限"
			} else {
				_ = os.Remove(probe)
			}
		}
		items = append(items, it)
	}
	return items
}

// checkUdpListenNote: 实际 UDP 监听探测依赖运行时网络状态，这里检查端口配置合法并给出提示
func checkUdpListenNote(levels []levelInfo) []HealthItem {
	it := HealthItem{Level: "*", Item: ItemUdpListen, Status: StatusOk, Message: "各世界端口配置合法"}
	var bad []string
	for _, l := range levels {
		if l.Port == "" {
			bad = append(bad, l.File)
		}
	}
	if len(bad) > 0 {
		it.Status = StatusError
		it.Message = "以下世界未配置 server_port：" + strings.Join(bad, "、")
		it.FixHint = "在各世界 server.ini [NETWORK] 配置 server_port"
	}
	return []HealthItem{it}
}

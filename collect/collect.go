package collect

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

type Collect struct {
	state             chan int
	stop              chan bool
	severLogList      []string
	serverChatLogList []string
	length            int
	clusterName       string
}

func NewCollect(baseLogPath string, clusterName string) *Collect {
	collect := &Collect{
		state: make(chan int, 1),
		severLogList: []string{
			filepath.Join(baseLogPath, "Master", "server_log.txt"),
		},
		serverChatLogList: []string{
			filepath.Join(baseLogPath, "Master", "server_chat_log.txt"),
		},
		stop:        make(chan bool, 2),
		length:      2,
		clusterName: clusterName,
	}
	collect.state <- 1
	return collect
}

func (c *Collect) Stop() {
	close(c.stop)
}

func (c *Collect) ReCollect(baseLogPath, clusterName string) {
	for i := 0; i < c.length; i++ {
		c.stop <- true
	}
	c.severLogList = []string{
		filepath.Join(baseLogPath, "Master", "server_log.txt"),
	}
	c.serverChatLogList = []string{
		filepath.Join(baseLogPath, "Master", "server_chat_log.txt"),
	}
	c.clusterName = clusterName
	c.state <- 1
}

func (c *Collect) StartCollect() {
	go func() {
		for {
			select {
			case <-c.state:
				// 采集
				for _, s := range c.severLogList {
					go c.tailServeLog(s)
				}
				for _, s := range c.serverChatLogList {
					go c.tailServerChatLog(s)
				}
			default:
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}()
}

func (c *Collect) parseNewIncomingLog(lines []string) {

	defer func() {
		if err := recover(); err != nil {
			log.Println("new incoming 日志解析异常:", err)
		}
	}()
	// 按行特征正则提取,不再按空格固定下标(真实块行长不一:steamid 行只有 4 词,
	// 旧下标取 steamid 会越界 panic,整条连接记录丢失)
	connect := model.Connect{ClusterName: c.clusterName}
	for _, line := range lines {
		if m := regexp.MustCompile(`Client connected from (\S+)`).FindStringSubmatch(line); m != nil {
			connect.Ip = strings.Split(m[1], "|")[0]
		}
		if m := regexp.MustCompile(`Client authenticated: \(([A-Za-z0-9_-]+)\) (.+)`).FindStringSubmatch(line); m != nil {
			connect.KuId = m[1]
			connect.Name = strings.TrimSpace(m[2])
		}
		if m := regexp.MustCompile(`Authenticated host '(\d+)'`).FindStringSubmatch(line); m != nil {
			connect.SteamId = m[1]
		}
	}
	database.DB.Create(&connect)
}

// parseServerLogEvent 从 server_log 的客户端认证/断开行生成玩家日志进/退记录。
// 聊天日志的 [Join/Leave Announcement] 依赖公告模组输出,这里做兜底,保证玩家日志页有数据。
func (c *Collect) parseServerLogEvent(text string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("server_log 玩家事件解析异常:", err)
		}
	}()

	var t, action, kuId, name string
	if m := regexp.MustCompile(`^\[([^\]]+)\]:\s*Client authenticated: \(([A-Za-z0-9_-]+)\) (.+)$`).FindStringSubmatch(text); m != nil {
		t, action, kuId, name = m[1], "[Join]", m[2], strings.TrimSpace(m[3])
	} else if m := regexp.MustCompile(`^\[([^\]]+)\]:\s*.*SendUserDisconnect for '(.+)'$`).FindStringSubmatch(text); m != nil && strings.TrimSpace(m[2]) != "" {
		t, action, name = m[1], "[Leave]", strings.TrimSpace(m[2])
	} else {
		return
	}

	playerLog := model.PlayerLog{
		Name:        name,
		Action:      action,
		Time:        t,
		KuId:        kuId,
		ClusterName: c.clusterName,
	}
	if spawn := c.getSpawnRole(name); spawn != nil && playerLog.Role == "" {
		playerLog.Role = spawn.Role
	}
	// 只补空字段:认证行自带的 KuId 是权威值,不被历史连接表覆盖
	if connect := c.getConnectInfo(name); connect != nil {
		if playerLog.Ip == "" {
			playerLog.Ip = connect.Ip
		}
		if playerLog.KuId == "" {
			playerLog.KuId = connect.KuId
		}
		if playerLog.SteamId == "" {
			playerLog.SteamId = connect.SteamId
		}
	}
	if err := database.DB.Create(&playerLog).Error; err != nil {
		log.Println("插入玩家日志失败:", err)
	}
}

func (c *Collect) tailServeLog(fileName string) {

	log.Println("开始采集 path:", fileName)
	config := tail.Config{
		ReOpen:    true,                                 // 重新打开
		Follow:    true,                                 // 是否跟随
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件的哪个地方开始读
		MustExist: false,                                // 文件不存在不报错
		Poll:      true,
	}
	tails, err := tail.TailFile(fileName, config)
	if err != nil {
		log.Println("文件监听失败", err)
	}
	var (
		which        = 0
		isNewConnect = false
		incoming     []string
	)
	for {
		select {
		case line, ok := <-tails.Lines:
			if !ok {
				log.Println("文件读取失败", err)
				time.Sleep(time.Second)
			} else {
				text := line.Text
				if find := strings.Contains(text, "New incoming connection"); find {
					isNewConnect = true
				} else if strings.Contains(text, "Client authenticated:") || strings.Contains(text, "SendUserDisconnect") {
					c.parseServerLogEvent(text)
				}
				// 获取接下来的五条数据
				if isNewConnect {
					incoming = append(incoming, text)
					which++
					if which > 4 {
						isNewConnect = false
						which = 0
						c.parseNewIncomingLog(incoming)
						incoming = []string{}
					}
				}
			}
		case <-c.stop:
			// 结束监听
			err := tails.Stop()
			if err != nil {
				log.Println("tail log 结束失败")
				return
			}
			return
		}
	}
}

func (c *Collect) parseChatLog(text string) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("玩家行为日志解析异常:", err)
		}
	}()
	//[00:00:55]: [Join Announcement] 猜猜我是谁
	if strings.Contains(text, "[Join Announcement]") {
		c.parseJoin(text)
	}
	//[00:02:28]: [Leave Announcement] 猜猜我是谁
	if strings.Contains(text, "[Leave Announcement]") {
		c.parseLeave(text)
	}
	//[00:02:17]: [Death Announcement] 猜猜我是谁 死于： 采摘的红蘑菇。她变成了可怕的鬼魂！
	if strings.Contains(text, "[Death Announcement]") {
		c.parseDeath(text)
	}
	//[00:02:37]: [Resurrect Announcement] 猜猜我是谁 复活自： TMIP 控制台.
	if strings.Contains(text, "[Resurrect Announcement]") {
		c.parseResurrect(text)
	}
	//[00:03:16]: [Say] (KU_Mt-zrX8K) 猜猜我是谁: 你好啊
	if strings.Contains(text, "[Say]") {
		c.parseSay(text)
	}
	//[10:01:42]: [Announcement] 欢迎访客歪比巴卜，游玩
	if strings.Contains(text, "[Announcement]") {
		c.parseAnnouncement(text)
	}
}

func (c *Collect) parseSay(text string) {
	fmt.Println(text)

	// 正则解析日志
	re := regexp.MustCompile(`\[(.*?)\]: (\[.*?\]) \((.*?)\) (.*?): (.*)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) != 6 {
		fmt.Println("无法解析日志:", text, matches)
		return
	}

	// 时间
	t := matches[1]
	// [Say]
	action := matches[2]
	kuId := matches[3]
	// 玩家名字，可包含空格
	name := matches[4]
	actionDesc := matches[5]

	// 获取玩家角色和连接信息
	spawn := c.getSpawnRole(name)
	connect := c.getConnectInfo(name)

	playerLog := model.PlayerLog{
		Name:        name,
		Role:        spawn.Role,
		Action:      action,
		ActionDesc:  actionDesc,
		Time:        t,
		Ip:          connect.Ip,
		KuId:        kuId,
		SteamId:     connect.SteamId,
		ClusterName: c.clusterName,
	}

	// 保存到数据库，并打印错误
	if err := database.DB.Create(&playerLog).Error; err != nil {
		fmt.Println("插入玩家日志失败:", err)
	}
}

func (c *Collect) parseResurrect(text string) {
	c.parseDeath(text)
}

func (c *Collect) parseDeath(text string) {
	fmt.Println(text)

	// 正则表达式 (1)时间, (2)动作, (3)剩余所有内容
	re := regexp.MustCompile(`^\[([^\]]+)\]:\s*(\[[^\]]+\])\s*(.*)$`)
	matches := re.FindStringSubmatch(text)
	if len(matches) != 4 {
		log.Println("无法解析 Announcement Log (正则不匹配):", text)
		return
	}

	t := matches[1]
	action := matches[2]
	// 擦屁股
	action = strings.ReplaceAll(action, " ", "")
	rest := strings.TrimSpace(matches[3]) // 名字 + 描述 整体

	var name string
	var actionDesc string

	// 死亡/复活的分隔符列表 (支持中英文)
	// 关键：在 Death Announce 和 Resurrect Announce 之间寻找共同的分隔模式
	// 中文：死于： / 复活自：
	// 英文：died from / resurrected from / revived by
	announcementWords := []string{
		"死于：", "died from", "was killed by", "starved", "suicide", // 死亡
		"复活自：", "resurrected from", "revived by", // 复活
	}

	splitIndex := -1
	for _, word := range announcementWords {
		// 查找分隔符
		idx := strings.Index(rest, word)
		if idx > splitIndex { // 找到最靠前的已知分隔符
			splitIndex = idx
			// 找到后立即退出循环，因为第一个匹配就是名字和描述的边界
			break
		}
	}

	if splitIndex != -1 {
		// 找到了分隔符：分割 rest
		name = strings.TrimSpace(rest[:splitIndex])
		actionDesc = strings.TrimSpace(rest[splitIndex:])
	} else {
		// 未找到已知分隔符，假设整个 rest 都是名字，描述为空 (适用于名字很长，或系统消息)
		name = rest
		actionDesc = ""
		fmt.Println("Announcement Log 未找到分隔符，将全部分配给 Name:", name)
	}

	spawn := c.getSpawnRole(name)
	connect := c.getConnectInfo(name)
	fmt.Println(connect)

	playerLog := model.PlayerLog{
		Name:        name,
		Role:        spawn.Role,
		Action:      action,
		ActionDesc:  actionDesc,
		Time:        t,
		Ip:          connect.Ip,
		KuId:        connect.KuId,
		SteamId:     connect.SteamId,
		ClusterName: c.clusterName,
	}

	if err := database.DB.Create(&playerLog).Error; err != nil {
		fmt.Println("插入玩家日志失败:", err)
	}
}

func (c *Collect) parseLeave(text string) {
	c.parseJoin(text)
}

func (c *Collect) parseJoin(text string) {
	fmt.Println(text)

	// 正则表达式：捕获 (1)时间, (2)动作, (3)玩家名
	re := regexp.MustCompile(`^\[([^\]]+)\]:\s*(\[[^\]]+\])\s*(.+)$`)
	matches := re.FindStringSubmatch(text)

	// 预期匹配 4 组：[完整匹配, 时间, 动作, 玩家名]
	if len(matches) != 4 {
		log.Println("无法解析 Join Log (正则不匹配):", text)
		return
	}

	// 捕获结果
	t := matches[1]      // 时间: 00:01:43
	action := matches[2] // 动作: [Join Announcement]
	// 擦屁股
	action = strings.ReplaceAll(action, " ", "")
	// 玩家名字是捕获组 3，使用 strings.TrimSpace 确保名字前后没有多余空格
	name := strings.TrimSpace(matches[3])

	spawn := c.getSpawnRole(name)
	connect := c.getConnectInfo(name)

	playerLog := model.PlayerLog{
		Name:        name,
		Role:        spawn.Role,
		Action:      action,
		Time:        t,
		Ip:          connect.Ip,
		KuId:        connect.KuId,
		SteamId:     connect.SteamId,
		ClusterName: c.clusterName,
	}

	// 保存到数据库，并打印错误
	if err := database.DB.Create(&playerLog).Error; err != nil {
		fmt.Println("插入玩家日志失败:", err)
	}
}

func (c *Collect) tailServerChatLog(fileName string) {
	log.Println("开始采集 path:", fileName)
	config := tail.Config{
		ReOpen:    true,                                 // 重新打开
		Follow:    true,                                 // 是否跟随
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件的哪个地方开始读
		MustExist: false,                                // 文件不存在不报错
		Poll:      true,
	}
	tails, err := tail.TailFile(fileName, config)
	if err != nil {
		log.Println("文件监听失败", err)
	}
	for {
		select {
		case line, ok := <-tails.Lines:
			if !ok {
				log.Println("文件读取失败", err)
				time.Sleep(time.Second)
			} else {
				text := line.Text
				c.parseChatLog(text)
			}
		case <-c.stop:
			// 结束监听
			err := tails.Stop()
			if err != nil {
				log.Println("tail log 结束失败")
				return
			}
			return
		}
	}
}

// RecordSpawn 记录玩家当前角色（名称→角色）。
// 旧 Spawn request 日志格式已消失，角色来源改为在线玩家观测（prefab）；
// 同角色去重、换角色追加，getSpawnRole 取最新记录做玩家日志角色富化。
func RecordSpawn(name, role, clusterName string) {
	if name == "" || role == "" {
		return
	}
	last := new(model.Spawn)
	database.DB.Where("name = ? AND cluster_name = ?", name, clusterName).Last(last)
	if last.Role == role {
		return
	}
	database.DB.Create(&model.Spawn{
		Name:        name,
		Role:        role,
		Time:        time.Now().Format("15:04:05"),
		ClusterName: clusterName,
	})
}

// LookupRole 查询玩家最新角色(名称→角色),供玩家列表等跨包使用;无记录返回空串。
func LookupRole(name, clusterName string) string {
	if name == "" {
		return ""
	}
	spawn := new(model.Spawn)
	database.DB.Where("name = ? and cluster_name = ?", name, clusterName).Last(spawn)
	return spawn.Role
}

func (c *Collect) getSpawnRole(name string) *model.Spawn {
	spawn := new(model.Spawn)
	database.DB.Where("name LIKE ? and cluster_name = ?", "%"+name+"%", c.clusterName).Last(spawn)
	return spawn
}

func (c *Collect) getConnectInfo(name string) *model.Connect {
	connect := new(model.Connect)
	database.DB.Where("name LIKE ? and cluster_name = ?", "%"+name+"%", c.clusterName).Last(connect)
	return connect
}

func (c *Collect) parseAnnouncement(text string) {
	fmt.Println(text)

	// 正则解析日志
	re := regexp.MustCompile(`\[(.*?)\]: (\[Announcement\]) (.*)`)
	matches := re.FindStringSubmatch(text)
	// 无法解析宣告日志: 00:55:21 Announcement test
	if len(matches) != 4 {
		fmt.Println("无法解析宣告日志:", text, matches)
		return
	}

	// 时间
	t := matches[1]
	// [Announcement]
	action := matches[2]
	// 宣告的内容
	actionDesc := matches[3]

	playerLog := model.PlayerLog{
		Name:        "-",
		Role:        "-",
		Action:      action,
		ActionDesc:  actionDesc,
		Time:        t,
		Ip:          "-",
		KuId:        "-",
		SteamId:     "-",
		ClusterName: c.clusterName,
	}

	// 保存到数据库，并打印错误
	if err := database.DB.Create(&playerLog).Error; err != nil {
		fmt.Println("插入玩家日志失败:", err)
	}
}

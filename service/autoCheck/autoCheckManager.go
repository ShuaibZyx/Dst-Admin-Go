package autoCheck

import (
	"dst-admin-go/config/database"
	"dst-admin-go/constant/consts"
	"dst-admin-go/model"
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/levelConfigUtils"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	steamAPIKey = "73DF9F781D195DFD3D19DED1CB72EEE6"
	appID       = 322330
	language    = 6
)

var Manager *AutoCheckManager

type AutoCheckManager struct {
	AutoChecks  []model.AutoCheck
	statusMap   map[string]chan int
	launchMutex sync.Mutex
	mutex       sync.Mutex
}

var gameConsoleService service.GameConsoleService
var gameService service.GameService
var logRecordService service.LogRecordService

func (m *AutoCheckManager) ReStart(clusterName string) {
	m.mutex.Lock()
	for s := range m.statusMap {
		close(m.statusMap[s])  // 关闭通道
		delete(m.statusMap, s) // 从 statusMap 中删除键值对
	}
	m.mutex.Unlock()

	// 清空所有表
	db := database.DB
	db.Where("1 = 1").Delete(&model.AutoCheck{})

	m.Start()
}

func (m *AutoCheckManager) Start() {
	log.Println("开始自动维护-清理旧数据")
	// 修复之前自动宕机藏数据问题，之前清空之前的数据，重新设置
	kvdb := database.DB
	kv := model.KV{}
	kvdb.Where("key = ?", "clear_old_auto_check").Find(&kv)
	if kv.Value != "Y" {

		db := database.DB
		db.Unscoped().Where("1=1").Delete(&model.AutoCheck{})

		kv.Key = "clear_old_auto_check"
		kv.Value = "Y"
		kvdb.Save(&kv)
	}

	for {
		dstConfig := dstConfigUtils.GetDstConfig()
		kleiPath := filepath.Join(dstUtils.GetKleiDstPath())
		baseLevelPath := filepath.Join(kleiPath, dstConfig.Cluster)
		if !fileUtils.Exists(baseLevelPath) {
			time.Sleep(1 * time.Minute)
		} else {
			break
		}
	}
	log.Println("开始自动维护-检查更新和运行状态")
	m.statusMap = make(map[string]chan int)
	// 游戏更新
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		m.StartGameUpdate()
	}()
	// 模组更新
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		m.StartGameModDown()
	}()
	// 宕机恢复
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		m.StartGameLevelDown()
	}()
	log.Println("自动维护结束")
}

func (m *AutoCheckManager) StartGameUpdate() {
	dstConfig := dstConfigUtils.GetDstConfig()
	// config, _ := levelConfigUtils.GetLevelConfig(dstConfig.Cluster)
	db := database.DB
	autoCheck := model.AutoCheck{}
	db.Where("cluster_name = ? and check_type = ?", dstConfig.Cluster, consts.UPDATE_GAME).Find(&autoCheck)

	if autoCheck.ID == 0 {
		autoCheck.ClusterName = dstConfig.Cluster
		autoCheck.Uuid = consts.UPDATE_GAME + "_" + dstConfig.Cluster
		autoCheck.Enable = 0
		autoCheck.Interval = 30
		autoCheck.CheckType = consts.UPDATE_GAME
	}
	log.Println("StartGameUpdate", autoCheck)
	taskId := autoCheck.Uuid + "|" + autoCheck.CheckType // B9: 复合键
	m.run(&autoCheck, m.stopChan(taskId))
}

func (m *AutoCheckManager) StartGameLevelDown() {
	dstConfig := dstConfigUtils.GetDstConfig()
	config, _ := levelConfigUtils.GetLevelConfig(dstConfig.Cluster)
	db := database.DB
	for i := range config.LevelList {
		autoCheck := model.AutoCheck{}
		uuid := config.LevelList[i].File
		db.Where("cluster_name = ? and check_type = ? and uuid = ?", dstConfig.Cluster, consts.LEVEL_DOWN, uuid).Find(&autoCheck)
		if autoCheck.ID == 0 {
			autoCheck.ClusterName = dstConfig.Cluster
			autoCheck.Uuid = uuid
			autoCheck.LevelName = config.LevelList[i].File
			autoCheck.Enable = 0
			autoCheck.Interval = 30
			autoCheck.CheckType = consts.LEVEL_DOWN
		}
		log.Println("StartGameLevelDown", autoCheck)
		taskId := autoCheck.Uuid + "|" + autoCheck.CheckType // B9: 复合键
		//因为sleep机制 永远不会返回 所以改协程执行
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			m.run(&autoCheck, m.stopChan(taskId))
			log.Println("StartGameLevelDownOver", autoCheck)
		}()
	}
}

func (m *AutoCheckManager) StartGameModDown() {
	dstConfig := dstConfigUtils.GetDstConfig()
	config, _ := levelConfigUtils.GetLevelConfig(dstConfig.Cluster)
	db := database.DB

	for i := range config.LevelList {
		autoCheck := model.AutoCheck{}
		uuid := config.LevelList[i].File
		db.Where("cluster_name = ? and check_type = ? and uuid = ?", dstConfig.Cluster, consts.LEVEL_MOD, uuid).Find(&autoCheck)
		if autoCheck.ID == 0 {
			autoCheck.ClusterName = dstConfig.Cluster
			autoCheck.Uuid = uuid
			autoCheck.Enable = 0
			autoCheck.Interval = 30
			autoCheck.CheckType = consts.LEVEL_MOD
		}
		log.Println("StartGameModDown", autoCheck)
		taskId := autoCheck.Uuid + "|" + autoCheck.CheckType // B9: 复合键
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			m.run(&autoCheck, m.stopChan(taskId))
			log.Println("StartGameModDownOver", autoCheck)
		}()
	}
	log.Println("StartGameModDownOk")
}

func (m *AutoCheckManager) run(task *model.AutoCheck, stop chan int) {
	for {
		select {
		case <-stop:
			return
		default:
			m.check(task)
		}
	}
}
func (m *AutoCheckManager) GetAutoCheck(clusterName, checkType, uuid string) *model.AutoCheck {
	db := database.DB
	autoCheck := model.AutoCheck{}
	db.Where("cluster_name = ? and check_type = ? and uuid = ?", clusterName, checkType, uuid).Find(&autoCheck)
	if autoCheck.Interval == 0 {
		autoCheck.Interval = 10
	}
	return &autoCheck
}

func (m *AutoCheckManager) check(task *model.AutoCheck) {
	// log.Println("开始检查 start", task)
	if task.Uuid != "" {
		task = m.GetAutoCheck(task.ClusterName, task.CheckType, task.Uuid)
	}

	// log.Println("开始检查", task.ClusterName, task.LevelName, task.CheckType, task.Enable)
	if task.Enable != 1 {
		time.Sleep(10 * time.Second)
	} else {
		checkInterval := time.Duration(task.Interval) * time.Minute
		strategy := StrategyMap[task.CheckType]
		if !strategy.Check(task.ClusterName, task.Uuid) {
			log.Println(task.ClusterName, task.Uuid, task.CheckType, " is not running, waiting for ", checkInterval)
			time.Sleep(checkInterval)
			if !strategy.Check(task.ClusterName, task.Uuid) {
				log.Println(task.ClusterName, task.Uuid, task.CheckType, "has not started, starting it...")
				err := strategy.Run(task.ClusterName, task.Uuid)
				if err != nil {
					log.Println(err)
				}
			}
		}
		log.Println("check true ", task.ClusterName, task.LevelName, task.CheckType)
		time.Sleep(checkInterval)
	}
}

// stopChan 返回任务的停止通道，不存在则创建（B9: Start* 系列此前拿到恒 nil 的 channel，run 无法停止）
func (m *AutoCheckManager) stopChan(taskId string) chan int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ch, ok := m.statusMap[taskId]
	if !ok {
		ch = make(chan int)
		m.statusMap[taskId] = ch
	}
	return ch
}

func (m *AutoCheckManager) AddAutoCheckTasks(task model.AutoCheck) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	db := database.DB
	db.Save(&task)

	taskId := task.Uuid + "|" + task.CheckType // B9: uuid+checkType 复合键，避免多任务互踩 channel
	if oldChan, ok := m.statusMap[taskId]; ok {
		close(oldChan) // 关闭旧的通道
	}
	m.statusMap[taskId] = make(chan int)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		m.run(&task, m.statusMap[taskId])
	}()

}

func (m *AutoCheckManager) DeleteAutoCheck(taskId string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var autoCheck model.AutoCheck
	db := database.DB
	db.Where("uuid = ? OR level_name = ?", taskId, taskId).Delete(&autoCheck) // B9: Where 在 Delete 前；兼容 uuid/level_name 两种调用

	for key, ch := range m.statusMap {
		if key == taskId || strings.HasPrefix(key, taskId+"|") {
			close(ch)
			delete(m.statusMap, key)
		}
	}

}

var StrategyMap = map[string]CheckStrategy{}

func init() {
	StrategyMap[consts.UPDATE_GAME] = &GameUpdateCheck{}
	StrategyMap[consts.LEVEL_MOD] = &LevelModCheck{}
	StrategyMap[consts.LEVEL_DOWN] = &LevelDownCheck{}
}

type CheckStrategy interface {
	Check(string, string) bool
	Run(string, string) error
}

type LevelModCheck struct{}

func (s *LevelModCheck) Check(clusterName, levelName string) bool {
	// 找到当前存档的modId, 然后根据判断当前存档的
	modoverridesPath := dstUtils.GetLevelModoverridesPath(clusterName, levelName)
	content, err := fileUtils.ReadFile(modoverridesPath)
	if err != nil {
		return true
	}
	workshopIds := dstUtils.WorkshopIds(content)
	if len(workshopIds) == 0 {
		return true
	}

	acfPath := dstUtils.GetUgcAcfPath(clusterName, levelName)
	acfWorkshops := dstUtils.ParseACFFile(acfPath)

	log.Println("acf path: ", acfPath)
	// log.Println("acf workshops: ", acfWorkshops)

	activeModMap := make(map[string]dstUtils.WorkshopItem)
	for i := range workshopIds {
		key := workshopIds[i]
		value, ok := acfWorkshops[key]
		if ok {
			activeModMap[key] = value
		}
	}
	return diffFetchModInfo2(activeModMap)
}

// Run 更新会重启世界
func (s *LevelModCheck) Run(clusterName, levelName string) error {
	log.Println("正在更新模组 ", clusterName, levelName)
	SendAnnouncement2(clusterName, levelName, consts.LEVEL_MOD)

	// TODO 删除acf文件
	// fileUtils.DeleteFile(dstUtils.GetUgcAcfPath(clusterName, levelName))

	cluster := clusterUtils.GetCluster(clusterName)
	bin := cluster.Bin
	beta := cluster.Beta
	gameService.StartLevel(clusterName, levelName, bin, beta)
	return nil
}

type LevelDownCheck struct{}

func (s *LevelDownCheck) Check(clusterName, levelName string) bool {
	logRecord := logRecordService.GetLastLog(clusterName, levelName)
	if logRecord.ID != 0 {
		if logRecord.Action == model.STOP {
			return true
		}
	}

	status := gameService.GetLevelStatus(clusterName, levelName)
	log.Println("世界状态", "clusterName", clusterName, "levelName", levelName, "status", status)
	return status
}

func (s *LevelDownCheck) Run(clusterName, levelName string) error {
	log.Println("正在启动世界 ", clusterName, levelName)
	if !gameService.GetLevelStatus(clusterName, levelName) {
		cluster := clusterUtils.GetCluster(clusterName)
		bin := cluster.Bin
		beta := cluster.Beta
		gameService.StartLevel(clusterName, levelName, bin, beta)
	}
	return nil
}

type GameUpdateCheck struct{}

func (s *GameUpdateCheck) Check(clusterName, levelName string) bool {
	localDstVersion := gameService.GetLocalDstVersion(clusterName)
	lastDstVersion := gameService.GetLastDstVersion()
	log.Println("localDstVersion", localDstVersion, "lastDstVersion", lastDstVersion, lastDstVersion < localDstVersion)
	return lastDstVersion <= localDstVersion
}

func (s *GameUpdateCheck) Run(clusterName, levelName string) error {
	log.Println("正在更新游戏 ", clusterName, levelName)
	SendAnnouncement2(clusterName, levelName, consts.UPDATE_GAME)
	gameService.UpdateGame(clusterName)
	time.Sleep(3 * time.Minute)

	// 只启动选择的世界
	levelConfig, err := levelConfigUtils.GetLevelConfig(clusterName)
	if err != nil {
		return err
	}
	dstConfig := dstConfigUtils.GetDstConfig()
	for i := range levelConfig.LevelList {
		item := levelConfig.LevelList[i]

		logRecord := logRecordService.GetLastLog(clusterName, item.File)
		if logRecord.ID != 0 {
			if logRecord.Action == model.STOP {
				continue
			} else {
				log.Println("正在重启", clusterName, item.File, dstConfig.Bin, dstConfig.Beta)
				gameService.StartLevel(clusterName, item.File, dstConfig.Bin, dstConfig.Beta)
				time.Sleep(30 * time.Second)
			}
		}
	}

	// gameService.StartGame(clusterName)
	return nil
}

func SendAnnouncement2(clusterName string, levelName string, checkType string) {
	db := database.DB
	autoCheck := model.AutoCheck{}
	db.Where("cluster_name = ? and uuid = ? and check_type = ?", clusterName, levelName, checkType).Find(&autoCheck)
	size := autoCheck.Times
	for i := 0; i < size; i++ {
		announcement := autoCheck.Announcement
		if announcement != "" {
			lines := strings.Split(announcement, "\n")
			for j := range lines {
				gameConsoleService.SentBroadcast2(clusterName, levelName, lines[j])
				time.Sleep(300 * time.Millisecond)
			}
		}
		time.Sleep(time.Duration(autoCheck.Sleep) * time.Second)
	}

}

func diffFetchModInfo2(activeModMap map[string]dstUtils.WorkshopItem) (noUpdate bool) {

	noUpdate = true
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			noUpdate = true // B8: 异常视为"无更新"，避免 Steam API 抖动导致世界被无端重启
		}
	}()

	var modIds []string
	for key := range activeModMap {
		modIds = append(modIds, key)
	}

	urlStr := "http://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", steamAPIKey)
	data.Set("language", "6")
	for i := range modIds {
		data.Set("publishedfileids["+strconv.Itoa(i)+"]", modIds[i])
	}
	urlStr = urlStr + "?" + data.Encode()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return true
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return true
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return true
	}

	respObj, ok := result["response"].(map[string]interface{})
	if !ok {
		return true
	}
	dataList, ok := respObj["publishedfiledetails"].([]interface{})
	if !ok {
		return true
	}
	for i := range dataList {

		data2, ok := dataList[i].(map[string]interface{})
		if !ok {
			continue
		}
		timeUpdated, find := data2["time_updated"].(float64)
		modId, ok2 := data2["publishedfileid"].(string)
		if find && ok2 {
			value, ok := activeModMap[modId]
			if ok {
				if timeUpdated > float64(value.TimeUpdated) {
					return false
				}
			}
		}

	}

	return true
}

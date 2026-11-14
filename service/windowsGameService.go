package service

import (
	"dst-admin-go/service/mod"
	"dst-admin-go/model"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/levelConfigUtils"
	"dst-admin-go/utils/shellUtils"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WindowsGameService struct {
	lock        sync.Mutex
	homeService HomeService
	logRecord   LogRecordService
}

func (g *WindowsGameService) GetLastDstVersion() int64 {

	url := "http://ver.tugos.cn/getLocalVersion"
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return 0
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	s := string(body)
	veriosn, err := strconv.Atoi(s)
	if err != nil {
		veriosn = 0
	}
	return int64(veriosn)
}

func (g *WindowsGameService) GetLocalDstVersion(clusterName string) int64 {
	cluster := clusterUtils.GetCluster(clusterName)
	versionTextPath := filepath.Join(cluster.ForceInstallDir, "version.txt")
	log.Println("versionTextPath", versionTextPath)
	version, err := fileUtils.ReadFile(versionTextPath)
	if err != nil {
		log.Println(err)
		return 0
	}
	version = strings.Replace(version, "\r", "", -1)
	version = strings.Replace(version, "\n", "", -1)
	l, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		log.Println(err)
		return 0
	}
	return l
}

func (g *WindowsGameService) UpdateGame(clusterName string) error {

	g.lock.Lock()
	defer g.lock.Unlock()
	// TODO 关闭相应的世界
	// g.StopGame(clusterName)

	updateGameCMd := dstUtils.GetDstUpdateCmd(clusterName)
	log.Println("正在更新游戏", "cluster: ", clusterName, "command: ", updateGameCMd)
	result, err := shellUtils.ExecuteCommandInWin(updateGameCMd)
	log.Println(result)

	levelConfig, _ := levelConfigUtils.GetLevelConfig(clusterName)
	for i := range levelConfig.LevelList {
		level := g.homeService.GetLevel(clusterName, levelConfig.LevelList[i].File)
		modoverrides := level.Modoverrides
		dstUtils.DedicatedServerModsSetup2(clusterName, modoverrides)
	}

	return err
}

func (g *WindowsGameService) GetLevelStatus(clusterName, level string) bool {
	return clusterContainer.Status(clusterName, level)
}

func (g *WindowsGameService) shutdownLevel(clusterName, level string) {
	if !g.GetLevelStatus(clusterName, level) {
		return
	}
	clusterContainer.StopLevel(clusterName, level)
}

func (g *WindowsGameService) killLevel(clusterName, level string) {
	clusterContainer.Kill(clusterName, level)
}

func (g *WindowsGameService) LaunchLevel(clusterName, level string, bin, beta int) {
	// 启动前同步下载该存档所需模组（已存在跳过）
	if n, err := mod.EnsureClusterMods(clusterName, func(s string) { log.Println("[模组]", s) }); err != nil {
		log.Println("[模组] 下载失败:", err)
	} else if n > 0 {
		log.Println("[模组] 本次同步下载", n, "个")
	}


	if runtime.GOOS == "windows" {
		cluster := clusterUtils.GetCluster(clusterName)
		dstInstallDir := cluster.ForceInstallDir
		if cluster.Beta == 1 {
			dstInstallDir = dstInstallDir + "-beta"
		}
		ugcDirectory := cluster.Ugc_directory
		persistent_storage_root := cluster.Persistent_storage_root
		conf_dir := cluster.Conf_dir
		go func() {
			clusterContainer.StartLevel(clusterName, level, bin, cluster.SteamCmd, dstInstallDir, ugcDirectory, persistent_storage_root, conf_dir)
		}()
		g.logRecord.RecordLog(clusterName, level, model.RUN)
	}

}

func (g *WindowsGameService) StartLevel(clusterName, level string, bin, beta int) {
	g.StopLevel(clusterName, level)
	g.LaunchLevel(clusterName, level, bin, beta)
	ClearScreen()
}

func (g *WindowsGameService) StopLevel(clusterName, level string) {

	g.shutdownLevel(clusterName, level)

	time.Sleep(3 * time.Second)

	if g.GetLevelStatus(clusterName, level) {
		var i uint8 = 1
		for {
			if !g.GetLevelStatus(clusterName, level) { // B5: 世界已停止才结束重试
				break
			}
			g.shutdownLevel(clusterName, level)
			time.Sleep(1 * time.Second)
			i++
			if i > 3 {
				break
			}
		}
	}
	g.killLevel(clusterName, level)

	g.logRecord.RecordLog(clusterName, level, model.STOP)
}

func (g *WindowsGameService) StopGame(clusterName string) {

	config, err := levelConfigUtils.GetLevelConfig(clusterName)
	if err != nil {
		log.Panicln(err)
	}
	var wg sync.WaitGroup
	wg.Add(len(config.LevelList))
	for i := range config.LevelList {
		go func(i int) {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			levelName := config.LevelList[i].File
			g.StopLevel(clusterName, levelName)
		}(i)
	}
	wg.Wait()
}

func (g *WindowsGameService) StartGame(clusterName string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.StopGame(clusterName)
	cluster := clusterUtils.GetCluster(clusterName)
	bin := cluster.Bin
	beta := cluster.Beta

	config, err := levelConfigUtils.GetLevelConfig(clusterName)
	if err != nil {
		log.Panicln(err)
	}
	var wg sync.WaitGroup
	wg.Add(len(config.LevelList))
	for i := range config.LevelList {
		go func(i int) {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			levelName := config.LevelList[i].File
			g.LaunchLevel(clusterName, levelName, bin, beta)
		}(i)
	}
	ClearScreen()
	wg.Wait()
}

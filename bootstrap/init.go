package bootstrap

import (
	"dst-admin-go/collect"
	"dst-admin-go/config"
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	"dst-admin-go/schedule"
	"dst-admin-go/service"
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

const logPath = "./dst-admin-go.log"

var f *os.File

func Init() {

	initConfig()
	initLog()
	initDB()
	initCollect()
	initSchedule()

	initUpdateModinfos()

	InitSnapshotBackup()
}

func initDB() {
	db, err := gorm.Open(sqlite.Open(global.Config.Db), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to connect database")
	}
	database.DB = db
	err = database.DB.AutoMigrate(
		&model.Spawn{},
		&model.PlayerLog{},
		&model.Connect{},
		&model.ModInfo{},
		&model.Cluster{},
		&model.JobTask{},
		&model.AutoCheck{},
		&model.Announce{},
		&model.WebLink{},
		&model.BackupSnapshot{},
		&model.LogRecord{},
		&model.KV{},
	)
	if err != nil {
		return
	}
	// 清理 mod_infos 历史重复行并建立活跃行唯一索引（幂等），杜绝并发重复插入
	if err := mod.EnsureModInfoUniqueness(database.DB); err != nil {
		log.Println("警告: mod_infos 唯一性初始化失败: ", err)
	}
	// 高频查询列索引（幂等）
	if err := EnsureQueryIndexes(database.DB); err != nil {
		log.Println("警告: 查询索引创建失败: ", err)
	}
}

func initConfig() {
	yamlFile, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		log.Println("警告: 读取 ./config.yml 失败，使用默认配置: ", err)
		yamlFile = nil
	}
	// C4: Parse 保证非 nil，缺 config.yml / 解析失败均安全回退默认，不再 nil 解引用崩溃
	_config, perr := config.Parse(yamlFile)
	if perr != nil {
		log.Println("警告: config.yml 解析失败，使用默认值: ", perr)
	}
	log.Println("config: ", _config)
	global.Config = _config
}

func initLog() {
	var err error
	f, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}

	// 组合一下即可，os.Stdout代表标准输出流
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	gin.ForceConsoleColor()
	gin.SetMode(gin.DebugMode)
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func initCollect() {

	//var clusters []model.Cluster
	//database.DB.Find(&clusters)
	//for _, cluster := range clusters {
	//	global.CollectMap.AddNewCollect(cluster.ClusterName)
	//}

	// home, _ := systemUtils.Home()
	dstConfig := dstConfigUtils.GetDstConfig()
	clusterName := dstConfig.Cluster
	newCollect := collect.NewCollect(filepath.Join(dstUtils.GetKleiDstPath(), clusterName), clusterName)
	newCollect.StartCollect()
	global.Collect = newCollect

	// autoCheck.AutoCheckObject = autoCheck.NewAutoCheckConfig(clusterName, dstConfig.Bin, dstConfig.Beta)

	autoCheckManager := autoCheck.AutoCheckManager{}
	autoCheck.Manager = &autoCheckManager // B9: 先赋值，避免早期 API 调用时 Manager 为 nil
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		autoCheckManager.Start()
	}()
}

func initSchedule() {
	schedule.ScheduleSingleton = schedule.NewSchedule()
	// service.InitAnnounce()

}

func initUpdateModinfos() {
	if global.Config.AutoUpdateModinfo.Enable {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			t := global.Config.AutoUpdateModinfo.UpdateCheckInterval
			ticker := time.NewTicker(time.Duration(t) * time.Minute)
			for {
				select {
				case <-ticker.C:
					log.Println("正在定时更新模组配置 间隔: ", t, "分钟")
					// TODO 中英文问题，每隔10分钟执行的任务
					mod.UpdateModinfoList("zh")
				}
			}
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
			}()
			t := global.Config.AutoUpdateModinfo.CheckInterval
			ticker := time.NewTicker(time.Duration(t) * time.Minute)
			for {
				select {
				case <-ticker.C:
					log.Println("正在定时检查模组配置是否更新 间隔: ", t, "分钟")
					// 每隔10分钟执行的任务
					mod.CheckModInfoUpdate()
				}
			}
		}()
	}
}

func InitSnapshotBackup() {
	var backupService service.BackupService
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		backupService.ScheduleBackupSnapshots()
	}()
}

package service

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/pathUtils"
	"dst-admin-go/utils/zip"
	"dst-admin-go/vo"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type BackupService struct {
	HomeService
	GameArchive

	console GameConsoleService
}

func (b *BackupService) GetBackupList(clusterName string) []vo.BackupVo {
	cluster := clusterUtils.GetCluster(clusterName)
	var backupPath = cluster.Backup
	var backupList []vo.BackupVo

	if !fileUtils.Exists(backupPath) {
		return backupList
	}
	//获取文件或目录相关信息
	fileInfoList, err := ioutil.ReadDir(backupPath)
	if err != nil {
		log.Panicln(err)
	}

	for _, file := range fileInfoList {
		if file.IsDir() {
			continue
		}
		suffix := filepath.Ext(file.Name())
		if suffix == ".zip" || suffix == ".tar" {
			backup := vo.BackupVo{
				FileName:   file.Name(),
				FileSize:   file.Size(),
				CreateTime: file.ModTime(),
				Time:       file.ModTime().Unix(),
			}
			backupList = append(backupList, backup)
		}
	}

	return backupList

}

func (b *BackupService) RenameBackup(ctx *gin.Context, fileName, newName string) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	backupPath := cluster.Backup
	p1, err := fileUtils.SafeJoin(backupPath, fileName)
	if err != nil {
		return
	}
	p2, err := fileUtils.SafeJoin(backupPath, newName)
	if err != nil {
		return
	}
	err = fileUtils.Rename(p1, p2)
	if err != nil {
		return
	}
}

func (b *BackupService) DeleteBackup(ctx *gin.Context, fileNames []string) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	backupPath := cluster.Backup
	for _, fileName := range fileNames {
		filePath, err := fileUtils.SafeJoin(backupPath, fileName)
		if err != nil {
			log.Println("非法备份名", err)
			continue
		}
		_ = err
		if !fileUtils.Exists(filePath) {
			continue
		}
		err = fileUtils.DeleteFile(filePath)
		if err != nil {
			return
		}
	}

}

func (b *BackupService) RestoreBackup(ctx *gin.Context, backupName string) error {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	filePath, err := fileUtils.SafeJoin(cluster.Backup, backupName)
	if err != nil {
		log.Println("非法备份名", err)
		return err
	}
	clusterPath := filepath.Join(dstUtils.GetClusterBasePath(cluster.ClusterName))
	log.Println("正在恢复存档", filePath, clusterPath)
	// A6: 先解压到临时目录，成功后再替换，避免恢复失败导致存档丢失
	tmpPath := clusterPath + ".restore_tmp"
	_ = fileUtils.DeleteDir(tmpPath)
	if err := zip.Unzip3(filePath, tmpPath); err != nil {
		log.Println("解压失败，原存档保持不变:", err)
		_ = fileUtils.DeleteDir(tmpPath)
		return err
	}
	// 闸门:恢复的存档含海难/火山世界时要求已订阅 IA 模组,未订阅整体拒绝(原存档保持不变)
	if err := mod.CheckIADirGate(cluster.ClusterName, tmpPath); err != nil {
		log.Println("备份恢复被拦截:", err)
		_ = fileUtils.DeleteDir(tmpPath)
		return err
	}
	// I3: 原子交换：旧档先改名，替换失败可回滚，成功后删旧档
	oldPath := clusterPath + ".restore_old"
	if fileUtils.Exists(oldPath) { // I-3: 遗留副本可能是唯一存档，先回滚或清场
		if !fileUtils.Exists(clusterPath) {
			if err := fileUtils.Rename(oldPath, clusterPath); err == nil {
				log.Println("检测到上次未完成恢复，已回滚旧存档")
			}
		} else {
			_ = fileUtils.DeleteDir(oldPath)
		}
	}
	if err := fileUtils.Rename(clusterPath, oldPath); err != nil {
		log.Println("旧存档改名失败:", err)
		_ = fileUtils.DeleteDir(tmpPath)
		return err
	}
	if err := fileUtils.Rename(tmpPath, clusterPath); err != nil {
		log.Println("替换存档目录失败，回滚:", err)
		_ = fileUtils.Rename(oldPath, clusterPath)
		return err
	}
	_ = fileUtils.DeleteDir(oldPath)
	// 安装mod
	modoverride, err := fileUtils.ReadFile(dstUtils.GetMasterModoverridesPath(cluster.ClusterName))
	if err != nil {
		log.Println("读取模组失败", err)
	}
	dstUtils.DedicatedServerModsSetup(cluster.ClusterName, modoverride)
	return nil
}

func (b *BackupService) CreateBackup(clusterName, backupName string) {

	cluster := clusterUtils.GetCluster(clusterName)
	backupPath := cluster.Backup

	saveAllLevels(&b.console, cluster.ClusterName) // M9

	src := dstUtils.GetClusterBasePath(clusterName)
	if !fileUtils.Exists(backupPath) {
		// 备份目录首次使用时自动创建(系统设置里配置的路径可能尚未成型)
		fileUtils.CreateDirIfNotExists(backupPath)
		if !fileUtils.Exists(backupPath) {
			log.Panicln("backup path is not exists")
		}
	}
	if backupName == "" {
		backupName = b.GenGameBackUpName(cluster.ClusterName)
	}
	dst := filepath.Join(backupPath, backupName)
	log.Println("src", src, dst)
	err := zip.Zip(src, dst)
	if err != nil {
		log.Panicln("create backup error", err)
	}
	log.Println("创建备份成功")
}

func (b *BackupService) DownloadBackup(c *gin.Context) {
	fileName := c.Query("fileName")

	clusterName := c.GetHeader("level")
	cluster := clusterUtils.GetCluster(clusterName)

	filePath, err := fileUtils.SafeJoin(cluster.Backup, fileName)
	if err != nil {
		return
	}
	//打开文件
	_, err = os.Open(filePath)
	//非空处理
	if err != nil {
		log.Panicln("download filePath error", err)
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Transfer-Encoding", "binary")
	// c.Header("Content-Length", strconv.FormatInt(f.Size(), 10))
	c.File(filePath)
}

func (b *BackupService) UploadBackup(c *gin.Context) {
	// 单文件
	file, ferr := c.FormFile("file")
	if ferr != nil || file == nil {
		return
	}
	log.Println(file.Filename)

	cluster := clusterUtils.GetClusterFromGin(c)
	if file == nil {
		return
	}
	dst, err := fileUtils.SafeJoin(cluster.Backup, file.Filename)
	if err != nil {
		return
	}

	if fileUtils.Exists(dst) {
		log.Panicln("backup is existed")
	}

	// 上传文件至指定的完整文件路径
	err = c.SaveUploadedFile(file, dst)
	if err != nil {
		return
	}

	// c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))

}

func (b *BackupService) ScheduleBackupSnapshots() {
	log.Println("开始创建快照")
	for {
		db := database.DB
		snapshot := model.BackupSnapshot{}
		db.First(&snapshot)
		if snapshot.Enable == 1 {
			if snapshot.Interval == 0 {
				snapshot.Interval = 8
			}
			if snapshot.MaxSnapshots == 0 {
				snapshot.MaxSnapshots = 6
			}
			time.Sleep(time.Duration(snapshot.Interval) * time.Minute)

			// 定时创建备份，每隔 x 分钟 备份一次
			cluster := clusterUtils.GetCluster("")
			if cluster.ClusterName != "" {
				snapshotPrefix := "(snapshot)"
				if snapshot.IsCSave == 1 {
					saveAllLevels(&b.console, cluster.ClusterName) // M9
				}
				b.CreateSnapshotBackup(snapshotPrefix, cluster.ClusterName)
				// 删除快照
				b.DeleteBackupSnapshots(snapshotPrefix, snapshot.MaxSnapshots, cluster.ClusterName, cluster.Backup)
			}
		} else {
			time.Sleep(1 * time.Minute)
		}
	}
}

// sumMd5 计算目录内容指纹（相对路径+内容哈希后整体求哈希）。
// 纯 Go 实现:旧版走 `find | md5sum` 管道,依赖 sh/md5sum——Windows 无 sh、macOS 是 md5 不是 md5sum,
// 两平台上恒返回 ""，快照备份被"两次md5相等"误判跳过、一次都建不出来。
func sumMd5(filePath string) string {
	var entries []string
	err := filepath.Walk(filePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(filePath, p)
		if rerr != nil {
			rel = p
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		entries = append(entries, filepath.ToSlash(rel)+" "+fmt.Sprintf("%x", md5.Sum(data)))
		return nil
	})
	if err != nil {
		return ""
	}
	sort.Strings(entries)
	return fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(entries, "\n"))))
}

func (b *BackupService) CreateSnapshotBackup(prefix, clusterName string) {

	cluster := clusterUtils.GetCluster(clusterName)

	snapshotMd5FilePath := pathUtils.Data("snapshotMd5")
	fileUtils.CreateFileIfNotExists(snapshotMd5FilePath)
	oldMd5, _ := fileUtils.ReadFile(snapshotMd5FilePath)
	saveDirPath := filepath.Join(dstUtils.GetClusterBasePath(clusterName), "Master", "save")
	md5 := sumMd5(saveDirPath)
	log.Println("oldMd5", oldMd5, "md5", md5)
	if oldMd5 == md5 {
		log.Println("两次md5 相等")
		return
	}
	if md5 != "" {
		fileUtils.WriterTXT(snapshotMd5FilePath, md5)
	}

	src := filepath.Join(dstUtils.GetKleiDstPath(), cluster.ClusterName)
	dst := filepath.Join(cluster.Backup, b.GenBackUpSnapshotName(prefix, cluster.ClusterName))
	log.Println("[Snapshot]正在定时创建游戏备份", "src: ", src, "dst: ", dst)
	err := zip.Zip(src, dst)
	if err != nil {
		log.Println("[Snapshot]create backup error", err)
	}
}

func (b *BackupService) DeleteBackupSnapshots(prefix string, maxSnapshots int, clusterName, backupPath string) {

	log.Println("[Snapshot]正在删除快照备份", "maxSnapshots", maxSnapshots, "clusterName: ", clusterName)

	backupList := b.GetBackupList(clusterName)
	var newBackupList []vo.BackupVo
	for i := range backupList {
		name := backupList[i].FileName
		if strings.HasPrefix(name, prefix) {
			newBackupList = append(newBackupList, backupList[i])
		}
	}
	if len(newBackupList) > maxSnapshots {
		deleteBackupList := newBackupList[:len(newBackupList)-maxSnapshots]
		for i := range deleteBackupList {
			filePath := filepath.Join(backupPath, deleteBackupList[i].FileName)
			log.Println("删除快照备份", filePath)
			if !fileUtils.Exists(filePath) {
				continue
			}
			err := fileUtils.DeleteFile(filePath)
			if err != nil {
				return
			}
		}
	}

}

func (b *BackupService) backupPath() string {
	dstConfig := dstConfigUtils.GetDstConfig()
	backupPath := dstConfigUtils.ResolvePath(dstConfig.Parent_path, dstConfig.Backup, dstConfigUtils.DefaultBackup)
	if !fileUtils.Exists(backupPath) {
		log.Panicln("backup path is not exists")
	}
	return backupPath
}

var SeasonMap = map[string]string{
	"spring": "春天",
	"summer": "夏天",
	"autumn": "秋天",
	"winter": "冬天",
}

// GenGameBackUpName 备份名称增加存档信息如  猜猜我是谁的世界-10天-spring-1-20-2023071415
func (b *BackupService) GenGameBackUpName(clusterName string) string {
	name := b.GetClusterIni(clusterName).ClusterName
	snapshoot := b.Snapshoot(clusterName)

	fmt.Printf("%v\n", snapshoot)

	// 20060102150405_猜猜我是谁的世界_40天_秋季(1/20).zip
	// 猜猜我是谁的房间_季节40天spring(1|20)_模组数量3.zip
	days := strconv.Itoa(snapshoot.Clock.Cycles)
	elapsedDayInSeason := strconv.Itoa(snapshoot.Seasons.ElapsedDaysInSeason)
	seasonDays := strconv.Itoa(snapshoot.Seasons.ElapsedDaysInSeason + snapshoot.Seasons.RemainingDaysInSeason)
	archiveDesc := days + "day_" + snapshoot.Clock.Phase + "_" + SeasonMap[snapshoot.Seasons.Season] + "(" + elapsedDayInSeason + "_" + seasonDays + ")"
	backupName := time.Now().Format("2006年01月02日15点04分05秒") + "_" + name + "_" + archiveDesc + ".zip"

	return backupName
}

func (b *BackupService) GenBackUpSnapshotName(prefix, clusterName string) string {
	backupName := b.GenGameBackUpName(clusterName)
	backupName = prefix + backupName
	return backupName
}

// M9: 备份前对所有世界发 c_save
func saveAllLevels(console *GameConsoleService, clusterName string) {
	basePath := dstUtils.GetClusterBasePath(clusterName)
	if entries, err := os.ReadDir(basePath); err == nil {
		for _, e := range entries {
			// F9: 仅对含 server.ini 的世界目录发 c_save，避免对 backup 等目录空转
			if e.IsDir() && fileUtils.Exists(filepath.Join(basePath, e.Name(), "server.ini")) {
				console.CSave(clusterName, e.Name())
			}
		}
	}
}

package service

import (
	"os"
	"path/filepath"

	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
)

// StorageSummary 存储概要:世界存档大小 + 备份数量/总大小(服务器信息卡展示)。
type StorageSummary struct {
	WorldBytes  int64 `json:"worldBytes"`
	BackupCount int   `json:"backupCount"`
	BackupBytes int64 `json:"backupBytes"`
}

// DirSize 递归统计目录占用字节;目录不存在返回 0。
func DirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// SumBackups 汇总备份数量与总字节。
func SumBackups(backups []vo.BackupVo) (int, int64) {
	var bytes int64
	for i := range backups {
		bytes += backups[i].FileSize
	}
	return len(backups), bytes
}

// GetStorageSummary 当前集群的存储概要(世界存档目录 + 备份)。
func GetStorageSummary(clusterName string) StorageSummary {
	bs := &BackupService{}
	count, bytes := SumBackups(bs.GetBackupList(clusterName))
	return StorageSummary{
		WorldBytes:  DirSize(dstUtils.GetClusterBasePath(clusterName)),
		BackupCount: count,
		BackupBytes: bytes,
	}
}

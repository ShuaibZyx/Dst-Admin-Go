package mod

import (
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// 海难/火山是岛屿冒险(IA)模组世界:世界选项必须建立在已订阅 workshop-1467214795 的基础上。
// 未订阅时:写入类(创建/编辑保存/预设/备份恢复)报错,拉起类(宕机恢复/定时/手动启动)拒绝拉起。

const IAModId = "workshop-1467214795"

// ErrIAModNotSubscribed IA 世界闸门错误:模组未订阅/不可用
var ErrIAModNotSubscribed = errors.New("海难/火山是岛屿冒险(IA)模组世界，需先订阅并启用 " + IAModId + " 模组")

// IsSubscribedFn 订阅检查入口(可替换,便于测试)
var IsSubscribedFn = IsModSubscribed

// ReadLevelOverride 读世界 leveldataoverride(可替换,便于测试)
var ReadLevelOverride = func(clusterName, levelName string) string {
	b, err := fileUtils.ReadFile(dstUtils.GetLevelLeveldataoverridePath(clusterName, levelName))
	if err != nil {
		return ""
	}
	return string(b)
}

// IsIAModWorld 判定 leveldataoverride 是否为 IA 模组世界(海难/火山)。
// 注意:IA 模组注册的火山 location 正规 id 是 volcanoworld(见模组 sw_locations.lua),
// 兼容旧模板的 volcano 短名,按前缀匹配。
func IsIAModWorld(leveldataoverride string) bool {
	return strings.Contains(leveldataoverride, `location="shipwrecked`) ||
		strings.Contains(leveldataoverride, `location="volcano`)
}

// CheckIAWorldWrite 写入类闸门:配置内容是 IA 世界时要求已订阅
func CheckIAWorldWrite(clusterName, leveldataoverride string) error {
	if IsIAModWorld(leveldataoverride) && !IsSubscribedFn(clusterName, IAModId) {
		return ErrIAModNotSubscribed
	}
	return nil
}

// CheckIAWorldStart 拉起类闸门:世界是 IA 世界时要求已订阅(宕机恢复/定时/手动启动共用)
func CheckIAWorldStart(clusterName, levelName string) error {
	return CheckIAWorldWrite(clusterName, ReadLevelOverride(clusterName, levelName))
}

// CheckIADirGate 目录闸门:目录内任意世界的 leveldataoverride 是 IA 世界时要求已订阅(备份恢复/预设替换用)
func CheckIADirGate(clusterName, dir string) error {
	var gateErr error
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "leveldataoverride.lua" {
			return nil
		}
		b, rerr := fileUtils.ReadFile(path)
		if rerr != nil {
			return nil
		}
		if werr := CheckIAWorldWrite(clusterName, string(b)); werr != nil {
			gateErr = werr
		}
		return nil
	})
	return gateErr
}

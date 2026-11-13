package migration

import (
	"dst-admin-go/utils/dstConfigUtils"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// IsClusterDir 目录项中含 cluster.ini 或 Master/ Caves/ 子目录即视为 cluster 存档目录
func IsClusterDir(entries []os.DirEntry) bool {
	for _, e := range entries {
		name := e.Name()
		if name == "cluster.ini" && !e.IsDir() {
			return true
		}
		if e.IsDir() && (name == "Master" || name == "Caves") {
			return true
		}
	}
	return false
}

// IsJunkName 不可作为 cluster 名
func IsJunkName(name string) bool {
	switch name {
	case "bak", "backup", "mod_download", "mod_config_download":
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	return strings.Contains(name, ".pre-migrate-")
}

// confDefault 存档 conf_dir 默认目录名
func confDefault(beta int) string {
	if beta == 1 {
		return "DoNotStarveTogetherBetaBranch"
	}
	return "DoNotStarveTogether"
}

// ImportCluster 把已有 cluster 存档目录导入父级 worlds 树（唯一的兼容入口）。
// cluster 名默认取 source 目录名；成功返回目标路径。
func ImportCluster(source, parent, cluster string, beta int) (string, error) {
	if parent == "" {
		return "", errors.New("未配置父级文件夹")
	}
	source = filepath.Clean(source)
	if !dirExists(source) {
		return "", errors.New("源目录不存在: " + source)
	}
	if cluster == "" {
		cluster = filepath.Base(source)
	}
	if IsJunkName(cluster) || cluster == "." || cluster == string(filepath.Separator) {
		return "", errors.New("非法 cluster 名: " + cluster)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return "", err
	}
	if !IsClusterDir(entries) {
		return "", errors.New("源目录不是 cluster 存档（缺 cluster.ini 或 Master/Caves）")
	}
	target := filepath.Join(parent, dstConfigUtils.DefaultWorlds, confDefault(beta), cluster)
	if dirNonEmpty(target) {
		return "", errors.New("目标已存在: " + target)
	}
	if err := MoveDir(source, target); err != nil {
		return "", err
	}
	return target, nil
}

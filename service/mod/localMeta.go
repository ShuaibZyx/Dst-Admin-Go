package mod

import (
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LocalModMeta 从本地模组文件兜底读取名字与图标路径（库里缺元数据时用）。
// 依次在 UGC 目录、服务器 mods 目录、模组下载缓存里找 workshop-<id> / <id> 目录。
func LocalModMeta(modid string) (name string, iconPath string) {
	for _, dir := range localModDirs(modid) {
		if !dirExists(dir) {
			continue
		}
		if name == "" {
			name = parseModinfoName(filepath.Join(dir, "modinfo.lua"))
		}
		if iconPath == "" {
			for _, icon := range []string{"icon.png", "preview.png"} {
				p := filepath.Join(dir, icon)
				if fileExists(p) {
					iconPath = p
					break
				}
			}
		}
		if name != "" && iconPath != "" {
			return name, iconPath
		}
	}
	return name, iconPath
}

// LocalModIcon 按 modid 找本地图标文件
func LocalModIcon(modid string) string {
	_, iconPath := LocalModMeta(modid)
	return iconPath
}

func localModDirs(modid string) []string {
	var dirs []string
	cfg := dstConfigUtils.GetDstConfig()
	cluster := clusterUtils.GetCluster(cfg.Cluster)

	// UGC：优先启用 ugc_directory 的布局，其次服务器目录下的 ugc_mods
	dirs = append(dirs,
		filepath.Join(dstUtils.GetUgcModPath(), "content", "322330", modid),
		filepath.Join(dstUtils.GetUgcModPath(), cluster.ClusterName, "Master", "content", "322330", modid),
		filepath.Join(dstUtils.GetUgcModPath(), cluster.ClusterName, "Caves", "content", "322330", modid),
	)
	// 服务器 mods 目录（workshop-<id>）
	dirs = append(dirs,
		filepath.Join(cluster.ForceInstallDir, "mods", "workshop-"+modid),
		filepath.Join(cluster.ForceInstallDir, "mods", modid),
	)
	// 模组下载缓存
	dirs = append(dirs,
		filepath.Join(cluster.ModDownloadPath, "steamapps", "workshop", "content", "322330", modid),
	)
	return dirs
}

var modNameRe = regexp.MustCompile(`(?m)^\s*name(?:\.[a-zA-Z_-]+)?\s*=\s*["']([^"']+)["']`)

// parseModinfoName 从 modinfo.lua 提取 name（优先 name.zh）
func parseModinfoName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	// 优先中文名
	if m := regexp.MustCompile(`(?m)^\s*name\.zh\s*=\s*["']([^"']+)["']`).FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	if m := modNameRe.FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

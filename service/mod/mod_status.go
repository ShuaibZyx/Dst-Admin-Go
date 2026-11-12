package mod

import (
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"os"
	"path/filepath"
	"strings"
)

// IsModSubscribed 检查模组是否已"订阅/可用"：
//  1. 已下载到 mods/workshop-<id>
//  2. 已下载到 ugc_mods(.../content/322330/<id>)
//  3. 在 dedicated_server_mods_setup.lua 订阅列表(ServerModSetup)
//
// 海难/火山是 IA 模组世界，模组未订阅/下载时世界无法加载，故创建前需拦截。
func IsModSubscribed(clusterName, workshopId string) bool {
	id := strings.TrimPrefix(workshopId, "workshop-")

	// 1) mods/workshop-<id>
	cluster := clusterUtils.GetCluster(clusterName)
	if cluster.ForceInstallDir != "" {
		modsDir := filepath.Join(cluster.ForceInstallDir, "mods", "workshop-"+id)
		if fileUtils.Exists(modsDir) {
			return true
		}
	}

	// 2) ugc_mods(content/322330/<id>)
	ugcPath := dstUtils.GetUgcWorkshopModPath(clusterName, "Master", id)
	if ugcPath != "" && fileUtils.Exists(ugcPath) {
		return true
	}

	// 3) dedicated_server_mods_setup.lua 订阅列表
	setupPath := dstUtils.GetModSetup2(clusterName)
	if setupPath != "" && fileUtils.Exists(setupPath) {
		lines, err := fileUtils.ReadLnFile(setupPath)
		if err == nil {
			for _, ln := range lines {
				if strings.Contains(ln, `ServerModSetup("`+id+`")`) || strings.Contains(ln, `ServerModSetup("workshop-`+id+`")`) {
					return true
				}
			}
		}
	}

	// 4) Steam workshop 内容库(Steam 订阅下载)—ForceInstallDir 未配置时的兜底
	if steamWorkshopModExists(id) {
		return true
	}

	return false
}

// steamWorkshopModExists 在常见 Steam 库路径下查找 workshop 模组(Steam 订阅信号)。
func steamWorkshopModExists(id string) bool {
	drives := []string{"C:", "D:", "E:", "F:"}
	libs := []string{
		`SteamLibrary\steamapps`,
		`Steam\steamapps`,
		`Program Files (x86)\Steam\steamapps`,
		`Program Files\Steam\steamapps`,
	}
	for _, d := range drives {
		for _, lib := range libs {
			p := filepath.Join(d+string(os.PathSeparator), lib, "workshop", "content", "322330", id)
			if fileUtils.Exists(p) {
				return true
			}
		}
	}
	return false
}

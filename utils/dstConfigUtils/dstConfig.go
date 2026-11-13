package dstConfigUtils

import (
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/systemUtils"
	"log"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func KleiDstPath() string {
	home, err := systemUtils.Home()
	if err != nil {
		panic("Home path error: " + err.Error())
	}
	klei := ""
	if runtime.GOOS == "windows" {
		klei = filepath.Join(home, "Documents", "klei", "DoNotStarveTogether")
	} else {
		klei = filepath.Join(home, ".klei", "DoNotStarveTogether")
	}
	if IsBeta() {
		klei = klei + "BetaBranch"
	}
	return klei
}

func IsBeta() bool {
	//判断是否存在，不存在创建一个
	if !fileUtils.Exists(dst_config_path) {
		if err := fileUtils.CreateFile(dst_config_path); err != nil {
			log.Panicln("create docker_dst_config error", err)
		}
	}
	data, err := fileUtils.ReadLnFile(dst_config_path)
	if err != nil {
		log.Panicln("read docker_dst_config error", err)
	}
	for _, value := range data {
		if value == "" {
			continue
		}
		if strings.Contains(value, "beta=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				beta, _ := strconv.ParseInt(s, 10, 64)
				if beta == 1 {
					return true
				}
			}
		}
	}
	return false
}

type DstConfig struct {
	Steamcmd                   string `json:"steamcmd"`
	Force_install_dir          string `json:"force_install_dir"`
	DoNotStarveServerDirectory string `json:"donot_starve_server_directory"`
	Cluster                    string `json:"cluster"`
	Parent_path                string `json:"parent_path"`
	Last_parent_path           string `json:"last_parent_path"`
	Backup                     string `json:"backup"`
	Mod_download_path          string `json:"mod_download_path"`
	Bin                        int    `json:"bin"`
	Beta                       int    `json:"beta"`

	Ugc_directory string `json:"ugc_directory"`
	// 根目录位置
	Persistent_storage_root string `json:"persistent_storage_root"`
	// 存档相对位置
	Conf_dir string `json:"conf_dir"`
}

var dst_config_path = "./dst_config"

// SetDstConfigPathForTest 供跨包测试替换配置文件路径，返回恢复函数
func SetDstConfigPathForTest(p string) func() {
	old := dst_config_path
	dst_config_path = p
	return func() { dst_config_path = old }
}

// 相对路径默认值（§7），空值时参与 ResolvePath 判定
const (
	DefaultSteamcmd  = "steamcmd"
	DefaultServerDir = "dst-dedicated-server"
	DefaultBackup    = "backup"
	DefaultMod       = "mod"
	DefaultWorlds    = "worlds"
)

// toRel 把任意形式的路径剥成相对子路径：去掉盘符、~ 与前导分隔符
func toRel(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if i := strings.Index(p, ":"); i >= 0 {
		p = p[i+1:]
	}
	p = strings.TrimPrefix(p, "~")
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return filepath.FromSlash(p)
}

// ResolvePath 强制放在父级文件夹下（不兼容旧配置）：一切输入都当作相对子路径。
// parent 为空视为未配置，返回空；空值取默认值 def。
func ResolvePath(parent, p, def string) string {
	if parent == "" {
		return ""
	}
	if p == "" {
		p = def
	}
	if p == "" {
		return ""
	}
	return filepath.Join(parent, toRel(p))
}

func NewDstConfig() *DstConfig {
	return &DstConfig{}
}

func GetDstConfig() DstConfig {

	dstConfig := NewDstConfig()

	//判断是否存在，不存在创建一个
	if !fileUtils.Exists(dst_config_path) {
		if err := fileUtils.CreateFile(dst_config_path); err != nil {
			log.Panicln("create docker_dst_config error", err)
		}

	}
	data, err := fileUtils.ReadLnFile(dst_config_path)
	if err != nil {
		log.Panicln("read docker_dst_config error", err)
	}
	for _, value := range data {
		if value == "" {
			continue
		}
		// TODO 这里解析有问题，如果路径含有 steamcmd 就会存在问题
		if strings.Contains(value, "steamcmd=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Steamcmd = s
			}
		}
		if strings.Contains(value, "force_install_dir=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Force_install_dir = s
			}
		}
		if strings.Contains(value, "donot_starve_server_directory=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.DoNotStarveServerDirectory = s
			}
		}
		if strings.Contains(value, "persistent_storage_root=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Persistent_storage_root = s
			}
		}
		if strings.Contains(value, "cluster=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Cluster = s
			}
		}
		if strings.HasPrefix(value, "parent_path=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Parent_path = s
			}
		}
		if strings.HasPrefix(value, "last_parent_path=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Last_parent_path = s
			}
		}
		if strings.Contains(value, "backup=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Backup = s
			}
		}
		if strings.Contains(value, "mod_download_path=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Mod_download_path = s
			}
		}
		if strings.Contains(value, "bin=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				bin, _ := strconv.ParseInt(s, 10, 64)
				dstConfig.Bin = int(bin)
			}
		}
		if strings.Contains(value, "beta=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				beta, _ := strconv.ParseInt(s, 10, 64)
				dstConfig.Beta = int(beta)
			}
		}
		if strings.Contains(value, "ugc_directory=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Ugc_directory = s
			}
		}
		if strings.Contains(value, "conf_dir=") {
			split := strings.Split(value, "=")
			if len(split) > 1 {
				s := strings.TrimSpace(split[1])
				dstConfig.Conf_dir = s
			}
		}
	}
	// 设置默认值
	if dstConfig.Cluster == "" {
		dstConfig.Cluster = "Cluster1"
	}
	// 不兼容旧配置：不做绝对默认值物化，空值由 ResolvePath 供给默认
	if dstConfig.Bin == 0 {
		dstConfig.Bin = 32
	}
	return *dstConfig
}

func SaveDstConfig(dstConfig *DstConfig) {
	log.Println(dstConfig)

	oldDstConfig := GetDstConfig()
	if dstConfig.Steamcmd == "" {
		dstConfig.Steamcmd = oldDstConfig.Steamcmd
	}
	if dstConfig.Force_install_dir == "" {
		dstConfig.Force_install_dir = oldDstConfig.Force_install_dir
	}
	//if dstConfig.Ugc_directory == "" {
	//	dstConfig.Ugc_directory = oldDstConfig.Ugc_directory
	//}
	//if dstConfig.Conf_dir == "" {
	//	dstConfig.Conf_dir = oldDstConfig.Conf_dir
	//}
	//if dstConfig.Persistent_storage_root == "" {
	//	dstConfig.Persistent_storage_root = oldDstConfig.Persistent_storage_root
	//}
	if dstConfig.Cluster == "" {
		dstConfig.Cluster = oldDstConfig.Cluster
	}
	if dstConfig.Backup == "" {
		dstConfig.Backup = oldDstConfig.Backup
	}
	if dstConfig.Mod_download_path == "" {
		dstConfig.Mod_download_path = oldDstConfig.Mod_download_path
	}
	// parent 变更且旧值非空时记录 last_parent_path；否则保留原值
	if dstConfig.Parent_path != oldDstConfig.Parent_path && oldDstConfig.Parent_path != "" {
		dstConfig.Last_parent_path = oldDstConfig.Parent_path
	} else {
		dstConfig.Last_parent_path = oldDstConfig.Last_parent_path
	}

	err := fileUtils.WriterLnFile(dst_config_path, []string{
		"parent_path=" + dstConfig.Parent_path,
		"last_parent_path=" + dstConfig.Last_parent_path,
		"steamcmd=" + dstConfig.Steamcmd,
		"force_install_dir=" + dstConfig.Force_install_dir,
		"donot_starve_server_directory=" + dstConfig.DoNotStarveServerDirectory,
		"ugc_directory=" + dstConfig.Ugc_directory,
		"conf_dir=" + dstConfig.Conf_dir,
		"persistent_storage_root=" + dstConfig.Persistent_storage_root,
		"cluster=" + dstConfig.Cluster,
		"backup=" + dstConfig.Backup,
		"mod_download_path=" + dstConfig.Mod_download_path,
		"bin=" + strconv.Itoa(dstConfig.Bin),
		"beta=" + strconv.Itoa(dstConfig.Beta),
	})
	if err != nil {
		log.Panicln("write docker_dst_config error:", err)
	}
	// global.Collect.ReCollect(filepath.Join(dstUtils.GetKleiDstPath(), dstConfig.Cluster), dstConfig.Cluster)
}

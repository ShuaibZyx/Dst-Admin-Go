package dstConfigUtils

import (
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/systemUtils"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 目录默认值（旧单数写法由 legacyAlias 在读取时归一）
const (
	DefaultSteamcmd  = "steamcmd"
	DefaultServerDir = "dst-dedicated-server"
	DefaultMod       = "mods"
	DefaultBackup    = "backups"
	DefaultWorlds    = "worlds"
)

var dst_config_path = "./dst_config"

// SetDstConfigPathForTest 测试用：替换 dst_config 路径，返回恢复函数
func SetDstConfigPathForTest(p string) func() {
	old := dst_config_path
	dst_config_path = p
	return func() { dst_config_path = old }
}

// DstConfig 面板根配置（./dst_config，key=value 行式）
type DstConfig struct {
	Parent_path             string `json:"parent_path"`
	Last_parent_path        string `json:"last_parent_path"`
	Steamcmd                string `json:"steamcmd"`
	Force_install_dir       string `json:"force_install_dir"`
	Conf_dir                string `json:"conf_dir"`
	Persistent_storage_root string `json:"persistent_storage_root"`
	Cluster                 string `json:"cluster"`
	Backup                  string `json:"backup"`
	Mod_download_path       string `json:"mod_download_path"`
	Bin                     int    `json:"bin"`
	Beta                    int    `json:"beta"`
}

// legacyAlias 旧单数默认值 → 复数（旧配置残留值在读取时归一，避免继续写单数目录）
var legacyAlias = map[string]string{
	"backup": "backups",
	"mod":    "mods",
	"world":  "worlds",
}

// NormalizeAlias 旧单数目录别名归一（mod→mods 等）
func NormalizeAlias(p string) string {
	if v, ok := legacyAlias[p]; ok {
		return v
	}
	return p
}

// toRel 把绝对路径/家目录前缀剥成相对段（一切输入强制落在父级文件夹下）
func toRel(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "~/")
	p = strings.TrimPrefix(p, "~"+string(os.PathSeparator))
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}
	p = strings.TrimPrefix(p, string(os.PathSeparator))
	p = strings.TrimPrefix(p, "/")
	return p
}

// ResolvePath 拼接父级与子路径：空值取默认，别名归一，绝对路径剥成相对
func ResolvePath(parent, p, def string) string {
	if parent == "" {
		return ""
	}
	if p == "" {
		p = def
	}
	p = NormalizeAlias(p)
	if p == "" {
		return ""
	}
	return filepath.Join(parent, toRel(p))
}

// SaveDirOf 存档目录名（conf_dir），空则 worlds；beta 分支带 -beta 后缀
func SaveDirOf(cfg DstConfig) string {
	dir := DefaultWorlds
	if cfg.Conf_dir != "" {
		dir = cfg.Conf_dir
	}
	if cfg.Beta == 1 {
		dir = dir + "-beta"
	}
	return dir
}

// UnifiedUgcDir 统一模组目录 <mod_download>/steamapps/workshop（未配置 parent 返回空）
func UnifiedUgcDir(cfg DstConfig) string {
	if cfg.Parent_path == "" {
		return ""
	}
	return filepath.Join(ResolvePath(cfg.Parent_path, cfg.Mod_download_path, DefaultMod), "steamapps", "workshop")
}

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
	return GetDstConfig().Beta == 1
}

// dst_config 读取缓存（mtime 失效）
var (
	cfgCacheMu   sync.RWMutex
	cfgCache     DstConfig
	cfgCachePath string
	cfgCacheTime time.Time
)

func NewDstConfig() *DstConfig {
	return &DstConfig{}
}

func GetDstConfig() DstConfig {
	if fi, err := os.Stat(dst_config_path); err == nil {
		cfgCacheMu.RLock()
		cached, cpath, ctime := cfgCache, cfgCachePath, cfgCacheTime
		cfgCacheMu.RUnlock()
		if cpath == dst_config_path && !ctime.IsZero() && fi.ModTime().Equal(ctime) {
			return cached
		}
	}

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
		// 精确按 key 匹配:值保留 = 号(Split 全切会截断),key 不做子串匹配(offline_cluster= 不得污染 cluster=)
		key, val, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "steamcmd":
			dstConfig.Steamcmd = val
		case "force_install_dir":
			dstConfig.Force_install_dir = val
		case "persistent_storage_root":
			dstConfig.Persistent_storage_root = val
		case "cluster":
			dstConfig.Cluster = val
		case "parent_path":
			dstConfig.Parent_path = val
		case "last_parent_path":
			dstConfig.Last_parent_path = val
		case "backup":
			dstConfig.Backup = val
		case "mod_download_path":
			dstConfig.Mod_download_path = val
		case "conf_dir":
			dstConfig.Conf_dir = val
		case "bin":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				dstConfig.Bin = int(n)
			}
		case "beta":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				dstConfig.Beta = int(n)
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
	// 写缓存（mtime 失效）
	if fi, err := os.Stat(dst_config_path); err == nil {
		cfgCacheMu.Lock()
		cfgCache, cfgCachePath, cfgCacheTime = *dstConfig, dst_config_path, fi.ModTime()
		cfgCacheMu.Unlock()
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

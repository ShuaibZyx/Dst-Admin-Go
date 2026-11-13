package migration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsClusterDir 目录项中含 cluster.ini 或 Master/ Caves/ 子目录即视为 cluster 存档目录（§11.5）
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

// IsJunkName 旧目录根层的杂物名，不随"世界"类迁移（§11.5）
func IsJunkName(name string) bool {
	switch name {
	case "bak", "backup", "mod_download", "mod_config_download":
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	if strings.Contains(name, ".pre-migrate-") {
		return true
	}
	return false
}

// KnownLegacyRoots 旧默认安装位置清单（§10）；last_parent_path 子树由 scan 侧追加
func KnownLegacyRoots(home string) []string {
	roots := []string{
		filepath.Join(home, "steamcmd"),
		filepath.Join(home, "dst-dedicated-server"),
		filepath.Join(home, "dst"),
		filepath.Join(home, "dontstarve_dedicated_server"),
	}
	klei := filepath.Join(home, ".klei")
	if runtime.GOOS == "windows" {
		klei = filepath.Join(home, "Documents", "klei")
	}
	roots = append(roots,
		filepath.Join(klei, "DoNotStarveTogether"),
		filepath.Join(klei, "DoNotStarveTogetherBetaBranch"),
		"/app/steamcmd",
		"/app/dst-dedicated-server",
		"/app/backup",
		"/app/mod",
	)
	return roots
}

// ClassifyBySignature 按特征文件判定目录类别；无法识别返回 ""。
// steamcmd.exe|steamcmd.sh|steamcmd → steamcmd；bin*/dontstarve_dedicated_server_nullrenderer* → server
func ClassifyBySignature(dir string) string {
	for _, n := range []string{"steamcmd.exe", "steamcmd.sh", "steamcmd"} {
		if fileExists(filepath.Join(dir, n)) {
			return KindSteamcmd
		}
	}
	for _, bin := range []string{"bin", "bin64"} {
		entries, err := os.ReadDir(filepath.Join(dir, bin))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "dontstarve_dedicated_server_nullrenderer") {
				return KindServer
			}
		}
	}
	if dirExists(filepath.Join(dir, "steamapps", "workshop")) {
		return KindMod
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// DiscoveryRoots 扫描发现的候选根（可注入测试）：家目录、cwd、可执行文件目录及其父级、Documents、各盘符根
var DiscoveryRoots = defaultDiscoveryRoots

func defaultDiscoveryRoots(home string) []string {
	roots := []string{home, filepath.Join(home, "Documents")}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd, filepath.Dir(cwd))
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	if runtime.GOOS == "windows" {
		for c := 'A'; c <= 'Z'; c++ {
			root := string(c) + `:\`
			if dirExists(root) {
				roots = append(roots, root)
			}
		}
	} else {
		roots = append(roots, "/", "/app", "/root")
	}
	return roots
}

// KnownNames 一级子目录名 → 类别（特征文件无法判定时的兜底）
var KnownNames = map[string]string{
	"steamcmd":                  KindSteamcmd,
	"dst-dedicated-server":      KindServer,
	"dst":                       KindServer,
	"dontstarve_dedicated_server": KindServer,
	"dontstarve_dedicated_server_x64": KindServer,
	"mod_download":              KindMod,
	"mod_config_download":       KindMod,
	"backup":                    KindBackup,
}

// DiscoverCandidates 在各根的两级子目录里按名字/特征发现数据目录（含 Documents/Klei/steamcmd 这类内嵌布局）
func DiscoverCandidates(roots []string) []scanCandidate {
	var out []scanCandidate
	seen := map[string]bool{}
	add := func(kind, path string) {
		key := cleanKey(path)
		if seen[key] || !dirExists(path) {
			return
		}
		seen[key] = true
		out = append(out, scanCandidate{kind: kind, path: path, label: LabelLegacyDefault, id: kind + ":discovered:" + filepath.Base(path)})
	}
	scanOne := func(dir string) {
		kind := ""
		if k, ok := KnownNames[filepath.Base(dir)]; ok {
			kind = k
		}
		if sig := ClassifyBySignature(dir); sig != "" {
			kind = sig
		}
		if kind == "" {
			return
		}
		add(kind, dir)
		// steamcmd 下的 workshop 缓存单独出 mod 单元
		if kind == KindSteamcmd && dirExists(filepath.Join(dir, "steamapps", "workshop")) {
			add(KindMod, filepath.Join(dir, "steamapps"))
		}
	}
	skip := func(name string) bool {
		if name == ".pre-migrate-" || strings.Contains(name, ".pre-migrate-") || name == "bak" {
			return true
		}
		// 隐藏目录仅放行 .klei
		return strings.HasPrefix(name, ".") && name != ".klei"
	}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > 3 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skip(e.Name()) {
				continue
			}
			sub := filepath.Join(dir, e.Name())
			scanOne(sub)
			walk(sub, depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 1)
	}
	return out
}

// IsNestedOrSame 规范化后同路径或父子关系（对称），用于去重（§11.5）
func IsNestedOrSame(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if a == b {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(a, b+sep) || strings.HasPrefix(b, a+sep)
}

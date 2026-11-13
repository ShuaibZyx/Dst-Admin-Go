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

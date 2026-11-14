package mod

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"regexp"
)

// ModStatus 模组本地文件状态
type ModStatus struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // ok | missing | broken
	Path   string `json:"path"`
}

// VerifyClusterMods 校验当前存档模组的本地文件完整性
func VerifyClusterMods(clusterName string) []ModStatus {
	ids := ClusterModIDs(clusterName)
	cfg := dstConfigUtils.GetDstConfig()
	modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
	out := make([]ModStatus, 0, len(ids))
	for _, id := range ids {
		dir := filepath.Join(modsRoot, id)
		st := ModStatus{Id: id, Path: dir, Name: modDisplayName(dir, id)}
		switch {
		case !dirExists(dir):
			st.Status = "missing"
		case !fileExists(filepath.Join(dir, "modinfo.lua")):
			st.Status = "broken"
		default:
			st.Status = "ok"
		}
		out = append(out, st)
	}
	return out
}

var modNameRe = regexp.MustCompile(`(?m)^\s*name\.?([a-zA-Z_-]*)\s*=\s*["']([^"']+)["']`)

// modDisplayName 名称优先取 modinfo 的 name.zh，其次 name，最后回落 id
func modDisplayName(dir, id string) string {
	data, err := os.ReadFile(filepath.Join(dir, "modinfo.lua"))
	if err != nil {
		return id
	}
	name, generic := "", ""
	for _, m := range modNameRe.FindAllStringSubmatch(string(data), -1) {
		switch m[1] {
		case "zh":
			name = m[2]
		case "":
			generic = m[2]
		}
	}
	if name != "" {
		return name
	}
	if generic != "" {
		return generic
	}
	return id
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

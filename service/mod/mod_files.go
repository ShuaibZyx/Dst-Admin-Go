package mod

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/fileUtils"
)

// 仅接受纯数字 workshop id，防路径穿越（modId 来自 URL 参数，会拼进文件路径）
var workshopIdPattern = regexp.MustCompile(`^[0-9]+$`)

// modFileCandidatePaths 列出该模组在磁盘上的所有已知位置：
// 统一模组目录、steamcmd 暂存残留、全局与各集群服务端的 mods/workshop-<id>（与 IsModSubscribed 检查路径一致）。
func modFileCandidatePaths(modId string) []string {
	cfg := dstConfigUtils.GetDstConfig()
	var paths []string

	modRoot := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod)
	if modRoot != "" {
		paths = append(paths, filepath.Join(modRoot, "steamapps", "workshop", "content", "322330", modId))
	}
	steamcmdRoot := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
	if steamcmdRoot != "" {
		paths = append(paths, filepath.Join(steamcmdRoot, "steamapps", "workshop", "content", "322330", modId))
	}
	serverRoot := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir)
	if serverRoot != "" {
		paths = append(paths, filepath.Join(serverRoot, "mods", "workshop-"+modId))
	}

	var clusters []model.Cluster
	if err := database.DB.Find(&clusters).Error; err == nil {
		for _, c := range clusters {
			if c.ForceInstallDir == "" {
				continue
			}
			paths = append(paths, filepath.Join(c.ForceInstallDir, "mods", "workshop-"+modId))
		}
	}

	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// RemoveModFiles 全量清扫模组文件（设计：docs/superpowers/specs/2026-11-15-delete-mod-cleanup-design.md）。
// 非纯数字 modId（本地模组/虚拟模组）不清扫，直接返回 nil。
// 尽量删除所有候选路径；任一失败继续尝试其余位置，返回第一个含路径的错误。
func RemoveModFiles(modId string) error {
	if !workshopIdPattern.MatchString(modId) {
		return nil
	}
	var firstErr error
	for _, p := range modFileCandidatePaths(modId) {
		if !fileUtils.Exists(p) {
			continue
		}
		if err := fileUtils.DeleteDir(p); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("模组文件删除失败: %s: %w", p, err)
		}
	}
	return firstErr
}

// modSetupFileCandidates 返回各处 dedicated_server_mods_setup.lua（全局服务端 + 各集群 ForceInstallDir）。
func modSetupFileCandidates() []string {
	cfg := dstConfigUtils.GetDstConfig()
	var paths []string
	if serverRoot := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir); serverRoot != "" {
		paths = append(paths, filepath.Join(serverRoot, "mods", "dedicated_server_mods_setup.lua"))
	}
	var clusters []model.Cluster
	if err := database.DB.Find(&clusters).Error; err == nil {
		for _, c := range clusters {
			if c.ForceInstallDir == "" {
				continue
			}
			paths = append(paths, filepath.Join(c.ForceInstallDir, "mods", "dedicated_server_mods_setup.lua"))
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// RemoveModSetupLines 从各处 dedicated_server_mods_setup.lua 摘除该模组的 ServerModSetup 订阅行。
// DedicatedServerModsSetup2 是只增不减的合并语义，删除模组必须显式摘行（IsModSubscribed 会读该文件判订阅）。
func RemoveModSetupLines(modId string) error {
	targets := map[string]bool{
		`ServerModSetup("` + modId + `")`:          true,
		`ServerModSetup("workshop-` + modId + `")`: true,
	}
	var firstErr error
	for _, p := range modSetupFileCandidates() {
		if !fileUtils.Exists(p) {
			continue
		}
		lines, err := fileUtils.ReadLnFile(p)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("订阅文件读取失败: %s: %w", p, err)
			}
			continue
		}
		kept := make([]string, 0, len(lines))
		changed := false
		for _, ln := range lines {
			if targets[strings.TrimSpace(ln)] {
				changed = true
				continue
			}
			kept = append(kept, ln)
		}
		if !changed {
			continue
		}
		if err := fileUtils.WriterLnFile(p, kept); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("订阅文件写入失败: %s: %w", p, err)
		}
	}
	return firstErr
}

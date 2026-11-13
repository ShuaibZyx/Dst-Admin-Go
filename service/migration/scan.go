package migration

import (
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/systemUtils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// homeDir 可注入的用户主目录（测试替换）
var homeDir = systemUtils.Home

// defaultRel 各类别的默认相对值（§7）
func defaultRel(kind string) string {
	switch kind {
	case KindSteamcmd:
		return dstConfigUtils.DefaultSteamcmd
	case KindServer:
		return dstConfigUtils.DefaultServerDir
	case KindMod:
		return dstConfigUtils.DefaultMod
	case KindBackup:
		return dstConfigUtils.DefaultBackup
	case KindWorlds:
		return dstConfigUtils.DefaultWorlds
	}
	return ""
}

// confDefault 存档 conf_dir 默认目录名（§9）
func confDefault(beta int) string {
	if beta == 1 {
		return "DoNotStarveTogetherBetaBranch"
	}
	return "DoNotStarveTogether"
}

type scanCandidate struct {
	kind  string
	path  string
	label string
	id    string
}

// Scan 检测五类资产的可迁移单元（§10）
func Scan(cfg dstConfigUtils.DstConfig, gameRunning bool) ([]MigrationUnit, error) {
	home, err := homeDir()
	if err != nil {
		return nil, err
	}
	parent := cfg.Parent_path
	cands := collectCandidates(cfg, home)

	var units []MigrationUnit
	claimed := map[string]bool{} // 已出单元的 Source（规范化）
	junkNote := map[string]string{}

	for _, c := range cands {
		entries, err := os.ReadDir(c.path)
		exists := err == nil

		// 共享源拆分（§11.5）：源里的 cluster 子目录先出 worlds 单元
		var clusterUnits []MigrationUnit
		var junkNames []string
		if exists {
			for _, e := range entries {
				name := e.Name()
				if IsJunkName(name) {
					junkNames = append(junkNames, name)
					continue
				}
				if !e.IsDir() {
					continue
				}
				sub := filepath.Join(c.path, name)
				subEntries, err := os.ReadDir(sub)
				if err != nil || !IsClusterDir(subEntries) {
					continue
				}
				key := cleanKey(sub)
				if claimed[key] {
					continue
				}
				claimed[key] = true
				clusterUnits = append(clusterUnits, MigrationUnit{
					ID:        "worlds:" + name,
					Kind:      KindWorlds,
					Source:    sub,
					SourceLabel: c.label,
					Target:    filepath.Join(parent, defaultRel(KindWorlds), confDefault(cfg.Beta), name),
				})
			}
		}
		units = append(units, clusterUnits...)

		if c.kind == KindWorlds {
			// worlds 根目录本身不出单元，只出 cluster 子单元
			if len(junkNames) > 0 {
				junkNote[c.path] = "已排除：" + strings.Join(junkNames, ", ")
			}
			continue
		}

		src := c.path
		// server 类 beta 后缀（与 gameService 运行时一致）
		if c.kind == KindServer && cfg.Beta == 1 {
			src = src + "-beta"
		}
		key := cleanKey(src)
		if claimed[key] {
			// 同一 Source 只出现一次
			continue
		}
		claimed[key] = true

		target := filepath.Join(parent, defaultRel(c.kind))
		if c.kind == KindServer && cfg.Beta == 1 {
			target = target + "-beta"
		}
		u := MigrationUnit{
			ID:          c.id,
			Kind:        c.kind,
			Source:      src,
			SourceLabel: c.label,
			Target:      target,
		}
		// 共享源：含 cluster 子单元时只搬顶层文件（§11.5）
		if len(clusterUnits) > 0 {
			u.TopLevelFilesOnly = true
			u.Note = "共享源拆分：cluster 子目录独立迁移"
		}
		units = append(units, u)
	}

	// 已排除杂物备注（挂到同源 worlds 单元）
	for i := range units {
		if units[i].Kind == KindWorlds {
			dir := filepath.Dir(units[i].Source)
			if note, ok := junkNote[dir]; ok {
				if units[i].Note != "" {
					units[i].Note += "; "
				}
				units[i].Note += note
			}
		}
	}

	// 嵌套外层标注跳过子树（§11.5）
	for i := range units {
		for j := range units {
			if i == j {
				continue
			}
			if IsNestedOrSame(units[i].Source, units[j].Source) && cleanKey(units[i].Source) != cleanKey(units[j].Source) {
				if isAncestor(units[i].Source, units[j].Source) {
					if units[i].Note != "" {
						units[i].Note += "; "
					}
					units[i].Note += "跳过子树：" + units[j].Source
					units[i].TopLevelFilesOnly = true
				}
			}
		}
	}

	// 状态与大小
	for i := range units {
		units[i].Status = unitStatus(units[i], parent, gameRunning)
		if size, err := DirSize(units[i].Source, units[i].TopLevelFilesOnly); err == nil {
			units[i].SizeBytes = size
		}
	}
	return units, nil
}

// collectCandidates 当前配置 + 旧默认位置 + 上次父级文件夹
func collectCandidates(cfg dstConfigUtils.DstConfig, home string) []scanCandidate {
	parent := cfg.Parent_path
	var cands []scanCandidate

	// 1. 当前配置解析后的位置
	current := []struct{ kind, path string }{
		{KindSteamcmd, dstConfigUtils.ResolvePath(parent, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)},
		{KindServer, dstConfigUtils.ResolvePath(parent, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir)},
		{KindMod, dstConfigUtils.ResolvePath(parent, cfg.Mod_download_path, dstConfigUtils.DefaultMod)},
		{KindBackup, dstConfigUtils.ResolvePath(parent, cfg.Backup, dstConfigUtils.DefaultBackup)},
		{KindWorlds, currentWorldsRoot(cfg)},
	}
	for _, c := range current {
		if c.path == "" {
			continue
		}
		cands = append(cands, scanCandidate{kind: c.kind, path: c.path, label: LabelCurrentConfig, id: c.kind})
	}

	// 2. 旧默认位置（存在的目录）
	for _, root := range KnownLegacyRoots(home) {
		if !dirExists(root) {
			continue
		}
		kind := classifyLegacyRoot(root)
		if kind == "" {
			continue
		}
		id := kind + ":legacy:" + filepath.Base(root)
		cands = append(cands, scanCandidate{kind: kind, path: root, label: LabelLegacyDefault, id: id})
	}

	// 3. 上次父级文件夹的默认子目录（存在的）
	if cfg.Last_parent_path != "" {
		last := cfg.Last_parent_path
		lastItems := []struct{ kind, path string }{
			{KindSteamcmd, filepath.Join(last, dstConfigUtils.DefaultSteamcmd)},
			{KindServer, filepath.Join(last, dstConfigUtils.DefaultServerDir)},
			{KindMod, filepath.Join(last, dstConfigUtils.DefaultMod)},
			{KindBackup, filepath.Join(last, dstConfigUtils.DefaultBackup)},
			{KindWorlds, filepath.Join(last, dstConfigUtils.DefaultWorlds, confDefault(cfg.Beta))},
		}
		for _, c := range lastItems {
			if !dirExists(c.path) {
				continue
			}
			id := c.kind + ":last:" + filepath.Base(c.path)
			cands = append(cands, scanCandidate{kind: c.kind, path: c.path, label: LabelLastParent, id: id})
		}
	}
	return cands
}

// currentWorldsRoot 当前配置的存档根（与 dstUtils.GetKleiDstPath 同构，但基于入参 cfg）
func currentWorldsRoot(cfg dstConfigUtils.DstConfig) string {
	psr := cfg.Persistent_storage_root
	if cfg.Parent_path != "" {
		psr = dstConfigUtils.ResolvePath(cfg.Parent_path, psr, dstConfigUtils.DefaultWorlds)
	}
	if psr == "" {
		home, err := homeDir()
		if err != nil {
			return ""
		}
		klei := filepath.Join(home, ".klei")
		if runtime.GOOS == "windows" {
			klei = filepath.Join(home, "Documents", "klei")
		}
		suffix := ""
		if cfg.Beta == 1 {
			suffix = "BetaBranch"
		}
		return filepath.Join(klei, "DoNotStarveTogether"+suffix)
	}
	conf := cfg.Conf_dir
	if conf == "" {
		conf = confDefault(cfg.Beta)
	}
	return filepath.Join(psr, conf)
}

// classifyLegacyRoot 旧位置目录名 → 类别；无法识别返回 ""
func classifyLegacyRoot(root string) string {
	base := filepath.Base(root)
	if strings.HasPrefix(base, "DoNotStarveTogether") {
		return KindWorlds
	}
	switch base {
	case "steamcmd":
		return KindSteamcmd
	case "mod":
		return KindMod
	case "backup":
		return KindBackup
	case "dst", "dst-dedicated-server", "dontstarve_dedicated_server":
		return KindServer
	}
	return ""
}

func unitStatus(u MigrationUnit, parent string, gameRunning bool) string {
	if parent == "" {
		return StatusNotFound
	}
	if !dirExists(u.Source) {
		return StatusNotFound
	}
	if cleanKey(u.Source) == cleanKey(u.Target) {
		return StatusAlreadyInPlace
	}
	if dirNonEmpty(u.Target) {
		return StatusTargetConflict
	}
	if gameRunning && (u.Kind == KindServer || u.Kind == KindWorlds) {
		return StatusGameRunning
	}
	return StatusMigratable
}

// DirSize 求目录大小；topLevelFilesOnly 只算顶层文件
func DirSize(path string, topLevelFilesOnly bool) (int64, error) {
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			if topLevelFilesOnly {
				continue
			}
			sub, err := DirSize(filepath.Join(path, e.Name()), false)
			if err != nil {
				continue
			}
			total += sub
			continue
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dirNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func cleanKey(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func isAncestor(a, b string) bool {
	a = cleanKey(a)
	b = cleanKey(b)
	sep := string(filepath.Separator)
	return strings.HasPrefix(b, a+sep)
}

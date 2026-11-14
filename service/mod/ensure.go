package mod

import (
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

// ClusterModIDs 收集指定存档所有世界 modoverrides 中引用的模组 ID（去重）
func ClusterModIDs(clusterName string) []string {
	re := regexp.MustCompile(`workshop-(\d+)`)
	seen := map[string]bool{}
	var ids []string
	base := filepath.Join(dstUtils.GetKleiDstPath(), clusterName)
	entries, err := os.ReadDir(base)
	if err != nil {
		return ids
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(base, e.Name(), "modoverrides.lua"))
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
	}
	return ids
}

// DownloadModToUnified 用 SteamCMD 下载模组并归位到统一模组目录；已存在则跳过。
// progress 可为 nil。
func DownloadModToUnified(modid string, progress func(string)) error {
	report := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		return errors.New("未配置根目录")
	}
	steamcmdDir := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
	modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
	target := filepath.Join(modsRoot, modid)
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return nil // 已存在
	}
	exe := steamcmdExePath(steamcmdDir)
	if _, err := os.Stat(exe); err != nil {
		return errors.New("未安装 SteamCMD，请先在系统设置使用「一键下载系统组件」")
	}
	report("下载模组 " + modid + " ...")
	cmd := exec.Command(exe, "+login", "anonymous", "+workshop_download_item", "322330", modid, "+quit")
	cmd.Dir = steamcmdDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("模组 " + modid + " 下载失败: " + err.Error())
	}
	_ = out
	src := filepath.Join(steamcmdDir, "steamapps", "workshop", "content", "322330", modid)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return errors.New("模组 " + modid + " 下载后未找到文件")
	}
	if err := os.MkdirAll(modsRoot, os.ModePerm); err != nil {
		return err
	}
	if err := os.Rename(src, target); err != nil {
		return errors.New("模组 " + modid + " 归位失败: " + err.Error())
	}
	return nil
}

// EnsureClusterMods 启动世界前同步下载该存档所需模组（已存在跳过），返回本次下载数量
func EnsureClusterMods(clusterName string, progress func(string)) (int, error) {
	ids := ClusterModIDs(clusterName)
	n := 0
	for _, id := range ids {
		cfg := dstConfigUtils.GetDstConfig()
		modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
		if info, err := os.Stat(filepath.Join(modsRoot, id)); err == nil && info.IsDir() {
			continue // 已存在，无需下载
		}
		if err := DownloadModToUnified(id, progress); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func steamcmdExePath(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "steamcmd.exe")
	}
	if _, err := os.Stat(filepath.Join(dir, "steamcmd.sh")); err == nil {
		return filepath.Join(dir, "steamcmd.sh")
	}
	return filepath.Join(dir, "steamcmd")
}

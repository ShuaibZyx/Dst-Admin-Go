package mod

import (
	"bufio"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
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
	err := RunSteamcmdStream(steamcmdDir, []string{"+login", "anonymous", "+workshop_download_item", "322330", modid, "+quit"}, report)
	if err != nil {
		return errors.New("模组 " + modid + " 下载失败: " + err.Error())
	}
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

// ensureModsMu 串行化：多世界并发启动时避免同一模组重复下载
var ensureModsMu sync.Mutex

// EnsureClusterMods 启动世界前同步下载该存档所需模组（已存在跳过），返回本次下载数量
func EnsureClusterMods(clusterName string, progress func(string)) (int, error) {
	ensureModsMu.Lock()
	defer ensureModsMu.Unlock()
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

// steamcmdMu 串行化 steamcmd 调用：并发运行会互抢 app 缓存导致 Missing configuration 等怪错
var steamcmdMu sync.Mutex

// RunSteamcmdStream 运行 steamcmd 并把输出行实时回调（相同行与高频行节流，关键行必发）
func RunSteamcmdStream(dir string, args []string, send func(string)) error {
	steamcmdMu.Lock()
	defer steamcmdMu.Unlock()
	exe := steamcmdExePath(dir)
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	// steamcmd 退出后其子进程可能仍占着管道导致读取悬挂：进程退出后最多等 3s 即收尾
	cmd.WaitDelay = 3 * time.Second
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return err
	}
	// Wait 在独立 goroutine：进程退出后关闭写端给扫描器 EOF，否则互相等待死锁
	waitCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = pw.Close()
		waitCh <- err
	}()
	last := ""
	lastAt := time.Time{}
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		key := line
		if len(key) > 120 {
			key = key[:120]
		}
		if key == last {
			continue
		}
		isKey := strings.Contains(line, "Success") || strings.Contains(line, "ERROR") ||
			strings.Contains(line, "error") || strings.Contains(line, "失败") || strings.Contains(line, "完成")
		if !isKey && time.Since(lastAt) < 700*time.Millisecond {
			continue
		}
		last = key
		lastAt = time.Now()
		if send != nil {
			send(line)
		}
	}
	return <-waitCh
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

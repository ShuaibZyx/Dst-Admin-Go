package mod

import (
	"bufio"
	"fmt"
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"errors"
	"io"
	"log"
	"net/http"
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

// cleanWorkshopDownloadResidue 清理 SteamCMD 工坊下载残留（中断任务的增量状态文件），避免后续下载异常
func cleanWorkshopDownloadResidue(steamcmdDir, modid string) {
	dl := filepath.Join(steamcmdDir, "steamapps", "workshop", "downloads")
	_ = os.RemoveAll(filepath.Join(dl, "322330", modid))
	_ = os.Remove(filepath.Join(dl, "state_322330_322330_"+modid+".patch"))
}

// invalidateWorkshopAcf 删除 SteamCMD 工坊安装记录（appworkshop_322330.acf）。
// 模组归位是把文件从 steamcmd 的 content 目录"搬走"，acf 会留下"已安装"的假记录；
// 之后其他模组下载若按 acf 复用这些文件块（chunk 去重）会因文件缺失而整体失败
// （workshop_log: Update canceled ... Missing game files）。删掉后 SteamCMD 重扫 content 目录重建记录。
func invalidateWorkshopAcf(steamcmdDir string) {
	_ = os.Remove(filepath.Join(steamcmdDir, "steamapps", "workshop", "appworkshop_322330.acf"))
}

// ensureLegacyBinFromDB manifest=-1 兜底:从库里该模组的 file_url 拉原始 zip,
// 写 <ugchandle>_legacy.bin(游戏 legacy 流程的解包素材)。成功返回 ugchandle,任一步失败返回 ""。
func ensureLegacyBinFromDB(modid, modDir string) string {
	var info model.ModInfo
	if err := database.DB.Where("modid = ?", modid).First(&info).Error; err != nil {
		return ""
	}
	if info.FileUrl == "" {
		return ""
	}
	ugc := UgchandleFromURL(info.FileUrl)
	if ugc == "" {
		return ""
	}
	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(info.FileUrl)
	if err != nil {
		log.Println("[legacy-bin] 下载失败:", modid, err)
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Println("[legacy-bin] 下载状态异常:", modid, res.StatusCode)
		return ""
	}
	zipBytes, err := io.ReadAll(res.Body)
	if err != nil || len(zipBytes) == 0 {
		log.Println("[legacy-bin] 读取失败:", modid, err)
		return ""
	}
	if err := WriteLegacyBin(modDir, ugc, zipBytes); err != nil {
		log.Println("[legacy-bin] 写入失败:", modid, err)
		return ""
	}
	return ugc
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
	if modDirComplete(target) {
		return nil // 已完整存在
	}
	if dirExists(target) {
		// 目录在但缺 modinfo.lua（校验判为损坏）：必须清理后重下，
		// 否则会误判"已存在"跳过下载，校验-修复永远循环报同一批模组
		if rerr := os.RemoveAll(target); rerr != nil {
			return errors.New("模组 " + modid + " 无法清理损坏目录（世界运行中会被占用，请停止后重试）: " + rerr.Error())
		}
	}
	exe := steamcmdExePath(steamcmdDir)
	if _, err := os.Stat(exe); err != nil {
		return errors.New("未安装 SteamCMD，请先在系统设置使用「一键下载系统组件」")
	}
	report("下载模组 " + modid + " ...")
	args := []string{"+login", "anonymous", "+workshop_download_item", "322330", modid, "+quit"}
	src := filepath.Join(steamcmdDir, "steamapps", "workshop", "content", "322330", modid)
	// attempt 执行一次下载。注意 SteamCMD 工坊下载失败时退出码仍为 0（仅打印 ERROR!），
	// 必须以产物是否落盘为准判定成败。
	attempt := func() error {
		cleanWorkshopDownloadResidue(steamcmdDir, modid)
		if serr := RunSteamcmdStream(steamcmdDir, args, report); serr != nil {
			return serr
		}
		if info, serr := os.Stat(src); serr != nil || !info.IsDir() {
			return errors.New("下载后未找到文件")
		}
		return nil
	}
	err := attempt()
	if err != nil {
		report("[提示] 模组 " + modid + " 下载未成功，5 秒后重试一次...")
		time.Sleep(5 * time.Second)
		invalidateWorkshopAcf(steamcmdDir) // stale 假记录会让 chunk 复用引用已搬走的文件而失败，清掉后全量重下
		err = attempt()
	}
	if err != nil {
		return errors.New("模组 " + modid + " 下载失败: " + err.Error())
	}
	if err := os.MkdirAll(modsRoot, os.ModePerm); err != nil {
		return err
	}
	// 归位前从 SteamCMD 的 acf 摘出安装记录(带真实 manifest),归位后登记到游戏侧 acf。
	// 游戏只认游戏侧 acf 的 WorkshopItemsInstalled 登记,缺失会被判"待下载"而不启用。
	scEntry, hasSc := steamcmdInstalledEntry(steamcmdDir, modid)
	if err := os.Rename(src, target); err != nil {
		return errors.New("模组 " + modid + " 归位失败: " + err.Error())
	}
	invalidateWorkshopAcf(steamcmdDir)
	if hasSc && (scEntry.Manifest != "" || scEntry.Ugchandle != "") {
		if scEntry.Size == 0 {
			scEntry.Size = dirSizeOnDisk(target)
		}
		if rerr := RegisterInstalled(modid, scEntry); rerr != nil {
			return errors.New("模组 " + modid + " 游戏侧登记失败: " + rerr.Error())
		}
	} else {
		// SteamCMD 记录缺失:按 manifest=-1 登记游戏会走 legacy 解包流程,必须配 legacy bin,
		// 否则 UMFD 解包失败模组不启用
		item := AcfItem{Size: dirSizeOnDisk(target), Manifest: "-1"}
		if item.Ugchandle = ensureLegacyBinFromDB(modid, target); item.Ugchandle == "" {
			return errors.New("模组 " + modid + " 缺少版本信息且无法补 legacy bin，游戏将无法启用它")
		}
		if rerr := RegisterInstalled(modid, item); rerr != nil {
			return errors.New("模组 " + modid + " 游戏侧登记失败: " + rerr.Error())
		}
	}
	// 收尾验收:ACF 登记有效或 legacy bin 在位,不过则整体报错(下载完成=游戏可用)
	if verr := verifyGameReadyFn(modid, target); verr != nil {
		return errors.New("模组 " + modid + " 下载后验收失败: " + verr.Error())
	}
	return nil
}

// ensureModsMu 串行化：多世界并发启动时避免同一模组重复下载
var ensureModsMu sync.Mutex

// EnsureClusterMods 启动世界前同步下载该存档所需模组（已存在跳过），返回本次下载数量
func EnsureClusterMods(clusterName string, progress func(string)) (int, error) {
	ensureModsMu.Lock()
	defer ensureModsMu.Unlock()
	// 补齐 steamclient 下载所需库布局：缺 libraryfolders.vdf 时游戏内下载必败
	// (Staging library folder not found)，待下载模组永远不会启用
	if err := EnsureSteamLibraryLayout(); err != nil {
		log.Println("[acf] 库布局补齐失败:", err)
	}
	ids := ClusterModIDs(clusterName)
	n := 0
	for _, id := range ids {
		cfg := dstConfigUtils.GetDstConfig()
		modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
		if modDirComplete(filepath.Join(modsRoot, id)) {
			continue // 已完整存在，无需下载
		}
		if err := DownloadModToUnified(id, progress); err != nil {
			return n, err
		}
		n++
	}
	// 自愈:本地已有文件但游戏侧 acf 缺登记(或历史遗留)时补登记,避免游戏判"待下载"而不启用
	if fixed := EnsureGameAcfEntries(); fixed > 0 && progress != nil {
		progress(fmt.Sprintf("已补登记 %d 个模组的安装记录", fixed))
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

// DownloadMods 下载/校验模组集合：ids 为空=当前存档全部；force 先删本地副本强制重下
func DownloadMods(clusterName string, ids []string, force bool, progress func(string)) (done int, failed int, err error) {
	if len(ids) == 0 {
		ids = ClusterModIDs(clusterName)
	}
	for _, id := range ids {
		if force {
			cfg := dstConfigUtils.GetDstConfig()
			modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
			if rerr := os.RemoveAll(filepath.Join(modsRoot, id)); rerr != nil {
				failed++
				if progress != nil {
					progress("[失败] 模组 " + id + " 无法清理旧文件（世界运行中会被占用，请停止后重试）")
				}
				continue
			}
		}
		if derr := DownloadModToUnified(id, progress); derr != nil {
			failed++
			if progress != nil {
				progress("[失败] " + derr.Error())
			}
			continue
		}
		done++
	}
	return done, failed, nil
}

// SyncMods 一键模组维护：mode="repair" 校验并修复问题项；mode="redownload" 全部强制重下。
// 两种模式结尾都刷新模组元数据。返回摘要行。
func SyncMods(clusterName, mode string, progress func(string)) (string, error) {
	report := func(format string, a ...interface{}) {
		if progress != nil {
			progress(fmt.Sprintf(format, a...))
		}
	}
	ids := ClusterModIDs(clusterName)
	if len(ids) == 0 {
		return "", fmt.Errorf("未在存档中找到任何模组登记（modoverrides），无法执行")
	}
	// 与 EnsureClusterMods 一致:先补 steamclient 库布局,缺 libraryfolders.vdf 时游戏内下载必败
	if err := EnsureSteamLibraryLayout(); err != nil {
		log.Println("[acf] 库布局补齐失败:", err)
	}
	if mode == "redownload" {
		report("执行模式：全部重新下载（先删后下），共 %d 个模组", len(ids))
	} else {
		report("执行模式：校验更新，共 %d 个模组", len(ids))
	}
	statuses := VerifyClusterMods(clusterName)
	ok, missing, broken := 0, 0, 0
	var needFix []string
	for _, st := range statuses {
		switch st.Status {
		case "ok":
			ok++
		case "missing":
			missing++
			needFix = append(needFix, st.Id)
		case "broken":
			broken++
			needFix = append(needFix, st.Id)
		}
	}
	report("校验结果：完整 %d，缺失 %d，损坏 %d", ok, missing, broken)

	target := needFix
	force := false
	if mode == "redownload" {
		target = ids
		force = true
		report("全部重新下载 %d 个模组（需先停止所有世界，否则文件被占用会跳过）...", len(target))
	} else if len(target) == 0 {
		report("校验完成：%d 个模组全部完整，无需下载", ok)
		report("刷新模组信息...")
		UpdateModinfoList("zh")
		return fmt.Sprintf("完成：校验 %d 个模组全部完整，成功 0，失败 0", len(statuses)), nil
	} else {
		report("修复 %d 个问题模组...", len(target))
	}
	n, failed, err := DownloadMods(clusterName, target, force, progress)
	if err != nil {
		return "", err
	}
	if failed > 0 {
		report("下载结束：成功 %d，失败 %d（失败多因世界运行中文件被占用）", n, failed)
	} else {
		report("模组下载完成，共处理 %d 个", n)
	}

	report("刷新模组信息...")
	UpdateModinfoList("zh")
	report("模组信息已更新")
	if failed > 0 {
		return fmt.Sprintf("失败：校验 %d，成功 %d，失败 %d", len(statuses), n, failed), nil
	}
	return fmt.Sprintf("完成：校验 %d，成功 %d，失败 0", len(statuses), n), nil
}

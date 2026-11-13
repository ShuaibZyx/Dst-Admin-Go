package migration

import (
	"dst-admin-go/utils/dstConfigUtils"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// MigrateResult 单个迁移单元的执行结果
type MigrateResult struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Migrate 只执行 selectedIDs 中的单元；未选中的一字节不动（§11）
func Migrate(units []MigrationUnit, selectedIDs []string, gameRunning bool) []MigrateResult {
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	var results []MigrateResult
	matched := map[string]bool{}
	for _, u := range units {
		if !selected[u.ID] {
			continue
		}
		matched[u.ID] = true
		results = append(results, migrateOne(u, gameRunning))
	}
	// 选中但不存在的 id 也回结果，便于前端提示
	for _, id := range selectedIDs {
		if !matched[id] {
			results = append(results, MigrateResult{ID: id, OK: false, Message: "未知单元"})
		}
	}
	return results
}

func migrateOne(u MigrationUnit, gameRunning bool) MigrateResult {
	if gameRunning && (u.Kind == KindServer || u.Kind == KindWorlds) {
		return MigrateResult{ID: u.ID, OK: false, Message: "游戏运行中"}
	}
	switch u.Status {
	case StatusTargetConflict:
		return MigrateResult{ID: u.ID, OK: false, Message: "目标位置已存在且非空"}
	case StatusAlreadyInPlace:
		return MigrateResult{ID: u.ID, OK: false, Message: "已在目标位置"}
	case StatusNotFound:
		return MigrateResult{ID: u.ID, OK: false, Message: "源不存在"}
	case StatusGameRunning:
		return MigrateResult{ID: u.ID, OK: false, Message: "游戏运行中"}
	case StatusMigratable:
		// 继续
	default:
		return MigrateResult{ID: u.ID, OK: false, Message: "状态不可迁移: " + u.Status}
	}

	var err error
	if u.TopLevelFilesOnly {
		err = moveTopLevelFiles(u.Source, u.Target)
	} else {
		err = moveDir(u.Source, u.Target)
	}
	if err != nil {
		return MigrateResult{ID: u.ID, OK: false, Message: err.Error()}
	}
	return MigrateResult{ID: u.ID, OK: true, Message: "完成"}
}

// moveDir 移动整个目录：优先 rename；跨卷则复制-校验-源改名保留（§11，绝不硬删源）
func moveDir(src, dst string) error {
	if dirNonEmpty(dst) {
		return errors.New("目标已存在且非空")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// 跨卷：复制 → 校验 → 源改名保留
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	if err := verifyDirEqual(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	bak := src + ".pre-migrate-" + time.Now().Format("20060102-150405")
	if err := os.Rename(src, bak); err != nil {
		// 校验已通过、目标完好；源留在原地可人工清理，不算失败
		return nil
	}
	return nil
}

// moveTopLevelFiles 只搬 Source 顶层文件，目录保留（共享源拆分，§11.5）
func moveTopLevelFiles(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		if err := os.Rename(from, to); err == nil {
			continue
		}
		// 跨卷：复制+校验大小后删源文件（唯一允许删源的场景）
		if err := copyFile(from, to); err != nil {
			return err
		}
		s1, err1 := fileSize(from)
		s2, err2 := fileSize(to)
		if err1 != nil || err2 != nil || s1 != s2 {
			os.Remove(to)
			return errors.New("文件校验失败: " + from)
		}
		if err := os.Remove(from); err != nil {
			return err
		}
	}
	return nil
}

// verifyDirEqual 校验两树的文件数与总字节一致
func verifyDirEqual(src, dst string) error {
	sf, sb, err := dirStat(src)
	if err != nil {
		return err
	}
	df, db, err := dirStat(dst)
	if err != nil {
		return err
	}
	if sf != df || sb != db {
		return fmt.Errorf("复制校验失败: 源 %d 文件/%d 字节, 目标 %d 文件/%d 字节", sf, sb, df, db)
	}
	return nil
}

func dirStat(path string) (files int, bytes int64, err error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			f, b, err := dirStat(filepath.Join(path, e.Name()))
			if err != nil {
				return 0, 0, err
			}
			files += f
			bytes += b
			continue
		}
		files++
		if info, err := e.Info(); err == nil {
			bytes += info.Size()
		}
	}
	return files, bytes, nil
}

func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(to, 0755); err != nil {
				return err
			}
			if err := copyTree(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// RewriteConfigAfterMigration 成功 kind 改写为相对默认值（§11.4）；未出现的 kind 不动
func RewriteConfigAfterMigration(cfg *dstConfigUtils.DstConfig, succeeded []MigrationUnit) {
	kinds := map[string]bool{}
	for _, u := range succeeded {
		kinds[u.Kind] = true
	}
	if kinds[KindSteamcmd] {
		cfg.Steamcmd = dstConfigUtils.DefaultSteamcmd
	}
	if kinds[KindServer] {
		cfg.Force_install_dir = dstConfigUtils.DefaultServerDir
	}
	if kinds[KindMod] {
		cfg.Mod_download_path = dstConfigUtils.DefaultMod
	}
	if kinds[KindBackup] {
		cfg.Backup = dstConfigUtils.DefaultBackup
	}
	if kinds[KindWorlds] {
		cfg.Persistent_storage_root = dstConfigUtils.DefaultWorlds
		cfg.Conf_dir = confDefault(cfg.Beta)
	}
}

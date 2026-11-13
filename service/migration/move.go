package migration

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// MoveDir 移动整个目录：优先 rename；跨卷则复制-校验-源改名保留（绝不硬删源）
func MoveDir(src, dst string) error {
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
	_ = os.Rename(src, bak) // 失败则源留在原地，不算导入失败
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

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dirNonEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

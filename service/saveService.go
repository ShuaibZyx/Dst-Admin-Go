package service

import (
	"fmt"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/systemUtils"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// SaveInfo 一个存档（cluster）
type SaveInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Current bool   `json:"current"` // 是否为当前部署的存档
	InPlace bool   `json:"inPlace"` // 是否已在存档目录内
	Running  bool   `json:"running"`  // 任一世界是否运行中
	Conflict bool   `json:"conflict"` // 存档目录内已有同名存档（搬入会冲突）
}

// SaveMoveResult 搬家单项结果
type SaveMoveResult struct {
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// saveScanRoots 可注入的检测扫描根
var saveScanRoots = defaultSaveScanRoots

func defaultSaveScanRoots() []string {
	home, err := systemUtils.Home()
	if err != nil {
		return nil
	}
	var roots []string
	if runtime.GOOS == "windows" {
		roots = append(roots, filepath.Join(home, "Documents", "klei"))
	} else {
		roots = append(roots, filepath.Join(home, ".klei"))
	}
	if cfg := dstConfigUtils.GetDstConfig(); cfg.Last_parent_path != "" {
		roots = append(roots, filepath.Join(cfg.Last_parent_path, "world"))
	}
	return roots
}

func isClusterDir(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == "cluster.ini" && !e.IsDir() {
			return true
		}
		if e.IsDir() && (e.Name() == "Master" || e.Name() == "Caves") {
			return true
		}
	}
	return false
}

func collectClusterDirs(root string) []string {
	var out []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(root, e.Name())
		if isClusterDir(sub) {
			out = append(out, sub)
			continue
		}
		// Klei 布局：root/DoNotStarveTogether*/<cluster>
		if strings.HasPrefix(e.Name(), "DoNotStarveTogether") {
			out = append(out, collectClusterDirs(sub)...)
		}
	}
	return out
}

// ListSaves 列出当前存档目录下的存档
func ListSaves() []SaveInfo {
	base := dstUtils.GetKleiDstPath()
	current := dstConfigUtils.GetDstConfig().Cluster
	var out []SaveInfo
	for _, p := range collectClusterDirs(base) {
		name := filepath.Base(p)
		out = append(out, SaveInfo{Name: name, Path: p, Current: name == current, InPlace: true, Running: isSaveRunning(name)})
	}
	return out
}

// DetectSaves 检测已知位置里现有的存档（已在存档目录内的不返回——无需搬入）
func DetectSaves() []SaveInfo {
	base := dstUtils.GetKleiDstPath()
	current := dstConfigUtils.GetDstConfig().Cluster
	seen := map[string]bool{}
	var out []SaveInfo
	for _, root := range append([]string{base}, saveScanRoots()...) {
		for _, p := range collectClusterDirs(root) {
			if isUnder(base, p) {
				continue
			}
			key := strings.ToLower(filepath.Clean(p))
			if runtime.GOOS != "windows" {
				key = filepath.Clean(p)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			name := filepath.Base(p)
			out = append(out, SaveInfo{
				Name:     name,
				Path:     p,
				Current:  name == current,
				Conflict: fileUtils.Exists(filepath.Join(base, name)),
			})
		}
	}
	return out
}

func isUnder(base, p string) bool {
	canon := func(s string) string {
		if abs, err := filepath.Abs(s); err == nil {
			s = abs
		}
		s = filepath.ToSlash(filepath.Clean(s))
		if runtime.GOOS == "windows" {
			s = strings.ToLower(s)
		}
		return s
	}
	b := canon(base)
	p = canon(p)
	return p == b || strings.HasPrefix(p, b+"/")
}

// MoveSaves 把外部存档搬进当前存档目录（同名冲突拒绝该项；跨卷复制校验后源改名保留）
func MoveSaves(sources []string) []SaveMoveResult {
	base := dstUtils.GetKleiDstPath()
	var results []SaveMoveResult
	for _, src := range sources {
		src = filepath.Clean(src)
		if !isClusterDir(src) {
			results = append(results, SaveMoveResult{Path: src, OK: false, Message: "不是存档目录"})
			continue
		}
		target, err := fileUtils.SafeJoin(base, filepath.Base(src))
		if err != nil {
			results = append(results, SaveMoveResult{Path: src, OK: false, Message: err.Error()})
			continue
		}
		if fileUtils.Exists(target) {
			results = append(results, SaveMoveResult{Path: src, OK: false, Message: fmt.Sprintf("命名冲突：已存在同名存档「%s」，不可搬入", filepath.Base(src))})
			continue
		}
		if err := MoveDir(src, target); err != nil {
			results = append(results, SaveMoveResult{Path: src, OK: false, Message: err.Error()})
			continue
		}
		results = append(results, SaveMoveResult{Path: src, OK: true, Message: "完成"})
	}
	return results
}

// MoveDir 移动目录：优先 rename；跨卷复制-校验-源改名保留（绝不硬删源）
func MoveDir(src, dst string) error {
	if fileUtils.Exists(dst) {
		return errors.New("目标已存在")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyTreeDir(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	if err := verifyTreeEqual(src, dst); err != nil {
		os.RemoveAll(dst)
		return err
	}
	_ = os.Rename(src, src+".pre-migrate-"+time.Now().Format("20060102-150405"))
	return nil
}

func verifyTreeEqual(a, b string) error {
	fa, ba, err := treeStat(a)
	if err != nil {
		return err
	}
	fb, bb, err := treeStat(b)
	if err != nil {
		return err
	}
	if fa != fb || ba != bb {
		return errors.New("复制校验失败")
	}
	return nil
}

func treeStat(root string) (files int, bytes int64, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			f, b, err := treeStat(filepath.Join(root, e.Name()))
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

// isSaveRunning 存档任一世界（shard）是否在运行
func isSaveRunning(clusterName string) bool {
	base := filepath.Join(dstUtils.GetKleiDstPath(), clusterName)
	entries, err := os.ReadDir(base)
	if err != nil {
		return false
	}
	var g GameService
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !fileUtils.Exists(filepath.Join(base, e.Name(), "server.ini")) {
			continue
		}
		if g.GetLevelStatus(clusterName, e.Name()) {
			return true
		}
	}
	return false
}

// DeleteSave 删除存档目录：运行中或非法名拒绝（破坏性操作，调用方需先经用户确认）
func DeleteSave(name string) error {
	if name == "" || !fileUtils.ValidName(name) {
		return errors.New("非法存档名")
	}
	if isSaveRunning(name) {
		return errors.New("存档正在运行，请先停止世界再删除")
	}
	base := filepath.Join(dstUtils.GetKleiDstPath(), name)
	if !dirExists(base) {
		return errors.New("存档不存在")
	}
	return fileUtils.DeleteDir(base)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

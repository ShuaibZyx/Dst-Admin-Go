package api

import (
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/systemUtils"
	"dst-admin-go/vo"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
)

// FsApi 服务端目录浏览（根目录选择弹窗用）
type FsApi struct{}

// 快捷位置哨兵路径（前端"位置"按钮）
const (
	RootsSentinel = "__roots__" // 文件系统：Windows=盘符列表；Unix=/ 顶层目录
	AppSentinel   = "__app__"   // 面板目录：当前工作目录
)

type dirItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListDirs 列出 path 下的子目录。
// path 为空 → 用户目录；__roots__ → 文件系统视图；__app__ → 面板目录。
func (f *FsApi) ListDirs(ctx *gin.Context) {
	path := ctx.Query("path")

	switch path {
	case RootsSentinel:
		ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{
			"path": RootsSentinel, "parent": RootsSentinel, "dirs": listRoots(),
		}})
		return
	case AppSentinel:
		cwd, err := os.Getwd()
		if err != nil {
			ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
			return
		}
		path = cwd
	case "":
		home, err := systemUtils.Home()
		if err != nil {
			ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
			return
		}
		path = home
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 404, Msg: err.Error()})
		return
	}
	var dirs []dirItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirs = append(dirs, dirItem{Name: e.Name(), Path: filepath.Join(path, e.Name())})
	}
	if dirs == nil {
		dirs = []dirItem{}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: gin.H{"path": path, "parent": filepath.Dir(path), "dirs": dirs},
	})
}

// listRoots 文件系统视图：Windows=盘符；Unix=/ 的顶层目录
func listRoots() []dirItem {
	var roots []dirItem
	if runtime.GOOS == "windows" {
		for c := 'A'; c <= 'Z'; c++ {
			root := string(c) + `:\`
			if info, err := os.Stat(root); err == nil && info.IsDir() {
				roots = append(roots, dirItem{Name: root, Path: root})
			}
		}
	} else {
		entries, err := os.ReadDir("/")
		if err != nil {
			return []dirItem{}
		}
		for _, e := range entries {
			if e.IsDir() {
				roots = append(roots, dirItem{Name: e.Name(), Path: "/" + e.Name()})
			}
		}
	}
	if roots == nil {
		roots = []dirItem{}
	}
	return roots
}

// Mkdir 递归创建目录（选择根目录时目标不存在的场景，前端弹窗确认后调用）
func (f *FsApi) Mkdir(ctx *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := ctx.Bind(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "path required"})
		return
	}
	if err := os.MkdirAll(req.Path, os.ModePerm); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"path": req.Path}})
}

// resolveOpenEntry 把条目参数解析为根内绝对路径（绝对路径或根下相对路径）。
// 越出 root（含 ..、兄弟目录前缀混淆、不同卷）或指向根本身均视为非法。
func resolveOpenEntry(root, p string) (string, error) {
	target := p
	if !filepath.IsAbs(p) {
		target = filepath.Join(root, p)
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(filepath.Clean(root), target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path outside root")
	}
	return target, nil
}

// openTarget 决定文件管理器打开位置：目录条目（存档）直接进入其内部；
// 文件条目（备份 zip）定位到其所在目录并选中。
func openTarget(entry string) (dir, selectPath string) {
	if info, err := os.Stat(entry); err == nil && info.IsDir() {
		return entry, ""
	}
	return filepath.Dir(entry), entry
}

// Open 在系统文件管理器中打开「文件」（存档管理/备份列表条目级按钮）。
// 只认 kind 白名单（save=存档根、backup=集群备份目录），不接受任意路径；
// path 可选=白名单根内的单个条目：存档（目录）进入其内部，备份（文件）打开所在目录并选中；
// 缺省则打开整个根目录。
func (f *FsApi) Open(ctx *gin.Context) {
	var root string
	switch ctx.Query("kind") {
	case "save":
		root = dstUtils.GetKleiDstPath()
	case "backup":
		root = clusterUtils.GetClusterFromGin(ctx).Backup
	default:
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "unsupported kind"})
		return
	}
	if root == "" || !fileUtils.Exists(root) {
		ctx.JSON(http.StatusOK, vo.Response{Code: 404, Msg: "dir not found"})
		return
	}

	dir := root
	target := ""
	if p := ctx.Query("path"); p != "" {
		entry, err := resolveOpenEntry(root, p)
		if err != nil {
			ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: err.Error()})
			return
		}
		if !fileUtils.Exists(entry) {
			ctx.JSON(http.StatusOK, vo.Response{Code: 404, Msg: "entry not found"})
			return
		}
		dir, target = openTarget(entry)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if target != "" {
			// explorer /select,<path> 打开所在目录并选中条目
			cmd = exec.Command("explorer", "/select,"+target)
		} else {
			cmd = exec.Command("explorer", dir)
		}
	case "darwin":
		if target != "" {
			cmd = exec.Command("open", "-R", target)
		} else {
			cmd = exec.Command("open", dir)
		}
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"path": dir}})
}

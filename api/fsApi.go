package api

import (
	"dst-admin-go/utils/systemUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"
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

package api

import (
	"dst-admin-go/utils/systemUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
)

// FsApi 服务端目录浏览（根目录选择弹窗用）
type FsApi struct{}

// ComputerSentinel 此电脑视图：列出各盘符根
const ComputerSentinel = "__computer__"

// ListDirs 列出 path 下的子目录；path 为空时从用户主目录开始；__computer__ 列盘符
func (f *FsApi) ListDirs(ctx *gin.Context) {
	path := ctx.Query("path")
	type dirItem struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if path == ComputerSentinel {
		var drives []dirItem
		if runtime.GOOS == "windows" {
			for c := 'A'; c <= 'Z'; c++ {
				root := string(c) + `:\`
				if info, err := os.Stat(root); err == nil && info.IsDir() {
					drives = append(drives, dirItem{Name: root, Path: root})
				}
			}
		} else {
			for _, root := range []string{"/", "/app", "/home", "/mnt", "/media", "/opt", "/root"} {
				if info, err := os.Stat(root); err == nil && info.IsDir() {
					drives = append(drives, dirItem{Name: root, Path: root})
				}
			}
		}
		if drives == nil {
			drives = []dirItem{}
		}
		ctx.JSON(http.StatusOK, vo.Response{
			Code: 200,
			Msg:  "success",
			Data: gin.H{"path": ComputerSentinel, "parent": ComputerSentinel, "dirs": drives},
		})
		return
	}
	if path == "" {
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

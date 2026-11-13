package api

import (
	"dst-admin-go/utils/systemUtils"
	"dst-admin-go/vo"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// FsApi 服务端目录浏览（父级文件夹选择弹窗用）
type FsApi struct{}

// ListDirs 列出 path 下的子目录；path 为空时从用户主目录开始
func (f *FsApi) ListDirs(ctx *gin.Context) {
	path := ctx.Query("path")
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
	type dirItem struct {
		Name string `json:"name"`
		Path string `json:"path"`
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

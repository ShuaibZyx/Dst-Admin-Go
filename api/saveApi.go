package api

import (
	"dst-admin-go/service"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SaveApi 存档管理（显示/上传/搬家同步）
type SaveApi struct{}

// ListSaves 列出存档目录内的存档
func (s *SaveApi) ListSaves(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"saves": service.ListSaves()}})
}

// DetectSaves 检测已知位置里现有的存档
func (s *SaveApi) DetectSaves(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"saves": service.DetectSaves()}})
}

// UploadFolder 上传存档文件夹（字段名 files/<相对路径>）
func (s *SaveApi) UploadFolder(ctx *gin.Context) {
	if err := ctx.Request.ParseMultipartForm(512 << 20); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	name, err := service.AssembleSaveFolder(ctx.Request.MultipartForm, dstUtils.GetKleiDstPath())
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"name": name}})
}

// Move 把外部存档搬进存档目录
func (s *SaveApi) Move(ctx *gin.Context) {
	var req struct {
		Sources []string `json:"sources"`
	}
	if err := ctx.Bind(&req); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: gin.H{"results": service.MoveSaves(req.Sources)}})
}

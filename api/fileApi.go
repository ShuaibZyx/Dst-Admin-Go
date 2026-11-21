package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/pathUtils"
	"dst-admin-go/vo"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

type FileApi struct {
}

func (d *FileApi) UploadUgcMods(ctx *gin.Context) {
	form, err := ctx.MultipartForm()
	if err != nil {
		abort500f(ctx, err)
		return
	}
	files := form.File["files"]
	filePaths := form.Value["filePaths"]

	dstConfig := dstConfigUtils.GetDstConfig()
	ugcModPath := dstUtils.GetUgcWorkshopModPath(dstConfig.Cluster, "Master", "")

	log.Println("上传ugc模组路径 ", ugcModPath)
	if len(files) != len(filePaths) {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "files/filePaths length mismatch", Data: nil})
		return
	}
	for i, file := range files {
		if strings.Contains(filePaths[i], "..") || strings.Contains(file.Filename, "..") || strings.ContainsAny(file.Filename, "/\\") {
			ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: "illegal path", Data: nil})
			return
		}
		dir := filepath.Join(ugcModPath, filepath.Dir(filePaths[i]))
		fileUtils.CreateDirIfNotExists(dir)
		p := filepath.Join(ugcModPath, filepath.Dir(filePaths[i]), file.Filename)
		log.Println(p)
		if err := ctx.SaveUploadedFile(file, p); err != nil {
			abort500f(ctx, err)
			return
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (d *FileApi) GetBackground(ctx *gin.Context) {
	// 用户上传的背景在数据根;没传过则回退资源根随包的默认图
	filePath := pathUtils.Data("background.png")
	if !fileUtils.Exists(filePath) {
		filePath = pathUtils.Res("dist", "assets", "background.png")
	}
	// 返回文件
	ctx.File(filePath)
}

func (d *FileApi) UploadBackground(ctx *gin.Context) {
	form, err := ctx.MultipartForm()
	if err != nil {
		abort500f(ctx, err)
		return
	}
	files := form.File["file"]
	for _, file := range files {
		p := pathUtils.Data("background.png") // 用户数据落数据根(exe 只读也能传)
		log.Println(p)
		if err := ctx.SaveUploadedFile(file, p); err != nil {
			abort500f(ctx, err)
			return
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

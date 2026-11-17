package api

import (
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
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
	// 文件路径
	filePath := "./dist/assets/background.png"
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
	dir := "./dist/assets"
	fileUtils.CreateDirIfNotExists(dir)
	for _, file := range files {
		p := filepath.Join("./dist/assets/background.png")
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

package api

import (
	"dst-admin-go/service"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"net/http"

	"github.com/gin-gonic/gin"
)

type InitApi struct {
}

const first = "./first"

func (f *InitApi) InitFirst(ctx *gin.Context) {

	exist := fileUtils.Exists(first)
	if exist {
		abort500f(ctx, "非法请求")
		return
	}

	initData := &service.InitDstData{}
	err := ctx.ShouldBind(initData)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	initEvnService.InitDstEnv(initData, ctx)

	fileUtils.CreateFile(first)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (f *InitApi) CheckIsFirst(ctx *gin.Context) {

	exist := fileUtils.Exists(first)

	code := 200
	msg := "is first"
	if exist {
		code = 400
		msg = "is not first"
	} else {
		dstConfig := dstConfigUtils.GetDstConfig()
		if dstConfig.Cluster != "" {
			initEvnService.InitBaseLevel(&dstConfig, "默认初始", "", true) // A5: 不预置 token，由用户填写
		}

	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

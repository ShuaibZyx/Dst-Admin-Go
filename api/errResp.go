package api

import (
	"fmt"
	"net/http"

	"dst-admin-go/vo"

	"github.com/gin-gonic/gin"
)

// abort500 统一业务错误响应（替代 panic→recover 链路；HTTP 200 + 业务码 500，与项目惯例一致）。
func abort500(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: msg})
}

// abort500f 格式化错误信息后统一响应。
func abort500f(ctx *gin.Context, args ...interface{}) {
	abort500(ctx, fmt.Sprint(args...))
}

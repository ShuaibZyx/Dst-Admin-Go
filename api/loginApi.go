package api

import (
	"dst-admin-go/constant/consts"
	"dst-admin-go/service"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

type LoginApi struct {
}

var loginService = service.LoginService{}

func (l *LoginApi) GetUserInfo(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "Init user success",
		Data: loginService.GetUserInfo(),
	})
}

func (l *LoginApi) Login(ctx *gin.Context) {

	userVO := vo.NewUserVO()
	err := ctx.ShouldBind(userVO)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	response := loginService.Login(userVO, ctx)
	ctx.JSON(http.StatusOK, response)
}

func (l *LoginApi) Logout(ctx *gin.Context) {
	loginService.Logout(ctx)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "Logout success",
	})
}

func (l *LoginApi) ChangePassword(ctx *gin.Context) {

	var body struct {
		NewPassword string `json:"newPassword"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		return
	}
	newPassword := body.NewPassword
	response := loginService.ChangePassword(newPassword)

	ctx.JSON(http.StatusOK, response)
}

// readUserInfo 读 password.txt 的 4 字段（部分更新的基底）
func readUserInfo() service.UserInfo {
	lines, err := fileUtils.ReadLnFile(consts.PasswordPath())
	if err != nil {
		return service.UserInfo{}
	}
	get := func(idx int) string {
		if idx >= len(lines) {
			return ""
		}
		parts := strings.SplitN(lines[idx], "=", 2)
		if len(parts) < 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}
	return service.UserInfo{
		Username:    get(0),
		Password:    get(1),
		DisplayName: get(2),
		PhotoURL:    get(3),
	}
}

func (l *LoginApi) UpdateUserInfo(ctx *gin.Context) {

	var body struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		PhotoURL    string `json:"photoURL"`
		Password    string `json:"password"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		abort500f(ctx, "参数解析错误: "+err.Error())
		return
		return
	}
	// 部分更新：空字段保留旧值（资料与密码分开保存，互不覆盖）
	old := readUserInfo()
	merged := service.MergeUserInfo(old, service.UserInfo{
		Username:    body.Username,
		DisplayName: body.DisplayName,
		PhotoURL:    body.PhotoURL,
		Password:    body.Password,
	})
	err := fileUtils.WriterLnFile(consts.PasswordPath(), []string{
		"username = " + merged.Username,
		"password = " + merged.Password,
		"displayName=" + merged.DisplayName,
		"photoURL=" + merged.PhotoURL,
	})
	if err != nil {
		abort500f(ctx, "修改用户信息失败: "+err.Error())
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "Logout success",
		Data: nil,
	})
}

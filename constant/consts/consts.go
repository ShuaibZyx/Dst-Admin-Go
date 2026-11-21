package consts

import (
	"dst-admin-go/utils/pathUtils"
	"dst-admin-go/utils/systemUtils"
	"fmt"
	"runtime"
)

const (

	// ClearScreenCmd 检查目前所有的screen作业，并删除已经无法使用的screen作业
	ClearScreenCmd = "screen -wipe "

	MasterRunning     = "masterRunning"
	CavesRunning      = "cavesRunning"
	UpdateGameVersion = "updateGameVersion"
	UpdateGameMod     = "updateGameMod"
	UpdateMasterMod   = "updateMasterMod"
	UpdateCavesMod    = "updateCavesMod"

	UPDATE_GAME = "UPDATE_GAME"
	LEVEL_MOD   = "LEVEL_MOD"
	LEVEL_DOWN  = "LEVEL_DOWN"
)

var HomePath string

// PasswordPath 口令文件位于数据根(惰性解析,数据根依赖运行期 CWD 判定)
func PasswordPath() string { return pathUtils.Data("password.txt") }

func init() {
	home, err := systemUtils.Home()
	if err != nil {
		panic("Home path error: " + err.Error())
	}
	HomePath = home
	fmt.Println("home path: " + HomePath)

	if runtime.GOOS == "windows" {
		//klei_path = filepath.Join(consts.HomePath, "Documents", "klei", "DoNotStarveTogether")
		// DefaultKleiDstPath = filepath.Join(home, "Documents", "klei", "DoNotStarveTogether")
	} else {
		// klei_path = filepath.Join(consts.HomePath, ".klei", "DoNotStarveTogether")
		// DefaultKleiDstPath = filepath.Join(home, ".klei", "DoNotStarveTogether")
	}

}

// 启动二进制形态（dst_config 的 bin 值）
const (
	BinX64    = 64   // bin64 下的 x64 原生二进制
	BinLuaJIT = 100  // x64 LuaJIT 构建
	BinBox86  = 86   // box86 模拟 x64（32 位主机）
	BinBox64  = 2664 // box64 模拟 x64（ARM 主机）
)

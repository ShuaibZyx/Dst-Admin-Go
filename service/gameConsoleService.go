package service

import (
	"dst-admin-go/constant/screenKey"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/utils/shellUtils"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

type GameConsoleService struct {
	GameService
}

func (c *GameConsoleService) ClearScreen() bool {
	if isWindows() {
		return true
	}
	result, err := shellUtils.Shell("screen -wipe")
	if err != nil {
		return false
	}
	res := strings.Split(result, "\n")[0]
	return res != ""
}

func (c *GameConsoleService) SentBroadcast2(clusterName string, levelName string, message string) {
	if isWindows() {
		WindowGameConsoleService.SentBroadcast2(clusterName, levelName, message)
		return
	}
	if c.GetLevelStatus(clusterName, levelName) {
		broadcast := screenStuffCmd(clusterName, levelName, "c_announce(\""+shellUtils.EscapeLuaStr(message)+"\")")
		log.Println(broadcast)
		shellUtils.Shell(broadcast)
	}

}

func (c *GameConsoleService) SentBroadcast(clusterName string, message string) {
	if isWindows() {
		WindowGameConsoleService.SentBroadcast(clusterName, message)
		return
	}
	if c.GetLevelStatus(clusterName, "Master") {
		broadcast := screenStuffCmd(clusterName, "Master", "c_announce(\""+shellUtils.EscapeLuaStr(message)+"\")")
		log.Println(broadcast)
		shellUtils.Shell(broadcast)
	}

	if c.GetLevelStatus(clusterName, "Caves") {
		broadcast2 := screenStuffCmd(clusterName, "Caves", "c_announce(\""+shellUtils.EscapeLuaStr(message)+"\")")
		log.Println(broadcast2)
		shellUtils.Shell(broadcast2)
	}

}

func (c *GameConsoleService) KickPlayer(clusterName, KuId string) {
	if isWindows() {
		WindowGameConsoleService.KickPlayer(clusterName, KuId)
		return
	}
	masterCMD := screenStuffCmd(clusterName, "Master", "TheNet:Kick(\""+shellUtils.EscapeLuaStr(KuId)+"\")")
	cavesCMD := screenStuffCmd(clusterName, "Caves", "TheNet:Kick(\""+shellUtils.EscapeLuaStr(KuId)+"\")")

	shellUtils.Shell(masterCMD)
	shellUtils.Shell(cavesCMD)
}

func (c *GameConsoleService) KillPlayer(clusterName, KuId string) {

	if isWindows() {
		WindowGameConsoleService.KillPlayer(clusterName, KuId)
		return
	}

	masterCMD := screenStuffCmd(clusterName, "Master", "UserToPlayer(\""+shellUtils.EscapeLuaStr(KuId)+"\"):PushEvent('death')")
	cavesCMD := screenStuffCmd(clusterName, "Caves", "UserToPlayer(\""+shellUtils.EscapeLuaStr(KuId)+"\"):PushEvent('death')")

	shellUtils.Shell(masterCMD)
	shellUtils.Shell(cavesCMD)
}

func (c *GameConsoleService) RespawnPlayer(clusterName string, KuId string) {

	if isWindows() {
		WindowGameConsoleService.RespawnPlayer(clusterName, KuId)
		return
	}

	masterCMD := screenStuffCmd(clusterName, "Master", "UserToPlayer(\""+shellUtils.EscapeLuaStr(KuId)+"\"):PushEvent('respawnfromghost')")
	cavesCMD := screenStuffCmd(clusterName, "Caves", "UserToPlayer(\""+shellUtils.EscapeLuaStr(KuId)+"\"):PushEvent('respawnfromghost')")

	shellUtils.Shell(masterCMD)
	shellUtils.Shell(cavesCMD)
}

func (c *GameConsoleService) RollBack(clusterName string, dayNum int) {

	if isWindows() {
		WindowGameConsoleService.RollBack(clusterName, dayNum)
		return
	}

	days := fmt.Sprint(dayNum)
	c.SentBroadcast(clusterName, ":pig 正在回档"+days+"天")

	masterCMD := screenStuffCmd(clusterName, "Master", "c_rollback("+days+")")
	cavesCMD := screenStuffCmd(clusterName, "Caves", "c_rollback("+days+")")

	shellUtils.Shell(masterCMD)
	shellUtils.Shell(cavesCMD)
}

func (c *GameConsoleService) CleanWorld(clusterName string) {

	basePath := dstUtils.GetClusterBasePath(clusterName)

	// filepath 而非 path:path 恒用 / 拼接,Windows 下与系统路径混用会出边界情况
	fileUtils.DeleteDir(filepath.Join(basePath, "Master", "backup"))
	fileUtils.DeleteDir(filepath.Join(basePath, "Master", "save"))

	fileUtils.DeleteDir(filepath.Join(basePath, "Caves", "backup"))
	fileUtils.DeleteDir(filepath.Join(basePath, "Caves", "save"))
}

func (c *GameConsoleService) Regenerateworld(clusterName string) {

	if isWindows() {
		WindowGameConsoleService.Regenerateworld(clusterName)
		return
	}

	c.SentBroadcast(clusterName, ":pig 即将重置世界！！！")

	masterCMD := screenStuffCmd(clusterName, "Master", "c_regenerateworld()")
	cavesCMD := screenStuffCmd(clusterName, "Caves", "c_regenerateworld()")
	shellUtils.Shell(masterCMD)
	shellUtils.Shell(cavesCMD)
}

func (c *GameConsoleService) MasterConsole(clusterName string, command string) {
	if isWindows() {
		WindowGameConsoleService.MasterConsole(clusterName, command)
		return
	}
	cmd := screenStuffCmd(clusterName, "Master", command)
	shellUtils.Shell(cmd)
}

func (c *GameConsoleService) CavesConsole(clusterName string, command string) {
	if isWindows() {
		WindowGameConsoleService.CavesConsole(clusterName, command)
		return
	}
	cmd := screenStuffCmd(clusterName, "Caves", command)
	shellUtils.Shell(cmd)
}

func (c *GameConsoleService) OperatePlayer(clusterName string, otype, kuId string) {
	if isWindows() {
		WindowGameConsoleService.OperatePlayer(clusterName, otype, kuId)
		return
	}
	command := ""
	//复活
	if otype == "0" {
		command = fmt.Sprintf("UserToPlayer('%s'):PushEvent('respawnfromghost')", shellUtils.EscapeLuaStr(kuId)) // B2
	}
	//杀死
	if otype == "1" {
		command = fmt.Sprintf("UserToPlayer('%s'):PushEvent('death')", shellUtils.EscapeLuaStr(kuId)) // B2
	}
	//更换角色
	if otype == "2" {
		command = fmt.Sprintf("c_despawn('%s')", shellUtils.EscapeLuaStr(kuId)) // B2
	}
	c.MasterConsole(clusterName, command)
	c.CavesConsole(clusterName, command)
}

// screenStuffCmd 构建 screen stuff 命令:会话 key 与 lua 命令均经 EscapeShellDQ 转义,
// 防 clusterName/levelName/command 注入(A3)。key 与 lua 都在双引号内,故用 EscapeShellDQ。
func screenStuffCmd(clusterName, levelName, luaCommand string) string {
	key := shellUtils.EscapeShellDQ(screenKey.Key(clusterName, levelName))
	lua := shellUtils.EscapeShellDQ(luaCommand)
	return "screen -S \"" + key + "\" -p 0 -X stuff \"" + lua + "\\n\""
}

func PsAux(processName string) string {
	cmd := "ps -aux | grep -v grep |grep " + shellUtils.Quote(processName) + " |sed -n '2P'|awk '{print $3,$4,$5,$6}'"
	res, err := shellUtils.Shell(cmd)
	if err != nil {
		log.Println("ps -aux |grep " + processName + " error: " + err.Error())
		return ""
	}
	return res
}

func (c *GameConsoleService) ReadLevelServerLog(clusterName, levelName string, length uint) []string {
	// levelServerIniPath := dstUtils2.GetLevelServerIniPath(clusterName, levelName)
	serverLogPath := dstUtils.GetLevelServerLogPath(clusterName, levelName)
	log.Println("serverLogPath", serverLogPath)
	lines, err := fileUtils.ReverseRead(serverLogPath, length)
	if err != nil {
		return []string{}
	}
	return lines
}

func (c *GameConsoleService) ReadLevelServerChatLog(clusterName, levelName string, length uint) []string {
	// levelServerIniPath := dstUtils2.GetLevelServerIniPath(clusterName, levelName)
	serverChatLogPath := dstUtils.GetLevelServerChatLogPath(clusterName, levelName)
	lines, err := fileUtils.ReverseRead(serverChatLogPath, length)
	if err != nil {
		log.Panicln("读取日志server_chat_log失败")
	}
	return lines
}

func (c *GameConsoleService) SendCommand(clusterName string, levelName string, command string) {
	if isWindows() {
		WindowGameConsoleService.SendCommand(clusterName, levelName, command)
		return
	}
	cmd := screenStuffCmd(clusterName, levelName, command)
	shellUtils.Shell(cmd)
}

func (c *GameConsoleService) CSave(clusterName string, levelName string) {

	if isWindows() {
		WindowGameConsoleService.CSave(clusterName, levelName)
		return
	}

	log.Println("正在 s_save() 存档", clusterName, levelName)
	command := "c_save()"
	cmd := screenStuffCmd(clusterName, levelName, command)
	shellUtils.Shell(cmd)

	time.Sleep(5 * time.Second)
}

func (c *GameConsoleService) CSaveMaster(clusterName string) {

	if isWindows() {
		WindowGameConsoleService.CSaveMaster(clusterName)
		return
	}

	c.CSave(clusterName, "Master")
}

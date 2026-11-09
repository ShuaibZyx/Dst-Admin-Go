package service

import (
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"fmt"
	"log"
	"path"
	"time"
)

type WindowsGameConsoleService struct {
	WindowsGameService
}

func (c *WindowsGameConsoleService) SentBroadcast2(clusterName string, levelName string, message string) {

	if c.GetLevelStatus(clusterName, levelName) {
		broadcast := "c_announce(\\\""
		broadcast += message
		broadcast += "\\\")"
		log.Println(broadcast)
		clusterContainer.Send(clusterName, levelName, broadcast)
	}

}

func (c *WindowsGameConsoleService) SentBroadcast(clusterName string, message string) {

	if c.GetLevelStatus(clusterName, "Master") {
		broadcast := "c_announce(\\\""
		broadcast += message
		broadcast += "\\\")"
		log.Println(broadcast)
		clusterContainer.Send(clusterName, "Master", broadcast)
	}

}

func (c *WindowsGameConsoleService) KickPlayer(clusterName, KuId string) {
	masterCMD := "TheNet:Kick(\\\"" + KuId + "\\\")"
	clusterContainer.Send(clusterName, "Master", masterCMD)
}

func (c *WindowsGameConsoleService) KillPlayer(clusterName, KuId string) {
	masterCMD := "UserToPlayer(\\\"" + KuId + "\\\"):PushEvent('death')"
	clusterContainer.Send(clusterName, "Master", masterCMD)
}

func (c *WindowsGameConsoleService) RespawnPlayer(clusterName string, KuId string) {

	masterCMD := "UserToPlayer(\\\"" + KuId + "\\\"):PushEvent('respawnfromghost')"
	clusterContainer.Send(clusterName, "Master", masterCMD)
}

func (c *WindowsGameConsoleService) RollBack(clusterName string, dayNum int) {
	days := fmt.Sprint(dayNum)

	masterCMD := "c_rollback(" + days + ")"
	clusterContainer.Send(clusterName, "Master", masterCMD)
}

func (c *WindowsGameConsoleService) CleanWorld(clusterName string) {

	basePath := dstUtils.GetClusterBasePath(clusterName)

	fileUtils.DeleteDir(path.Join(basePath, "Master", "backup"))
	fileUtils.DeleteDir(path.Join(basePath, "Master", "save"))

	fileUtils.DeleteDir(path.Join(basePath, "Caves", "backup"))
	fileUtils.DeleteDir(path.Join(basePath, "Caves", "save"))
}

func (c *WindowsGameConsoleService) Regenerateworld(clusterName string) {

	c.SentBroadcast(clusterName, ":pig 即将重置世界！！！")
	masterCMD := "c_regenerateworld()"
	clusterContainer.Send(clusterName, "Master", masterCMD)
}

func (c *WindowsGameConsoleService) MasterConsole(clusterName string, command string) {
	clusterContainer.Send(clusterName, "Master", command) // B11: Windows 走 stdin，不经 shell/screen
}

func (c *WindowsGameConsoleService) CavesConsole(clusterName string, command string) {
	clusterContainer.Send(clusterName, "Caves", command) // B11/B1: 洞穴走 Caves
}

func (c *WindowsGameConsoleService) OperatePlayer(clusterName string, otype, kuId string) {
	command := ""
	//复活
	if otype == "0" {
		command = fmt.Sprintf("UserToPlayer('%s'):PushEvent('respawnfromghost')", kuId) // B2
	}
	//杀死
	if otype == "1" {
		command = fmt.Sprintf("UserToPlayer('%s'):PushEvent('death')", kuId) // B2
	}
	//更换角色
	if otype == "2" {
		command = fmt.Sprintf("c_despawn('%s')", kuId) // B2
	}
	c.MasterConsole(clusterName, command)
	c.CavesConsole(clusterName, command)
}

func (c *WindowsGameConsoleService) ReadLevelServerLog(clusterName, levelName string, length uint) []string {
	// levelServerIniPath := dstUtils2.GetLevelServerIniPath(clusterName, levelName)
	serverLogPath := dstUtils.GetLevelServerLogPath(clusterName, levelName)
	lines, err := fileUtils.ReverseRead(serverLogPath, length)
	if err != nil {
		log.Panicln("读取日志server_log失败")
	}
	return lines
}

func (c *WindowsGameConsoleService) ReadLevelServerChatLog(clusterName, levelName string, length uint) []string {
	// levelServerIniPath := dstUtils2.GetLevelServerIniPath(clusterName, levelName)
	serverChatLogPath := dstUtils.GetLevelServerChatLogPath(clusterName, levelName)
	lines, err := fileUtils.ReverseRead(serverChatLogPath, length)
	if err != nil {
		log.Panicln("读取日志server_chat_log失败")
	}
	return lines
}

func (c *WindowsGameConsoleService) SendCommand(clusterName string, levelName string, command string) {
	clusterContainer.Send(clusterName, levelName, command)
}

func (c *WindowsGameConsoleService) CSave(clusterName string, levelName string) {
	log.Println("正在 c_save() 存档", clusterName, levelName)
	clusterContainer.Send(clusterName, levelName, "c_save()") // B11: Windows 走 stdin
	time.Sleep(5 * time.Second)
}

func (c *WindowsGameConsoleService) CSaveMaster(clusterName string) {
	c.CSave(clusterName, "Master")
}

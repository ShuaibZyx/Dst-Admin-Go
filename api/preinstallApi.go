package api

import (
	"dst-admin-go/service/autoCheck"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"path/filepath"
)

type PreinstallApi struct{}

func (p *PreinstallApi) UsePreinstall(ctx *gin.Context) {
	lock.Lock()
	defer lock.Unlock()
	cluster := clusterUtils.GetClusterFromGin(ctx)

	name := ctx.DefaultQuery("name", "default")
	log.Println(name)

	// 先做无副作用校验（模板存在性/IA 闸门），通过后再停服备份——避免误操作先把世界杀了
	if !fileUtils.Exists("./static/preinstall/" + name) {
		abort500f(ctx, "./static/preinstall/"+name, "不存在，请先添加")
		return
	}

	// 闸门:预设含海难/火山世界时要求已订阅 IA 模组
	if err := mod.CheckIADirGate(cluster.ClusterName, "./static/preinstall/"+name); err != nil {
		ctx.JSON(http.StatusBadRequest, vo.Response{Code: 400, Msg: err.Error()})
		return
	}

	// 关闭之前的世界
	gameService.StopGame(cluster.ClusterName)

	// 创建备份（替换前自动保存存档，取代"请自行备份"的提示）
	backupService.CreateBackup(cluster.ClusterName, "")

	path := dstUtils.GetClusterBasePath(cluster.ClusterName)
	bakpath := filepath.Join(dstUtils.GetKleiDstPath(), "bak")
	//删除上次的备份目录 防止更名失败
	err := fileUtils.DeleteDir(bakpath)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	//先重命名但是不删除
	err = fileUtils.Rename(path, bakpath)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	err = fileUtils.Copy("./static/preinstall/"+name, filepath.Join(dstUtils.GetKleiDstPath()))
	if err != nil {
		abort500f(ctx, err)
		return
	}
	newpath := dstUtils.GetClusterBasePath(cluster.ClusterName)
	err = fileUtils.Rename(filepath.Join(dstUtils.GetKleiDstPath(), name), newpath)
	if err != nil {
		abort500f(ctx, err)
		return
	}
	//还原cluster_token 等
	tocopy := []string{"adminlist.txt", "blocklist.txt", "cluster_token.txt", "whitelist.txt"}
	for _, s := range tocopy {
		if fileUtils.Exists(filepath.Join(bakpath, s)) {
			err = fileUtils.Copy(filepath.Join(bakpath, s), newpath)
			if err != nil {
				abort500f(ctx, err)
				return
			}
		}
	}
	err = fileUtils.DeleteDir(bakpath)
	if err != nil {
		abort500f(ctx, err)
		return
	}
	// TODO 宕机恢复重新读取
	autoCheck.Manager.ReStart(cluster.ClusterName)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})

}

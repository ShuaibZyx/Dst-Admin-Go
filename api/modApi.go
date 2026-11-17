package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	mod2 "dst-admin-go/service/mod"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"dst-admin-go/vo"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ModApi struct {
}

func (m *ModApi) SearchModList(ctx *gin.Context) {

	//获取查询参数
	text := ctx.Query("text")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "10"))
	lang := ctx.DefaultQuery("lang", "zh")

	data, err := mod2.SearchModList(text, page, size, lang)
	if err != nil {
		abort500f(ctx, "搜索mod失败", err)
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: data,
	})
}

func (m *ModApi) GetModInfo(ctx *gin.Context) {

	moId := ctx.Param("modId")
	lang := ctx.DefaultQuery("lang", "zh")
	modinfo, err, status := mod2.SubscribeModByModId(moId, lang)
	if err != nil {
		abort500f(ctx, "模组下载失败", "status: ", status)
		return
	}
	var mod_config map[string]interface{}
	_ = json.Unmarshal([]byte(modinfo.ModConfig), &mod_config)
	modData := map[string]interface{}{
		"auth":          modinfo.Auth,
		"consumer_id":   modinfo.ConsumerAppid,
		"creator_appid": modinfo.CreatorAppid,
		"description":   modinfo.Description,
		"file_url":      modinfo.FileUrl,
		"modid":         modinfo.Modid,
		"img":           modinfo.Img,
		"last_time":     modinfo.LastTime,
		"name":          modinfo.Name,
		"v":             modinfo.V,
		"mod_config":    mod_config,
		"update":        modinfo.Update,
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: modData,
	})
}

// GetModSubscribed 查询模组是否已订阅/可用(海难/火山世界创建闸门用)
func (m *ModApi) GetModSubscribed(ctx *gin.Context) {
	modId := ctx.Param("modId")
	clusterName := ctx.Query("cluster")
	if clusterName == "" {
		clusterName = "Cluster1"
	}
	subscribed := mod2.IsModSubscribed(clusterName, modId)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: map[string]interface{}{"modId": modId, "subscribed": subscribed},
	})
}

func (m *ModApi) GetMyModList(ctx *gin.Context) {

	var modInfos []model.ModInfo
	db := database.DB

	db.Find(&modInfos)

	var modDataList []map[string]interface{}

	//var workshopIds []string
	//for i := range modInfos {
	//	workshopIds = append(workshopIds, modInfos[i].Modid)
	//}
	//publishedFileDetails, err := mod.GetPublishedFileDetailsWithGet(workshopIds)

	for _, modinfo := range modInfos {

		//update := false
		//tags := []string{}
		//if err == nil {
		//	for i := range publishedFileDetails {
		//		publishedfiledetail := publishedFileDetails[i]
		//		if modinfo.Modid == publishedfiledetail.Publishedfileid && modinfo.LastTime < publishedfiledetail.TimeUpdated {
		//			update = true
		//			for _, tag := range publishedfiledetail.Tags {
		//				tags = append(tags, tag.Tag)
		//			}
		//		}
		//	}
		//}

		var mod_config map[string]interface{}
		_ = json.Unmarshal([]byte(modinfo.ModConfig), &mod_config)

		modData := map[string]interface{}{
			"auth":          modinfo.Auth,
			"consumer_id":   modinfo.ConsumerAppid,
			"creator_appid": modinfo.CreatorAppid,
			"description":   modinfo.Description,
			"file_url":      modinfo.FileUrl,
			"modid":         modinfo.Modid,
			"img":           modinfo.Img,
			"last_time":     modinfo.LastTime,
			"name":          modinfo.Name,
			"v":             modinfo.V,
			"mod_config":    mod_config,
			"update":        modinfo.Update,
		}
		modDataList = append(modDataList, modData)
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: modDataList,
	})

}

func (m *ModApi) UpdateAllModInfos(ctx *gin.Context) {
	lang := ctx.DefaultQuery("lang", "zh")
	mod2.UpdateModinfoList(lang)
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (m *ModApi) DeleteMod(ctx *gin.Context) {

	modId := ctx.Param("modId")

	// 先全量清扫模组文件；失败（世界运行中占用等）则整体不动、库记录保留，可重试
	if err := mod2.RemoveModFiles(modId); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "模组文件删除失败（世界运行中会被占用），请停止世界后重试: " + err.Error()})
		return
	}
	// 摘除 dedicated_server_mods_setup.lua 里的 ServerModSetup 订阅行（该文件只增不减，须显式清）
	if err := mod2.RemoveModSetupLines(modId); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "订阅行移除失败: " + err.Error()})
		return
	}

	db := database.DB
	db.Where("modid = ?", modId).Delete(&model.ModInfo{})

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: modId,
	})
}

func (m *ModApi) DeleteSetupWorkshop(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	dstPath := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir)
	modsPath := filepath.Join(dstPath, "mods")
	// 删除所有workshop-xxx mod

	directories, err := fileUtils.ListDirectories(modsPath)
	if err != nil {
		abort500f(ctx, "delete dst workshop file error", err)
		return
	}
	var workshopList []string
	for _, directory := range directories {
		if strings.Contains(directory, "workshop") {
			workshopList = append(workshopList, directory)
		}
	}
	for _, workshop := range workshopList {
		err := fileUtils.DeleteDir(workshop)
		if err != nil {
			return
		}
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})
}

func (m *ModApi) GetModInfoFile(ctx *gin.Context) {
	modId := ctx.Param("modId")
	var modInfos model.ModInfo
	db := database.DB
	db.Where("modid = ?", modId).Find(&modInfos)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: modInfos,
	})
}

func (m *ModApi) SaveModInfoFile(ctx *gin.Context) {

	var modInfos model.ModInfo
	err := ctx.ShouldBind(&modInfos)
	if err != nil {
		abort500f(ctx, "参数解析失败")
		return
	}
	// 按 modid 幂等写入，避免请求体无 ID 时 Save 变成重复插入
	if err := mod2.UpsertModInfo(&modInfos); err != nil {
		abort500f(ctx, "模组信息保存失败", err)
		return
	}

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: modInfos,
	})
}

func (m *ModApi) UpdateMod(ctx *gin.Context) {

	modId := ctx.Param("modId")
	lang := ctx.DefaultQuery("lang", "zh")

	// 先全量清扫旧文件(统一目录/steamcmd 暂存/服务端副本,与 DeleteMod 一致);
	// 失败则整体不动、库记录保留可重试
	if err := mod2.RemoveModFiles(modId); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "旧模组文件删除失败（世界运行中会被占用），请停止世界后重试: " + err.Error()})
		return
	}
	db := database.DB
	db.Where("modid = ?", modId).Delete(&model.ModInfo{})

	modinfo, err, status := mod2.SubscribeModByModId(modId, lang)
	if err != nil {
		log.Println("模组更新失败", "status: ", status, "err: ", err)
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "模组更新失败：" + err.Error()})
		return
	}
	var mod_config map[string]interface{}
	json.Unmarshal([]byte(modinfo.ModConfig), &mod_config)
	mod := map[string]interface{}{
		"auth":          modinfo.Auth,
		"consumer_id":   modinfo.ConsumerAppid,
		"creator_appid": modinfo.CreatorAppid,
		"description":   modinfo.Description,
		"file_url":      modinfo.FileUrl,
		"modid":         modinfo.Modid,
		"img":           modinfo.Img,
		"last_time":     modinfo.LastTime,
		"name":          modinfo.Name,
		"v":             modinfo.V,
		"mod_config":    mod_config,
		"update":        modinfo.Update,
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: mod,
	})
}

// AddModInfoFile 手动添加模组
func (m *ModApi) AddModInfoFile(ctx *gin.Context) {

	cluster := clusterUtils.GetClusterFromGin(ctx)
	lang := ctx.DefaultQuery("lang", "zh")

	var payload struct {
		WorkshopId string `json:"workshopId"`
		Modinfo    string `json:"modinfo"`
	}
	err := ctx.ShouldBind(&payload)
	if err != nil {
		abort500f(ctx, "参数解析失败")
		return
	}
	if payload.WorkshopId == "" {
		abort500f(ctx, "workshopId can not be null")
		return
	}

	// 创建workshop文件
	workshopDirPath := filepath.Join(cluster.ModDownloadPath, "/steamapps/workshop/content/322330", payload.WorkshopId)
	fileUtils.CreateDirIfNotExists(workshopDirPath)
	modinfoPath := filepath.Join(workshopDirPath, "modinfo.lua")

	err = fileUtils.CreateFileIfNotExists(modinfoPath)
	if err != nil {
		abort500f(ctx, "创建 modinfo.lua 失败", modinfoPath, err)
		return
	}
	err = fileUtils.WriterTXT(modinfoPath, payload.Modinfo)
	if err != nil {
		abort500f(ctx, "写入 modinfo.lua 失败， path: ", modinfoPath, "error: ", err)
		return
	}

	mod2.AddModInfo(lang, payload.WorkshopId)

	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: nil,
	})

}

// UploadModFile 手动添加模组
func (m *ModApi) UploadModFile(ctx *gin.Context) {

}

type WorkshopItemDetail struct {
	WorkShopId  string  `json:"workshopId"`
	Name        string  `json:"name"`
	Timeupdated int64   `json:"timeupdated"`
	Timelast    float64 `json:"timelast"`
	Img         string  `json:"img"`
}

type AppworkshopAcf struct {
}

func (m *ModApi) GetUgcModAcf(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	levelName := ctx.Query("levelName")

	acfPath := dstUtils.GetUgcAcfPath(cluster.ClusterName, levelName)
	log.Println("acfPath", acfPath)
	acfWorkshops := dstUtils.ParseACFFile(acfPath)

	var workshopItemDetails []WorkshopItemDetail

	var modIds []string
	for key := range acfWorkshops {
		modIds = append(modIds, key)
	}

	urlStr := "http://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", global.SteamAPIKey())
	data.Set("language", "6")
	for i := range modIds {
		data.Set("publishedfileids["+strconv.Itoa(i)+"]", modIds[i])
	}
	urlStr = urlStr + "?" + data.Encode()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		abort500f(ctx, err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		abort500f(ctx, err)
		return
	}

	dataList, ok := result["response"].(map[string]interface{})["publishedfiledetails"].([]interface{})
	if !ok {
		abort500f(ctx, err)
		return
	}
	for i := range dataList {
		workshop := dataList[i].(map[string]interface{})
		_, find := workshop["time_updated"]
		if find {
			timeUpdated := workshop["time_updated"].(float64)
			modId := workshop["publishedfileid"].(string)
			value, ok := acfWorkshops[modId]
			if ok {
				img := workshop["preview_url"].(string)
				img = fmt.Sprintf("%s?imw=64&imh=64&ima=fit&impolicy=Letterbox&imcolor=%%23000000&letterbox=true", img)
				workshopItemDetails = append(workshopItemDetails, WorkshopItemDetail{
					WorkShopId:  modId,
					Timeupdated: value.TimeUpdated,
					Timelast:    timeUpdated,
					Img:         img,
					Name:        workshop["title"].(string),
				})
			}
		}
	}
	ctx.JSON(http.StatusOK, vo.Response{
		Code: 200,
		Msg:  "success",
		Data: workshopItemDetails,
	})
}

func (m *ModApi) DeleteUgcModFile(ctx *gin.Context) {
	cluster := clusterUtils.GetClusterFromGin(ctx)
	clusterName := cluster.ClusterName
	levelName := ctx.Query("levelName")
	workshopId := ctx.Query("workshopId")

	modFilePath := dstUtils.GetUgcWorkshopModPath(clusterName, levelName, workshopId)

	log.Println("modFilePath", modFilePath)
	if fileUtils.Exists(modFilePath) {
		err := fileUtils.DeleteDir(modFilePath)
		if err != nil {
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

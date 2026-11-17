package mod

import (
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/utils/fileUtils"
	"encoding/json"
	"errors"
	"fmt"
	lua "github.com/yuin/gopher-lua"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"archive/zip"
	"bytes"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"
)

const (
	appID    = 322330
	language = 6
)

type searchResult struct {
	Page      int       `json:"page"`
	Size      int       `json:"size"`
	Total     int       `json:"total"`
	TotalPage int       `json:"totalPage"`
	Data      []ModInfo `json:"data"`
}

// ModInfo 存储 mod 信息的结构体
type ModInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Author        string  `json:"author"`
	Desc          string  `json:"desc"`
	Time          int     `json:"time"`
	Sub           int     `json:"sub"`
	Img           string  `json:"img"`
	FileUrl       string  `json:"file_url"`
	V             string  `json:"v"`
	LastTime      float64 `json:"last_time"`
	ConsumerAppid float64 `json:"consumer_appid"`
	CreatorAppid  float64 `json:"creator_appid"`
	Vote          struct {
		Star int `json:"star"`
		Num  int `json:"num"`
	} `json:"vote"`
	Child []string `json:"child,omitempty"`
}

// 获取饥荒本身modid的位置
func get_dst_ucgs_mods_installed_path(modid string) (string, bool) {
	// /root/dst-dedicated-server/ugc_mods/MyDediServer/Master/content/322330/1185229307
	// 先从 mods 文件读取

	dstConfig := dstConfigUtils.GetDstConfig()
	masterModFilePath := dstUtils.GetUgcWorkshopModPath(dstConfig.Cluster, "Master", modid)
	caveModFilePath := dstUtils.GetUgcWorkshopModPath(dstConfig.Cluster, "Caves", modid)

	log.Println("masterModFilePath: ", masterModFilePath)
	log.Println("caveModFilePath: ", caveModFilePath)

	if fileUtils.Exists(masterModFilePath) {
		return masterModFilePath, true
	}
	if fileUtils.Exists(caveModFilePath) {
		return caveModFilePath, true // C3
	}
	return "", false
}

func get_mod_info_config(lang, mod_id string) map[string]interface{} {
	// 从服务器本地读取mod信息
	if dst_mod_installed_path, ok := get_dst_ucgs_mods_installed_path(mod_id); ok {
		modinfo_path := filepath.Join(dst_mod_installed_path, "modinfo.lua")
		log.Println("ucg modinfo.lua: ", modinfo_path)
		if _, err := os.Stat(modinfo_path); err == nil {
			return read_mod_info(lang, mod_id, modinfo_path)
		}
	}

	// 检查 mod 文件是否已经存在
	dstConfig := dstConfigUtils.GetDstConfig()
	mod_download_path := dstConfigUtils.ResolvePath(dstConfig.Parent_path, dstConfig.Mod_download_path, dstConfigUtils.DefaultMod)
	fileUtils.CreateFileIfNotExists(mod_download_path)
	// 下载的模组位置
	mod_path := filepath.Join(mod_download_path, "steamapps", "workshop", "content", "322330", mod_id)
	if _, err := os.Stat(filepath.Join(mod_path, "modinfo.lua")); err != nil {
		// 本地缺失：用统一下载器下载并归位。注意 SteamCMD 的 workshop 下载永远落在
		// steamcmd 缓存目录（force_install_dir 对 workshop_download_item 不生效），
		// 必须由 DownloadModToUnified 归位后 modinfo.lua 才会出现在 mod_path。
		log.Println("模组本地缺失，开始下载: ", mod_id)
		if derr := DownloadModToUnified(mod_id, nil); derr != nil {
			log.Println("模组下载失败: ", derr)
			return make(map[string]interface{})
		}
	}

	// 查找 modinfo.lua 文件
	modinfo_path := filepath.Join(mod_path, "modinfo.lua")
	if _, err := os.Stat(modinfo_path); err != nil {
		fmt.Println("Error finding modinfo.lua:", err)
		return make(map[string]interface{})
	}
	return read_mod_info(lang, mod_id, modinfo_path)
}

func read_mod_info(lang, mod_id, modinfo_path string) map[string]interface{} {
	// 读取 modinfo.lua 文件内容
	script, err := ioutil.ReadFile(modinfo_path)
	if err != nil {
		fmt.Println("Error reading modinfo.lua:", err)
		return make(map[string]interface{})
	}

	// 打印 modinfo.lua 文件内容
	// fmt.Println("Modinfo.lua content:")
	// fmt.Println(string(content))

	return parseModInfoLua(lang, mod_id, string(script))
}

func parseModInfoLua(lang, mod_id, script string) map[string]interface{} {
	L := lua.NewState()
	defer L.Close()

	// 模�~K~_�~P�~L�~N��~C
	//lang := "zh"
	L.SetGlobal("locale", lua.LString(lang))
	L.SetGlobal("folder_name", lua.LString(fmt.Sprintf("workshop-%s", mod_id)))
	L.SetGlobal("ChooseTranslationTable", L.NewFunction(func(L *lua.LState) int {
		tbl := L.ToTable(1)
		langTbl := tbl.RawGetString(lang)
		if langTbl != lua.LNil {
			L.Push(langTbl)
		} else {
			L.Push(tbl.RawGetInt(1))
		}
		return 1
	}))

	// 运行Lua脚本文件
	if err := L.DoString(script); err != nil {
		// 执行出错会得到残缺的元数据，这里必须留痕（此前错误被忽略导致空配置无从排查）
		log.Println("解析 modinfo.lua 出错:", mod_id, err)
	}

	// 获取所有全局变量
	global := L.Get(lua.GlobalsIndex).(*lua.LTable)
	m := make(map[string]interface{})
	global.ForEach(func(k lua.LValue, v lua.LValue) {
		if !excludeList[k.String()] && v.Type() != lua.LTFunction {
			// data, _ := json.Marshal(toInterface(v))
			// fmt.Printf("%s = %v\n", k.String(), string(data))
			m[k.String()] = toInterface(v)
		}
	})

	// data, _ := json.Marshal(m)
	// fmt.Println(string(data))
	return m
}

// 定义需要排除的 Lua 自带对象的名称
var excludeList = map[string]bool{
	"_G": true, "assert": true, "collectgarbage": true, "dofile": true, "error": true,
	"getmetatable": true, "ipairs": true, "load": true, "loadfile": true, "module": true,
	"next": true, "pairs": true, "pcall": true, "print": true, "rawequal": true, "rawget": true,
	"rawset": true, "require": true, "select": true, "setmetatable": true, "tonumber": true,
	"tostring": true, "type": true, "unpack": true, "xpcall": true, "debug": true, "_VERSION": true,
	"os": true, "_GOPHER_LUA_VERSION": true, "string": true, "math": true, "io": true, "channel": true,
	"package": true, "coroutine": true, "table": true,
}

// 将 Lua 值转换为 interface{} 类型
func toInterface(lv lua.LValue) interface{} {
	switch lv.Type() {
	case lua.LTNil:
		return nil
	case lua.LTBool:
		return bool(lv.(lua.LBool))
	case lua.LTNumber:
		return float64(lv.(lua.LNumber))
	case lua.LTString:
		return lv.String()
	case lua.LTTable:
		t := lv.(*lua.LTable)
		if isTableArray(t) {
			arr := make([]interface{}, t.Len())
			t.ForEach(func(i lua.LValue, v lua.LValue) {
				index := int(float64(i.(lua.LNumber)) - 1)
				if index != -1 && index <= len(arr) {
					arr[index] = toInterface(v)
				}
			})
			return arr
		} else {
			return toMap(t)
		}
	default:
		return lv.String()
	}
}

// 将 Lua table 转换为 map[string]interface{}
func toMap(t *lua.LTable) map[string]interface{} {
	m := make(map[string]interface{})
	t.ForEach(func(k lua.LValue, v lua.LValue) {
		key := ""
		switch k.Type() {
		case lua.LTString:
			key = k.String()
		case lua.LTNumber:
			key = fmt.Sprintf("%g", float64(k.(lua.LNumber)))
		default:
			key = fmt.Sprintf("%v", k)
		}
		m[key] = toInterface(v)
	})
	return m
}

// 判断 Lua table 是否为数组类型
func isTableArray(t *lua.LTable) bool {
	maxIndex := 0
	isSequential := true
	t.ForEach(func(k lua.LValue, v lua.LValue) {
		if i, ok := k.(lua.LNumber); ok {
			if i != lua.LNumber(int(i)) {
				isSequential = false
			} else if int(i) > maxIndex {
				maxIndex = int(i)
			}
		} else {
			isSequential = false
		}
	})
	return isSequential && maxIndex == t.Len()
}

// SearchModList 搜索 mod 的函数
func SearchModList(text string, page int, num int, lang string) (map[string]interface{}, error) {
	modId, ok := isModId(text)
	if ok {
		modInfo := SearchModInfoByWorkshopId(modId)
		data := []ModInfo{}
		if modInfo.ID != "" {
			data = append(data, modInfo)
		}
		return map[string]interface{}{
			"page":      1,
			"size":      1,
			"total":     1,
			"totalPage": 1,
			"data":      data,
		}, nil
	}

	urlStr := "http://api.steampowered.com/IPublishedFileService/QueryFiles/v1/"
	data := url.Values{
		"page":             {fmt.Sprintf("%d", page)},
		"key":              {global.SteamAPIKey()},
		"appid":            {"322330"},
		"language":         {"6"},
		"return_tags":      {"true"},
		"numperpage":       {fmt.Sprintf("%d", num)},
		"search_text":      {text},
		"return_vote_data": {"true"},
		"return_children":  {"true"},
	}
	if lang == "zh" {
		data.Set("language", "6")
	} else {
		data.Set("language", "")
	}
	query := func() (map[string]interface{}, error) {
		var out map[string]interface{}
		for i := 0; i < 2; i++ {
			resp, err := http.Get(urlStr + "?" + data.Encode())
			if err != nil {
				fmt.Println("搜索 mod 失败")
				return nil, err
			}
			derr := json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			if derr != nil {
				fmt.Println("解析 mod 数据失败")
				return nil, derr
			}
			if out["response"] != nil {
				break
			}
		}
		if out == nil || out["response"] == nil {
			return nil, fmt.Errorf("no response found in mod data")
		}
		return out, nil
	}

	modData, err := query()
	if err != nil {
		return nil, err
	}
	// 中文检索在 language=6 下分词失灵（实测 total=0）：无结果时去语言过滤重试
	if resp0, ok := modData["response"].(map[string]interface{}); ok {
		if t0, ok2 := resp0["total"].(float64); ok2 && int(t0) == 0 && lang == "zh" {
			data.Set("language", "")
			if retryData, rerr := query(); rerr == nil {
				modData = retryData
			}
		}
	}

	modResponse := modData["response"].(map[string]interface{})
	total := int(modResponse["total"].(float64))
	modInfoRaw, _ := modResponse["publishedfiledetails"].([]interface{})

	modList := make([]ModInfo, 0)
	if len(modInfoRaw) > 0 {
		for _, modInfoRaw := range modInfoRaw {
			modInfo := modInfoRaw.(map[string]interface{})
			img := modInfo["preview_url"].(string)
			voteData := modInfo["vote_data"].(map[string]interface{})
			auth := modInfo["creator"].(string)
			var authorURL string
			if auth != "" {
				authorURL = fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", auth)
			}
			mod := ModInfo{
				ID:     fmt.Sprintf("%v", modInfo["publishedfileid"]),
				Name:   fmt.Sprintf("%v", modInfo["title"]),
				Author: authorURL,
				Desc:   fmt.Sprintf("%v", modInfo["file_description"]),
				Time:   int(modInfo["time_updated"].(float64)),
				Sub:    int(modInfo["subscriptions"].(float64)),
				Img:    img,
				Vote: struct {
					Star int `json:"star"`
					Num  int `json:"num"`
				}{
					Star: int(voteData["score"].(float64)*5) + 1,
					Num:  int(voteData["votes_up"].(float64) + voteData["votes_down"].(float64)),
				},
			}
			if modInfo["num_children"].(float64) != 0 {
				children := modInfo["children"].([]interface{})
				child := make([]string, len(children))
				for i, c := range children {
					child[i] = fmt.Sprintf("%v", c.(map[string]interface{})["publishedfileid"])
				}
				mod.Child = child
			}
			modList = append(modList, mod)
		}
	}

	return map[string]interface{}{
		"page":      page,
		"size":      num,
		"total":     total,
		"totalPage": int(math.Ceil(float64(total) / float64(num))),
		"data":      modList,
	}, nil
}

func GetModInfo2(modID string) (model.ModInfo, error, int) {
	urlStr := "http://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", global.SteamAPIKey())
	data.Set("language", "6")
	data.Set("publishedfileids[0]", modID)
	urlStr = urlStr + "?" + data.Encode()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 1
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 2
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 3
	}

	dataList, ok := result["response"].(map[string]interface{})["publishedfiledetails"].([]interface{})
	if !ok || len(dataList) == 0 {
		fmt.Println("get mod error")
		return model.ModInfo{}, err, 4
	}

	data2 := dataList[0].(map[string]interface{})
	img := data2["preview_url"].(string)
	auth := data2["creator"].(string)
	var authorURL string
	if auth != "" {
		authorURL = fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", auth)
	} else {
		authorURL = ""
	}

	modId := data2["publishedfileid"].(string)
	name := data2["title"].(string)
	last_time := data2["time_updated"].(float64)
	description := data2["file_description"].(string)
	auth = authorURL
	file_url := data2["file_url"]
	img = fmt.Sprintf("%s?imw=64&imh=64&ima=fit&impolicy=Letterbox&imcolor=%%23000000&letterbox=true", img)
	v := getVersion(data["views"])
	creator_appid := data2["creator_appid"].(float64)
	consumer_appid := data2["consumer_appid"].(float64)

	var fileUrl = ""
	if file_url != nil {
		fileUrl = file_url.(string)
	}
	newModInfo := model.ModInfo{
		Auth:          auth,
		ConsumerAppid: consumer_appid,
		CreatorAppid:  creator_appid,
		Description:   description,
		FileUrl:       fileUrl,
		Modid:         modId,
		Img:           img,
		LastTime:      last_time,
		Name:          name,
		V:             v,
	}
	return newModInfo, nil, 0
}

func isWorkshopId(id string) bool {
	_, err := strconv.Atoi(id)
	return err == nil
}

func SubscribeModByModId(modId, lang string) (model.ModInfo, error, int) {

	if !isWorkshopId(modId) {

		modConfigJson, _ := json.Marshal(get_mod_info_config(lang, modId))
		modConfig := string(modConfigJson)

		newModInfo := model.ModInfo{
			Auth:          "",
			ConsumerAppid: 0,
			CreatorAppid:  0,
			Description:   "",
			Modid:         modId,
			Img:           "xxx",
			LastTime:      0,
			Name:          modId,
			V:             "",
			ModConfig:     modConfig,
		}

		if err := UpsertModInfo(&newModInfo); err != nil {
			return newModInfo, err, 0
		}
		return newModInfo, nil, 0
	}

	urlStr := "http://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", global.SteamAPIKey())
	data.Set("language", "6")
	data.Set("publishedfileids[0]", modId)
	urlStr = urlStr + "?" + data.Encode()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 1
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 2
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return model.ModInfo{}, err, 3
	}

	dataList, ok := result["response"].(map[string]interface{})["publishedfiledetails"].([]interface{})
	if !ok || len(dataList) == 0 {
		fmt.Println("get mod error")
		return model.ModInfo{}, err, 4
	}

	data2 := dataList[0].(map[string]interface{})
	img := data2["preview_url"].(string)
	auth := data2["creator"].(string)
	var authorURL string
	if auth != "" {
		authorURL = fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", auth)
	} else {
		authorURL = ""
	}

	name := data2["title"].(string)
	last_time := data2["time_updated"].(float64)
	description := data2["file_description"].(string)
	auth = authorURL
	file_url := data2["file_url"]
	img = fmt.Sprintf("%s?imw=64&imh=64&ima=fit&impolicy=Letterbox&imcolor=%%23000000&letterbox=true", img)
	v := getVersion(data["views"])
	creator_appid := data2["creator_appid"].(float64)
	consumer_appid := data2["consumer_appid"].(float64)

	if modInfo, ok := getModInfoConfig2(modId); ok {
		// 配置为空的记录（历史解析失败遗留）即使时间戳一致也重新解析，否则空配置永远无法恢复
		if last_time == modInfo.LastTime && modInfo.ModConfig != "" && modInfo.ModConfig != "{}" {
			return modInfo, nil, 0
		}

		// 更新配置项
		var modConfig string
		var fileUrl = ""
		if file_url != nil {
			fileUrl = file_url.(string)
		}
		if fileUrl != "" {
			modConfigJson, _ := json.Marshal(get_v1_mod_info_config(lang, modId, fileUrl))
			modConfig = string(modConfigJson)
		} else {
			modConfigJson, _ := json.Marshal(get_mod_info_config(lang, modId))
			modConfig = string(modConfigJson)
		}

		modInfo.LastTime = last_time
		modInfo.Name = name
		modInfo.Auth = auth
		modInfo.Description = description
		modInfo.Img = img
		modInfo.V = v
		modInfo.ModConfig = modConfig
		modInfo.Update = false
		db := database.DB
		db.Save(&modInfo)
		return modInfo, nil, 0
	}

	var fileUrl = ""
	if file_url != nil {
		fileUrl = file_url.(string)
	}

	var modConfig string

	if fileUrl != "" {
		modConfigJson, _ := json.Marshal(get_v1_mod_info_config(lang, modId, fileUrl))
		modConfig = string(modConfigJson)
	} else {
		modConfigJson, _ := json.Marshal(get_mod_info_config(lang, modId))
		modConfig = string(modConfigJson)
	}

	newModInfo := model.ModInfo{
		Auth:          auth,
		ConsumerAppid: consumer_appid,
		CreatorAppid:  creator_appid,
		Description:   description,
		FileUrl:       fileUrl,
		Modid:         modId,
		Img:           img,
		LastTime:      last_time,
		Name:          name,
		V:             v,
		ModConfig:     modConfig,
	}

	if err := UpsertModInfo(&newModInfo); err != nil {
		return newModInfo, err, 0
	}
	return newModInfo, nil, 0
}

func getModInfoConfig2(modid string) (model.ModInfo, bool) {
	db := database.DB
	modInfo := model.ModInfo{}
	db.Where("modid = ?", modid).First(&modInfo)

	if modInfo.Modid == "" {
		return modInfo, false
	}
	return modInfo, true
}

func getModInfoConfig(modid string, lastTime float64) (model.ModInfo, bool) {
	db := database.DB
	modInfo := model.ModInfo{}
	db.Where("modid = ? and last_time = ?", modid, lastTime).First(&modInfo)

	if modInfo.Modid == "" {
		return modInfo, false
	}
	return modInfo, true
}

func getVersion(tags interface{}) string {
	tagList, ok := tags.([]interface{})
	if !ok {
		return ""
	}
	for _, tag := range tagList {
		tagStr := tag.(map[string]interface{})["tag"].(string)
		if len(tagStr) > 8 && tagStr[:8] == "version:" {
			return tagStr[8:]
		}
	}
	return ""
}

// installCdnZip CDN 安装:解压 zip 归位到统一模组目录并完成收尾登记
// (游戏侧 ACF 登记 + legacy bin 预置)。返回 zipReader 供调用方继续提取 modinfo。
func installCdnZip(zipBytes []byte, modid, fileURL string) (*zip.Reader, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	modDir := filepath.Join(dstUtils.GetUgcModPath(), "content", "322330", modid)
	UnzipToDir(zipReader, modDir)
	// CDN 解压安装同样要登记游戏侧 acf,否则游戏判"待下载"而不启用该模组
	if err := RegisterInstalledBest(modid, modDir, fileURL, 0); err != nil {
		return nil, errors.New("游戏侧登记失败: " + err.Error())
	}
	// 预置 legacy bin:游戏对无法按 manifest 认领的模组走"下载→解包 <ugchandle>_legacy.bin",
	// 文件缺失时解包失败(UMFD failed to open file),模组整个不启用
	if err := WriteLegacyBin(modDir, UgchandleFromURL(fileURL), zipBytes); err != nil {
		log.Println("[legacy-bin] 预置失败:", modid, err)
	}
	// 收尾验收:ACF 登记有效或 legacy bin 在位,不过则整体报错(下载完成=游戏可用)
	if err := verifyGameReadyFn(modid, modDir); err != nil {
		return nil, errors.New("安装收尾验收失败: " + err.Error())
	}
	return zipReader, nil
}

func get_v1_mod_info_config(lang, modid, file_url string) map[string]interface{} {
	log.Println("开始下载 v1 mod，并提取 modinfo.lua 文件")
	// 0: 下载失败, 1: 下载成功, 2: mod 中没有 modinfo.lua 文件
	modinfo := map[string][]byte{"modinfo": nil, "modinfo_chs": nil}
	var tmp bytes.Buffer
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", file_url, nil)
		if err != nil {
			log.Println(file_url, "下载失败")
			log.Println(err)
			continue
		}
		client := http.Client{
			Timeout: time.Duration(10 * time.Second),
		}
		res, err := client.Do(req)
		if err != nil {
			log.Println(file_url, "下载失败")
			log.Println(err)
			continue
		}
		defer res.Body.Close()
		_, err = tmp.ReadFrom(res.Body)
		if err != nil {
			log.Println(err)
			continue
		}
		break
	}
	if tmp.Len() == 0 {
		log.Panicln(file_url, "下载失败 3 次，不再尝试")
		return make(map[string]interface{})
	}
	log.Println(file_url, "下载成功，开始解压")
	zipReader, err := installCdnZip(tmp.Bytes(), modid, file_url)
	if err != nil {
		log.Panicln("模组zip 解压失败")
		return make(map[string]interface{})
	}
	for _, file := range zipReader.File {
		switch file.Name {
		case "modinfo.lua":
			log.Println(file_url, "开始解压 modinfo.lua")
			f, _ := file.Open()
			modinfoBytes, err := ioutil.ReadAll(f)
			if err != nil {
				log.Println(file_url, "解压 modinfo.lua 失败")
				log.Println(err)
				continue
			}
			modinfo["modinfo"] = modinfoBytes
		case "modinfo_chs.lua":
			log.Println(file_url, "开始解压 modinfo_chs.lua")
			f, _ := file.Open()
			modinfoBytes, err := ioutil.ReadAll(f)
			if err != nil {
				log.Println(file_url, "解压 modinfo_chs.lua 失败")
				log.Println(err)
				continue
			}
			modinfo["modinfo_chs"] = modinfoBytes
		}
	}
	if modinfo["modinfo"] != nil {
		return parseModInfoLua(lang, modid, string(modinfo["modinfo"]))
	}
	return make(map[string]interface{})
}

func isModId(str string) (int, bool) {
	id, err := strconv.Atoi(str)
	return id, err == nil
}

func SearchModInfoByWorkshopId(modID int) ModInfo {

	urlStr := "http://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", global.SteamAPIKey())
	data.Set("language", "6")
	data.Set("publishedfileids[0]", strconv.Itoa(modID))
	urlStr = urlStr + "?" + data.Encode()

	log.Println(urlStr)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		fmt.Println(err)
		return ModInfo{}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return ModInfo{}
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return ModInfo{}
	}

	dataList, ok := result["response"].(map[string]interface{})["publishedfiledetails"].([]interface{})
	if !ok || len(dataList) == 0 {
		fmt.Println("get mod error")
		return ModInfo{}
	}

	data2 := dataList[0].(map[string]interface{})
	if data2["consumer_appid"] == nil || data2["consumer_appid"].(float64) != 322330 {
		return ModInfo{}
	}
	img := data2["preview_url"].(string)
	auth := data2["creator"].(string)
	var authorURL string
	if auth != "" {
		authorURL = fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", auth)
	} else {
		authorURL = ""
	}

	modId := data2["publishedfileid"].(string)
	name := data2["title"].(string)
	description := data2["file_description"].(string)
	auth = authorURL
	img = fmt.Sprintf("%s?imw=64&imh=64&ima=fit&impolicy=Letterbox&imcolor=%%23000000&letterbox=true", img)
	modInfo := ModInfo{
		ID:     modId,
		Name:   name,
		Author: auth,
		Desc:   description,
		Time:   int(data2["time_updated"].(float64)),
		Sub:    int(data2["subscriptions"].(float64)),
		Img:    img,
	}

	return modInfo
}

func AddModInfo(lang, modid string) {
	var modInfo model.ModInfo
	var err error
	if !isWorkshopId(modid) {
		modInfo, err, _ = GetLocalModInfo(lang, modid)
	} else {
		// 获取mod基本信息
		modInfo, err, _ = GetModInfo2(modid)
	}

	if err != nil {
		log.Panicln("获取modinfo 失败", err)
	}
	// 从数据查找是由有
	oldModinfo, ok := getModInfoConfig2(modid)

	// 更新配置项
	var modConfig string
	modConfigJson, err := json.Marshal(get_mod_info_config(lang, modid))
	if err != nil {
		log.Println(err)
	}
	modConfig = string(modConfigJson)

	if ok {
		oldModinfo.LastTime = modInfo.LastTime
		oldModinfo.Name = modInfo.Name
		oldModinfo.Auth = modInfo.Auth
		oldModinfo.Description = modInfo.Description
		oldModinfo.Img = modInfo.Img
		oldModinfo.V = modInfo.V
		oldModinfo.ModConfig = modConfig
		if err := UpsertModInfo(&oldModinfo); err != nil {
			log.Println("mod info save error: ", err)
		}
	} else {
		modInfo.ModConfig = modConfig
		if err := UpsertModInfo(&modInfo); err != nil {
			log.Println("mod info save error: ", err)
		}
	}

}

func GetLocalModInfo(lang, modId string) (model.ModInfo, error, int) {
	modConfigJson, _ := json.Marshal(get_mod_info_config(lang, modId))
	modConfig := string(modConfigJson)

	newModInfo := model.ModInfo{
		Auth:          "",
		ConsumerAppid: 0,
		CreatorAppid:  0,
		Description:   "",
		Modid:         modId,
		Img:           "xxx",
		LastTime:      0,
		Name:          modId,
		V:             "",
		ModConfig:     modConfig,
	}

	if err := UpsertModInfo(&newModInfo); err != nil {
		return newModInfo, err, 0
	}
	return newModInfo, nil, 0
}

func UnzipToDir(zipReader *zip.Reader, destDir string) error {
	for _, file := range zipReader.File {
		// 获取目标文件的路径
		destPath := filepath.Join(destDir, file.Name)

		// 检查目录路径的安全性，避免目录遍历漏洞
		if !filepath.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法文件路径: %s", destPath)
		}

		// 如果是目录，则创建它
		if file.FileInfo().IsDir() {
			err := os.MkdirAll(destPath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		// 如果是文件，先确保目录存在
		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return err
		}

		// 创建文件并写入解压内容
		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer outFile.Close()

		// 打开 zip 文件中的文件流
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		// 将文件内容写入到本地文件
		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

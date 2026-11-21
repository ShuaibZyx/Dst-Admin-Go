package mod

import (
	"bytes"
	"crypto/tls"
	"dst-admin-go/config/database"
	"dst-admin-go/config/global"
	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/fileUtils"
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// steamHTTPClient 复用连接并带全链路超时。
// 此前是裸的 &http.Client{Transport: &http.Transport{TLSClientConfig: ...}}：没有 Dial/TLS/
// 响应头/总超时，一旦连到不可达的 Akamai IP 就一路挂到 OS TCP 超时（实测单次 55s~2m31s），
// 前端表现就是「接口不响应」。Proxy 走环境变量，网络受限时可挂代理。
var steamHTTPClient = &http.Client{
	Timeout: 40 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// 非 http.DefaultTransport 的 Transport 默认不做 HTTP/2，显式开启
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // 忽略证书验证
		DialContext:           (&net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   8 * time.Second,
		// 命中黑洞 IP 时 TCP/TLS 都能连上但收不到响应头，就靠这里掐断。
		// 单次成功实测 1~5s，12s 留足余量（再短会误伤批量请求）
		ResponseHeaderTimeout: 12 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// steamAttempts Steam API 的网络是间歇性的：同一域名背后多个 Akamai IP，部分不通
// （实测 curl 8/8 成功且全落在 .192/.156；Go 命中的 .136/.142/.152 全超时）。
// 每次重试重新拨号，多数能落到可用 IP。
const steamAttempts = 3

type Publishedfiledetail struct {
	Publishedfileid string  `json:"publishedfileid"`
	Result          int     `json:"result"`
	Creator         string  `json:"creator"`
	CreatorAppID    int     `json:"creator_app_id"`
	ConsumerAppID   int     `json:"consumer_app_id"`
	Filename        string  `json:"filename"`
	FileURL         string  `json:"file_url"`
	HcontentFile    string  `json:"hcontent_file"`
	PreviewURL      string  `json:"preview_url"`
	HcontentPreview string  `json:"hcontent_preview"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	TimeCreated     float64 `json:"time_created"`
	TimeUpdated     float64 `json:"time_updated"`
	Visibility      int     `json:"visibility"`

	BanReason             string `json:"ban_reason"`
	Subscriptions         int    `json:"subscriptions"`
	Favorited             int    `json:"favorited"`
	LifetimeSubscriptions int    `json:"lifetime_subscriptions"`
	LifetimeFavorited     int    `json:"lifetime_favorited"`
	Views                 int    `json:"views"`
	Tags                  []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
}

type PublishedFileDetailsData struct {
	Response struct {
		Result               int                   `json:"result"`
		Resultcount          int                   `json:"resultcount"`
		Publishedfiledetails []Publishedfiledetail `json:"publishedfiledetails"`
	} `json:"response"`
}

type PublishedFileDetailsDataGet struct {
	Response struct {
		Publishedfiledetails []Publishedfiledetail `json:"publishedfiledetails"`
	} `json:"response"`
}

func GetPublishedFileDetails(workshopIds []string) ([]Publishedfiledetail, error) {
	url := "https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/"
	method := "POST"

	var lastErr error
	for attempt := 1; attempt <= steamAttempts; attempt++ {
		// 每次重试都要重建 multipart body：POST body 是一次性流，复用会读到空
		payload := &bytes.Buffer{}
		writer := multipart.NewWriter(payload)
		_ = writer.WriteField("itemcount", strconv.Itoa(len(workshopIds)))
		for i := range workshopIds {
			_ = writer.WriteField("publishedfileids["+strconv.Itoa(i)+"]", workshopIds[i])
		}
		if err := writer.Close(); err != nil {
			log.Println(err)
			return nil, err
		}

		req, err := http.NewRequest(method, url, payload)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := steamHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("Steam API 第 %d/%d 次失败: %v", attempt, steamAttempts, err)
			continue
		}

		var publishedFileDetailsData PublishedFileDetailsData
		err = json.NewDecoder(res.Body).Decode(&publishedFileDetailsData)
		res.Body.Close()
		if err != nil {
			lastErr = err
			log.Printf("Steam API 第 %d/%d 次解码失败: %v", attempt, steamAttempts, err)
			continue
		}

		if publishedFileDetailsData.Response.Result == 1 {
			return publishedFileDetailsData.Response.Publishedfiledetails, nil
		}
		lastErr = errors.New("请求失败")
	}
	return nil, lastErr
}

func GetPublishedFileDetailsBatched(workshopIds []string, batchSize int) ([]Publishedfiledetail, error) {
	var allPublishedFileDetails []Publishedfiledetail

	// 拆分 workshopIds 到批次
	for i := 0; i < len(workshopIds); i += batchSize {
		end := i + batchSize
		if end > len(workshopIds) {
			end = len(workshopIds)
		}

		// 获取当前批次的 workshopIds
		batch := workshopIds[i:end]

		// 调用原始函数
		publishedFileDetails, err := GetPublishedFileDetailsWithGet(batch)
		if err != nil {
			return nil, err
		}

		// 将结果添加到总结果中
		allPublishedFileDetails = append(allPublishedFileDetails, publishedFileDetails...)
	}

	return allPublishedFileDetails, nil
}

func GetPublishedFileDetailsWithGet(workshopIds []string) ([]Publishedfiledetail, error) {
	// 必须 https：API key 在查询串里，明文 http 会把它直接暴露在网络上
	urlStr := "https://api.steampowered.com/IPublishedFileService/GetDetails/v1/"
	data := url.Values{}
	data.Set("key", global.SteamAPIKey())
	data.Set("language", "6")
	for i := range workshopIds {
		data.Set("publishedfileids["+strconv.Itoa(i)+"]", workshopIds[i])
	}
	urlStr = urlStr + "?" + data.Encode()

	var lastErr error
	for attempt := 1; attempt <= steamAttempts; attempt++ {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		res, err := steamHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("Steam API(GET) 第 %d/%d 次失败: %v", attempt, steamAttempts, err)
			continue
		}

		var publishedFileDetailsData PublishedFileDetailsDataGet
		err = json.NewDecoder(res.Body).Decode(&publishedFileDetailsData)
		res.Body.Close()
		if err != nil {
			lastErr = err
			log.Printf("Steam API(GET) 第 %d/%d 次解码失败: %v", attempt, steamAttempts, err)
			continue
		}
		return publishedFileDetailsData.Response.Publishedfiledetails, nil
	}
	return nil, lastErr
}

// GetModModinfoLua TODO 获取模组 modinfo.lua
func GetModModinfoLua(workshopId string) string {
	return ""
}

func CheckModInfoUpdate() {
	var modInfos []model.ModInfo
	var needUpdateList []model.ModInfo
	var workshopIds []string

	db := database.DB
	db.Find(&modInfos)

	for i := range modInfos {
		workshopIds = append(workshopIds, modInfos[i].Modid)
	}

	publishedFileDetails, err := GetPublishedFileDetails(workshopIds)
	if err == nil {
		for i := range publishedFileDetails {
			publishedfiledetail := publishedFileDetails[i]
			for j := range modInfos {
				if modInfos[j].Modid == publishedfiledetail.Publishedfileid && modInfos[j].LastTime < publishedfiledetail.TimeUpdated {
					needUpdateList = append(needUpdateList, modInfos[i])
				}
			}
		}
		if len(needUpdateList) > 0 {
			for i := range needUpdateList {
				needUpdateList[i].Update = true
			}
			db.Save(&needUpdateList)
		}
	}
}

func UpdateModinfoList(lang string) {

	var modInfos []model.ModInfo
	var needUpdateList []model.ModInfo
	var workshopIds []string

	db := database.DB
	db.Find(&modInfos)

	for i := range modInfos {
		workshopIds = append(workshopIds, modInfos[i].Modid)
	}
	publishedFileDetails, err := GetPublishedFileDetailsBatched(workshopIds, 20)
	if err != nil {
		log.Panicln(err)
	}
	seen := map[string]bool{}
	for i := range publishedFileDetails {
		publishedfiledetail := publishedFileDetails[i]
		for j := range modInfos {
			if modInfos[j].Modid != publishedfiledetail.Publishedfileid {
				continue
			}
			// 配置为空的记录（历史解析失败遗留）也加入重解析，否则空配置永远无法恢复
			configEmpty := modInfos[j].ModConfig == "" || modInfos[j].ModConfig == "{}"
			if (modInfos[j].LastTime < publishedfiledetail.TimeUpdated || configEmpty) && !seen[modInfos[j].Modid] {
				seen[modInfos[j].Modid] = true
				needUpdateList = append(needUpdateList, modInfos[j])
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(needUpdateList))

	for i := range needUpdateList {
		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					log.Println(r)
				}
				wg.Done()
			}()
			modId := needUpdateList[i].Modid
			// 删除之前的数据
			dstConfig := dstConfigUtils.GetDstConfig()
			mod_download_path := dstConfigUtils.ResolvePath(dstConfig.Parent_path, dstConfig.Mod_download_path, dstConfigUtils.DefaultMod)
			mod_path := filepath.Join(mod_download_path, "/steamapps/workshop/content/322330/", modId)
			_ = fileUtils.DeleteDir(mod_path)
			_, _, _ = SubscribeModByModId(modId, lang)
		}(i)
	}
	wg.Wait()

}

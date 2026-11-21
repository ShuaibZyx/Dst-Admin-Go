package service

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 我的订阅（Steam Workshop）：Cookie 抓取订阅页 + GetPublishedFileDetails 校验入库。
// 订阅库是跨存档的模组资产层——可下载/更新/删除，但不可配置/启用（那是存档模组的事）。

var (
	reFileDetails = regexp.MustCompile(`filedetails/\?id=([0-9]+)`)
	rePublishedId = regexp.MustCompile(`publishedfileid\\?":\\?"?([0-9]+)`)
)

// ParseWorkshopFileIds 从订阅页 HTML 提取模组 ID（两种格式，去重保序）
func ParseWorkshopFileIds(html string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, m := range reFileDetails.FindAllStringSubmatch(html, -1) {
		add(m[1])
	}
	for _, m := range rePublishedId.FindAllStringSubmatch(html, -1) {
		add(m[1])
	}
	return out
}

// fetchPageWithRetry 抓单页：空响应/网络错误重试（代理不稳定时缺页=丢订阅），全部失败报错
func fetchPageWithRetry(do func(string) (string, error), u string, attempts int) (string, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		html, err := do(u)
		if err == nil && strings.TrimSpace(html) != "" {
			return html, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = errors.New("empty response")
		}
		if i < attempts-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return "", lastErr
}

// FetchMySubscriptions 用 Cookie 翻页抓取"我的订阅"（myworkshopfiles=权威来源，每页 30，6 页 180 槽位）。
// 不叠加 browse 页：其 HTML 混入其他游戏推荐位的 publishedfileid，徒增垃圾 ID 与详情查询。
func FetchMySubscriptions(steamID64, cookie string) ([]string, error) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0"
	seen := map[string]bool{}
	var ids []string
	addAll := func(list []string) {
		for _, id := range list {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	fetch := func(u string) (string, error) {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Cookie", cookie)
		req.Header.Set("User-Agent", ua)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	base := "https://steamcommunity.com/profiles/" + url.PathEscape(steamID64) + "/myworkshopfiles?appid=322330&browsefilter=mysubscriptions&numperpage=30"
	// 并发抓页（串行太慢），每页失败重试
	var mu sync.Mutex
	var wg sync.WaitGroup
	failed := 0
	for p := 1; p <= 6; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			html, err := fetchPageWithRetry(fetch, base+"&p="+strconv.Itoa(p), 3)
			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				log.Printf("[subs] 第 %d 页抓取失败: %v", p, err)
				return
			}
			page := ParseWorkshopFileIds(html)
			mu.Lock()
			addAll(page)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	if failed > 0 && len(ids) == 0 {
		return nil, errors.New("订阅页全部抓取失败")
	}
	if failed > 0 {
		// 部分页失败：返回已抓到的 ID，但带错误信号——调用方不得据此做镜像删除
		return ids, errors.New("部分订阅页抓取失败")
	}
	return ids, nil
}

// ExtractCookie 从用户输入提取 Steam Cookie：纯 cookie 直接用；
// 粘贴 curl 命令时自动解析 -b / --cookie（兼容 cmd 脱字符 ^ 转义与单双引号）
func ExtractCookie(input string) string {
	s := strings.ReplaceAll(input, "^", "")
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "curl") && !strings.Contains(s, "-b") && !strings.Contains(s, "--cookie") {
		return s
	}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?:-b|--cookie)\s+"([^"]+)"`),
		regexp.MustCompile(`(?:-b|--cookie)\s+'([^']+)'`),
	} {
		if m := re.FindStringSubmatch(s); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// DeleteSubscriptionRecord 按 modid 删除订阅记录，返回是否删到
func DeleteSubscriptionRecord(modid string) (bool, error) {
	db := database.DB
	var rec model.ModSubscription
	if err := db.Where("modid = ?", modid).First(&rec).Error; err != nil {
		return false, nil
	}
	if err := db.Unscoped().Delete(&rec).Error; err != nil {
		return false, err
	}
	return true, nil
}

// LocalFile 本地文件条目（相对模组目录）
type LocalFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// LocalModInfo 已下载模组的本地信息：路径/体积/文件清单/modinfo.lua 原文
type LocalModInfo struct {
	Exists    bool        `json:"exists"`
	Path      string      `json:"path"`
	SizeBytes int64       `json:"sizeBytes"`
	FileCount int         `json:"fileCount"`
	Files     []LocalFile `json:"files"`
	Modinfo   string      `json:"modinfo"`
}

// GetLocalModInfo 读统一下载目录中该模组的本地信息
func GetLocalModInfo(modid string) LocalModInfo {
	cfg := dstConfigUtils.GetDstConfig()
	root := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod)
	dir := filepath.Join(root, "steamapps", "workshop", "content", "322330", modid)
	info := LocalModInfo{Path: dir}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return info
	}
	info.Exists = true
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info.FileCount++
		var size int64
		if fi, e := d.Info(); e == nil {
			size = fi.Size()
		}
		info.SizeBytes += size
		rel, _ := filepath.Rel(dir, p)
		info.Files = append(info.Files, LocalFile{Name: filepath.ToSlash(rel), Size: size})
		return nil
	})
	sort.Slice(info.Files, func(i, j int) bool { return info.Files[i].Size > info.Files[j].Size })
	if b, err := os.ReadFile(filepath.Join(dir, "modinfo.lua")); err == nil {
		info.Modinfo = string(b)
	}
	return info
}

// GetSubscriptionDetail 详情：工坊实时信息 + 本地文件信息（下载后能拿到的全量）
func GetSubscriptionDetail(modid string) (map[string]interface{}, error) {
	out := map[string]interface{}{"local": GetLocalModInfo(modid)}
	var rec model.ModInfo
	if err := database.DB.Where("modid = ?", modid).First(&rec).Error; err == nil {
		out["hasUpdate"] = rec.Update
		out["syncedAt"] = rec.CreatedAt.Unix()
	}
	// 工坊信息取不到必须上抛：此前这里静默吞掉错误，上层照样返回 200 + 没有 workshop 字段，
	// 前端只能显示一个空详情，看不到任何失败原因
	details, err := mod.GetPublishedFileDetails([]string{modid})
	if err != nil {
		return out, err
	}
	if len(details) == 0 {
		return out, errors.New("未找到该创意工坊条目")
	}
	d := details[0]
	out["workshop"] = map[string]interface{}{
		"title":                 d.Title,
		"description":           d.Description,
		"previewUrl":            d.PreviewURL,
		"timeCreated":           d.TimeCreated,
		"timeUpdated":           d.TimeUpdated,
		"subscriptions":         d.Subscriptions,
		"favorited":             d.Favorited,
		"views":                 d.Views,
		"lifetimeSubscriptions": d.LifetimeSubscriptions,
		"lifetimeFavorited":     d.LifetimeFavorited,
		"tags":                  d.Tags,
		"fileUrl":               d.FileURL,
	}
	return out, nil
}

// AddSubscription 将单个创意工坊模组加入订阅库（模组订阅 tab 的订阅入口，不下载）
func AddSubscription(modid string) (bool, error) {
	details, err := mod.GetPublishedFileDetails([]string{modid})
	if err != nil {
		return false, err
	}
	if len(details) == 0 {
		return false, errors.New("未找到该创意工坊条目")
	}
	d := details[0]
	rec := model.ModSubscription{
		Modid:       d.Publishedfileid,
		Name:        d.Title,
		Img:         d.PreviewURL,
		TimeUpdated: int64(d.TimeUpdated),
		SyncedAt:    time.Now().Unix(),
	}
	db := database.DB
	var exist model.ModSubscription
	if err := db.Where("modid = ?", rec.Modid).First(&exist).Error; err == nil {
		rec.ID = exist.ID
		rec.CreatedAt = exist.CreatedAt
	}
	if err := db.Save(&rec).Error; err != nil {
		return false, err
	}
	return true, nil
}

// SyncMySubscriptions 拉取订阅并入库（仅保留 DST 322330 模组），返回入库数量
func SyncMySubscriptions(steamID64, cookie string) (int, error) {
	ids, fetchErr := FetchMySubscriptions(steamID64, cookie)
	log.Printf("[subs] fetched %d ids, err=%v", len(ids), fetchErr)
	if fetchErr != nil && len(ids) == 0 {
		return 0, fetchErr
	}
	details, err := mod.GetPublishedFileDetails(ids)
	log.Printf("[subs] details %d, err=%v", len(details), err)
	if err != nil {
		return 0, err
	}
	db := database.DB
	now := time.Now().Unix()
	saved := 0
	for _, d := range details {
		if d.ConsumerAppID != 322330 {
			continue
		}
		rec := model.ModSubscription{
			Modid:       d.Publishedfileid,
			Name:        d.Title,
			Img:         d.PreviewURL,
			TimeUpdated: int64(d.TimeUpdated),
			SyncedAt:    now,
		}
		var exist model.ModSubscription
		if err := db.Where("modid = ?", rec.Modid).First(&exist).Error; err == nil {
			rec.ID = exist.ID
			rec.CreatedAt = exist.CreatedAt
		}
		if err := db.Save(&rec).Error; err == nil {
			saved++
		}
	}
	// 镜像语义：同步即与 Steam 订阅对齐，不在最新集合中的记录删除（否则取消订阅后本地残留）。
	// 仅在抓取完整（无页失败）时清理，避免代理丢页误删有效记录。
	if len(details) > 0 && fetchErr == nil {
		keep := make([]string, 0, len(details))
		for _, d := range details {
			if d.ConsumerAppID == 322330 {
				keep = append(keep, d.Publishedfileid)
			}
		}
		if err := db.Where("modid NOT IN ?", keep).Unscoped().Delete(&model.ModSubscription{}).Error; err != nil {
			log.Printf("[subs] 清理失效记录失败: %v", err)
		}
	}
	return saved, nil
}

package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/utils/dstConfigUtils"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type SteamSyncApi struct{}

// extractSteamID64 从 Steam 个人主页链接取 steamid64（/id/ 昵称链接无法直接解析，返回空）
func extractSteamID64(profileURL string) string {
	m := regexp.MustCompile(`steamcommunity\.com/profiles/(\d{17})`).FindStringSubmatch(profileURL)
	if m == nil {
		return ""
	}
	return m[1]
}

var workshopIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sharedfiles/filedetails/\?id=(\d+)`),
	regexp.MustCompile(`data-publishedfileid="(\d+)"`),
}

// extractWorkshopIDs 从创意工坊页面 HTML 提取模组 ID（去重）
func extractWorkshopIDs(html string) []string {
	seen := map[string]bool{}
	var ids []string
	for _, re := range workshopIDPatterns {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
	}
	return ids
}

func kvGet(key string) string {
	kv := model.KV{}
	database.DB.Where("key = ?", key).Find(&kv)
	return kv.Value
}

// Sync 一键同步当前 Steam 账号订阅的饥荒模组到统一模组目录（SSE 进度）
func (s *SteamSyncApi) Sync(ctx *gin.Context) {
	profile := strings.TrimSpace(kvGet("steam_profile"))
	steamID := extractSteamID64(profile)
	if steamID == "" && profile != "" {
		// 纯数字按 steamid64 处理
		if regexp.MustCompile(`^\d{17}$`).MatchString(profile) {
			steamID = profile
		}
	}
	if steamID == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在个人信息中填写 Steam 主页链接（steamcommunity.com/profiles/<17位数字>）", "data": nil})
		return
	}
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "请先在系统设置中选择根目录", "data": nil})
		return
	}

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	eventCh := make(chan string, 64)
	go func() {
		runSteamSync(cfg, steamID, eventCh)
		close(eventCh)
	}()
	for line := range eventCh {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
}

// Check 校验 Steam 主页链接：可解析且资料公开时返回订阅模组数量
func (s *SteamSyncApi) Check(ctx *gin.Context) {
	profile := strings.TrimSpace(ctx.Query("profile"))
	steamID := extractSteamID64(profile)
	if steamID == "" && regexp.MustCompile(`^\d{17}$`).MatchString(profile) {
		steamID = profile
	}
	if steamID == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "链接格式不对，需要 steamcommunity.com/profiles/<17位数字>", "data": nil})
		return
	}
	url := fmt.Sprintf("https://steamcommunity.com/profiles/%s/myworkshopfiles/?appid=322330&browsefilter=mysubscriptions", steamID)
	body, err := httpGet(url)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"code": 400, "msg": "无法访问该主页，资料需设为公开", "data": nil})
		return
	}
	ids := extractWorkshopIDs(body)
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": gin.H{"steamID": steamID, "count": len(ids)}})
}

func runSteamSync(cfg dstConfigUtils.DstConfig, steamID string, eventCh chan string) {
	send := func(s string) { eventCh <- s }

	send("读取 Steam 订阅列表（" + steamID + "）...")
	ids := []string{}
	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf("https://steamcommunity.com/profiles/%s/myworkshopfiles/?appid=322330&browsefilter=mysubscriptions&p=%d", steamID, page)
		body, err := httpGet(url)
		if err != nil {
			send("[失败] 获取订阅列表失败（资料需公开）: " + err.Error())
			return
		}
		pageIDs := extractWorkshopIDs(body)
		if len(pageIDs) == 0 {
			break
		}
		ids = append(ids, pageIDs...)
	}
	if len(ids) == 0 {
		send("未找到订阅的饥荒模组（确认资料公开且有 322330 模组订阅）")
		return
	}
	send(fmt.Sprintf("共 %d 个订阅模组", len(ids)))

	steamcmdDir := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
	modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
	exe := steamcmdExePath(steamcmdDir)
	if _, err := os.Stat(exe); err != nil {
		send("[失败] 未找到 SteamCMD，请先在系统设置使用「一键下载系统组件」")
		return
	}

	installed, downloaded, failed := 0, 0, 0
	for i, id := range ids {
		target := filepath.Join(modsRoot, id)
		if dirExists(target) {
			installed++
			continue
		}
		send(fmt.Sprintf("[%d/%d] 下载模组 %s ...", i+1, len(ids), id))
		cmd := exec.Command(exe, "+login", "anonymous", "+workshop_download_item", "322330", id, "+quit")
		cmd.Dir = steamcmdDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			failed++
			send(fmt.Sprintf("[失败] %s: %s", id, tailLines(string(out), 2)))
			continue
		}
		// SteamCMD 下载落在 steamcmd/steamapps/workshop 下，归位到统一模组目录
		src := filepath.Join(steamcmdDir, "steamapps", "workshop", "content", "322330", id)
		if !dirExists(src) {
			failed++
			send(fmt.Sprintf("[失败] %s: 下载后未找到文件", id))
			continue
		}
		if err := os.MkdirAll(modsRoot, os.ModePerm); err != nil {
			failed++
			send("[失败] 创建模组目录: " + err.Error())
			continue
		}
		if err := os.Rename(src, target); err != nil {
			failed++
			send(fmt.Sprintf("[失败] %s 归位: %s", id, err.Error()))
			continue
		}
		downloaded++
	}
	send(fmt.Sprintf("完成：新下载 %d，已存在 %d，失败 %d", downloaded, installed, failed))
}

func httpGet(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (dst-admin-go)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

package api

import (
	"dst-admin-go/config/database"
	"dst-admin-go/model"
	"dst-admin-go/service"
	"dst-admin-go/utils/clusterUtils"
	"dst-admin-go/utils/dstUtils"
	"dst-admin-go/vo"
	"encoding/json"
	"net/http"
	"os"
	"regexp"

	"github.com/gin-gonic/gin"
)

// 我的订阅（Steam Workshop 资产库）：同步/列表/删除。
// 列表标记"当前存档已启用"（modoverrides.lua enabled=true）；不可配置/启用（属存档模组）。

type SubscriptionApi struct{}

// enabledInCurrent 读当前存档各世界 modoverrides，返回启用的 modid 集合（workshop-<id> 或 <id>）。
// 按 workshop 条目分段取段内最后一个 enabled 值：块内 configuration_options 是嵌套花括号，
// 不能用 [^}]* 跨块匹配（会永远匹配不到）
func enabledInCurrent(clusterName string) map[string]bool {
	enabled := map[string]bool{}
	levels := []string{"Master", "Caves", "Shipwrecked", "Volcano"}
	reID := regexp.MustCompile(`\["workshop-(\d+)"\]`)
	reEn := regexp.MustCompile(`enabled\s*=\s*(true|false)`)
	for _, lv := range levels {
		p := dstUtils.GetLevelModoverridesPath(clusterName, lv)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(b)
		locs := reID.FindAllStringSubmatchIndex(s, -1)
		for i, loc := range locs {
			start := loc[1]
			end := len(s)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			ms := reEn.FindAllStringSubmatch(s[start:end], -1)
			if len(ms) > 0 && ms[len(ms)-1][1] == "true" {
				enabled[s[loc[2]:loc[3]]] = true
			}
		}
	}
	return enabled
}

// Sync POST /api/mod/subscription/sync — body {cookie, steamid}：抓订阅入库（仅 DST 模组）
func (s *SubscriptionApi) Sync(ctx *gin.Context) {
	var body struct {
		Cookie  string `json:"cookie"`
		SteamID string `json:"steamid"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "参数错误"})
		return
	}
	if body.Cookie == "" || body.SteamID == "" {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "cookie 与 steamid 必填"})
		return
	}
	// 兼容：粘贴整条 curl 命令时从中解析 cookie
	body.Cookie = service.ExtractCookie(body.Cookie)
	if body.Cookie == "" {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "未能从输入中解析出 Cookie"})
		return
	}
	saved, err := service.SyncMySubscriptions(body.SteamID, body.Cookie)
	if err != nil && saved == 0 {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "同步失败: " + err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: map[string]interface{}{"count": saved}})
}

// Add POST /api/mod/subscription/add — 模组订阅 tab 的订阅操作：加入订阅库（不下载）
func (s *SubscriptionApi) Add(ctx *gin.Context) {
	var body struct {
		Modid string `json:"modid"`
	}
	if err := ctx.BindJSON(&body); err != nil || body.Modid == "" {
		ctx.JSON(http.StatusOK, vo.Response{Code: 400, Msg: "参数错误"})
		return
	}
	if _, err := service.AddSubscription(body.Modid); err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success"})
}

// Detail GET /api/mod/subscription/detail/:id — 工坊实时详情 + 本地文件信息 + 各世界启用情况
func (s *SubscriptionApi) Detail(ctx *gin.Context) {
	modid := ctx.Param("id")
	data, err := service.GetSubscriptionDetail(modid)
	if err != nil {
		// 连不上 Steam 时要给明确的失败响应，不能像以前那样挂 1~2 分钟后回 200 + 空数据
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: "获取创意工坊详情失败: " + err.Error()})
		return
	}
	data["enabledWorlds"] = enabledWorldsFor(clusterUtilsFromCtx(ctx), modid)
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: data})
}

// enabledWorldsFor 返回启用该模组的世界名列表（当前存档各 level 的 modoverrides）
func enabledWorldsFor(clusterName, modid string) []string {
	var worlds []string
	reEn := regexp.MustCompile(`\["workshop-` + modid + `"\]\s*=\s*\{[^}]*?enabled\s*=\s*true`)
	reAny := regexp.MustCompile(`\["workshop-` + modid + `"\]`)
	for _, lv := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
		b, err := os.ReadFile(dstUtils.GetLevelModoverridesPath(clusterName, lv))
		if err != nil {
			continue
		}
		s := string(b)
		// 嵌套花括号：按条目分段取最后一个 enabled（与 enabledInCurrent 同法）
		if reAny.MatchString(s) {
			loc := reAny.FindStringIndex(s)
			body := s[loc[1]:]
			if n := reAny.FindStringIndex(body); n != nil {
				body = body[:n[0]]
			}
			ms := regexp.MustCompile(`enabled\s*=\s*(true|false)`).FindAllStringSubmatch(body, -1)
			if (len(ms) > 0 && ms[len(ms)-1][1] == "true") || reEn.MatchString(s) {
				worlds = append(worlds, lv)
			}
		}
	}
	return worlds
}

// List GET /api/mod/subscription — 订阅列表（含当前存档启用标记、系统模组库已下载标记）
func (s *SubscriptionApi) List(ctx *gin.Context) {
	cluster := clusterUtilsFromCtx(ctx)
	enabled := enabledInCurrent(cluster)
	downloaded := map[string]bool{}
	lib := map[string]model.ModInfo{}
	var libRows []model.ModInfo
	database.DB.Find(&libRows)
	for _, m := range libRows {
		downloaded[m.Modid] = true
		lib[m.Modid] = m
	}
	var rows []model.ModSubscription
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	type item struct {
		model.ModSubscription
		EnabledInCurrent bool                   `json:"enabledInCurrent"`
		Downloaded       bool                   `json:"downloaded"`
		Auth             string                 `json:"auth"`
		Description      string                 `json:"description"`
		V                string                 `json:"v"`
		ModConfig        map[string]interface{} `json:"mod_config"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		it := item{ModSubscription: r, EnabledInCurrent: enabled[r.Modid], Downloaded: downloaded[r.Modid]}
		if m, ok := lib[r.Modid]; ok {
			it.Auth = m.Auth
			it.Description = m.Description
			it.V = m.V
			_ = json.Unmarshal([]byte(m.ModConfig), &it.ModConfig)
		}
		out = append(out, it)
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success", Data: out})
}

// Delete DELETE /api/mod/subscription/:id — 删除订阅记录（不触碰 Steam 订阅状态）
func (s *SubscriptionApi) Delete(ctx *gin.Context) {
	modid := ctx.Param("id")
	deleted, err := service.DeleteSubscriptionRecord(modid)
	if err != nil {
		ctx.JSON(http.StatusOK, vo.Response{Code: 500, Msg: err.Error()})
		return
	}
	if !deleted {
		ctx.JSON(http.StatusOK, vo.Response{Code: 404, Msg: "记录不存在"})
		return
	}
	ctx.JSON(http.StatusOK, vo.Response{Code: 200, Msg: "success"})
}

// helper：从 gin ctx 取集群名（与 clusterUtils 同语义，避免循环依赖）
func clusterUtilsFromCtx(ctx *gin.Context) string {
	if c := ctx.GetHeader("Cluster"); c != "" {
		return c
	}
	return clusterUtils.GetClusterFromGin(ctx).ClusterName
}

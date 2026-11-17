package mod

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dst-admin-go/utils/dstConfigUtils"
	"dst-admin-go/utils/dstUtils"
)

// 游戏侧 appworkshop_322330.acf 登记。
//
// 背景:游戏判定模组"已有/需下载"只看 ACF 的 WorkshopItemsInstalled 登记,与磁盘上有没有
// 文件无关。面板自己下载归位的模组(SteamCMD 搬运 / CDN zip 解压)不会写这份登记,游戏就判
// "queued to download";下载又因 steamclient 缺库布局(Staging library folder not found)而失败,
// 于是模组整个不启用 -> 世界 prefab(如火山的 volcanoworld)缺失 -> 启动崩溃。
// 这里做三件事:登记/刷新 ACF、启动前自愈式补登记、补齐 steamclient 下载所需的库布局。

// AcfItem WorkshopItemsInstalled 单条登记记录
type AcfItem struct {
	Size        int64
	TimeUpdated int64
	Manifest    string
	Ugchandle   string
}

// ---- 极简 KeyValues(VDF) 解析/序列化,仅覆盖 appworkshop_*.acf 用到的形态 ----

type kvNode struct {
	key      string
	val      string
	isBlock  bool
	children []*kvNode
}

type kvParser struct {
	s string
	i int
}

func (p *kvParser) skipWS() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *kvParser) parseString() (string, error) {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		return "", errors.New("acf: 期望字符串")
	}
	p.i++
	var sb strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '\\' && p.i+1 < len(p.s) {
			sb.WriteByte(p.s[p.i+1])
			p.i += 2
			continue
		}
		if c == '"' {
			p.i++
			return sb.String(), nil
		}
		sb.WriteByte(c)
		p.i++
	}
	return "", errors.New("acf: 字符串未闭合")
}

// parseKVChildren 解析块内条目,消费到配对的 '}'
func (p *kvParser) parseKVChildren() ([]*kvNode, error) {
	var out []*kvNode
	for {
		p.skipWS()
		if p.i >= len(p.s) {
			return out, nil
		}
		if p.s[p.i] == '}' {
			p.i++
			return out, nil
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.i < len(p.s) && p.s[p.i] == '{' {
			p.i++
			ch, err := p.parseKVChildren()
			if err != nil {
				return nil, err
			}
			out = append(out, &kvNode{key: key, isBlock: true, children: ch})
			continue
		}
		val, err := p.parseString()
		if err != nil {
			return nil, err
		}
		out = append(out, &kvNode{key: key, val: val})
	}
}

// ParseKV 解析整个 VDF 文件,返回根节点(key=根名,isBlock)
func ParseKV(data string) (*kvNode, error) {
	p := &kvParser{s: data}
	p.skipWS()
	key, err := p.parseString()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.i >= len(p.s) || p.s[p.i] != '{' {
		return nil, errors.New("acf: 期望根块")
	}
	p.i++
	ch, err := p.parseKVChildren()
	if err != nil {
		return nil, err
	}
	return &kvNode{key: key, isBlock: true, children: ch}, nil
}

func writeKVChildren(sb *strings.Builder, children []*kvNode, indent int) {
	pad := strings.Repeat("\t", indent)
	for _, n := range children {
		if n.isBlock {
			sb.WriteString(pad + "\"" + n.key + "\"\n" + pad + "{\n")
			writeKVChildren(sb, n.children, indent+1)
			sb.WriteString(pad + "}\n")
			continue
		}
		sb.WriteString(pad + "\"" + n.key + "\"\t\t\"" + n.val + "\"\n")
	}
}

// WriteKV 序列化为 Steam 可读的 VDF
func WriteKV(root *kvNode) string {
	var sb strings.Builder
	sb.WriteString("\"" + root.key + "\"\n{\n")
	writeKVChildren(&sb, root.children, 1)
	sb.WriteString("}\n")
	return sb.String()
}

func kvChild(n *kvNode, key string) *kvNode {
	for _, c := range n.children {
		if c.key == key {
			return c
		}
	}
	return nil
}

func kvSet(n *kvNode, key, val string) {
	if c := kvChild(n, key); c != nil {
		c.val = val
		return
	}
	n.children = append(n.children, &kvNode{key: key, val: val})
}

// ---- ACF 读写 ----

// gameAcfPaths 游戏侧 ACF 路径:统一 ugc 目录一个;legacy 按存档/世界各一个
func gameAcfPaths() []string {
	cfg := dstConfigUtils.GetDstConfig()
	if dstConfigUtils.UnifiedUgcDir(cfg) != "" {
		return []string{dstUtils.GetUgcAcfPath("", "")}
	}
	root := dstUtils.GetUgcModPath()
	var out []string
	matches, _ := filepath.Glob(filepath.Join(root, "*", "*", "appworkshop_322330.acf"))
	out = append(out, matches...)
	return out
}

// ReadAcfItem 读取指定段(WorkshopItemsInstalled / WorkshopItemDetails)中某模组的记录
func ReadAcfItem(acfPath, section, modid string) (AcfItem, bool) {
	data, err := os.ReadFile(acfPath)
	if err != nil {
		return AcfItem{}, false
	}
	root, err := ParseKV(string(data))
	if err != nil {
		return AcfItem{}, false
	}
	sec := kvChild(root, section)
	if sec == nil {
		return AcfItem{}, false
	}
	entry := kvChild(sec, modid)
	if entry == nil {
		return AcfItem{}, false
	}
	it := AcfItem{}
	if c := kvChild(entry, "size"); c != nil {
		it.Size, _ = strconv.ParseInt(c.val, 10, 64)
	}
	if c := kvChild(entry, "timeupdated"); c != nil {
		it.TimeUpdated, _ = strconv.ParseInt(c.val, 10, 64)
	}
	if c := kvChild(entry, "manifest"); c != nil {
		it.Manifest = c.val
	}
	if c := kvChild(entry, "ugchandle"); c != nil {
		it.Ugchandle = c.val
	}
	return it, true
}

// ReadAcfDetail 读 WorkshopItemDetails(游戏查询缓存),取 latest_* 优先
func ReadAcfDetail(acfPath, modid string) (AcfItem, bool) {
	data, err := os.ReadFile(acfPath)
	if err != nil {
		return AcfItem{}, false
	}
	root, err := ParseKV(string(data))
	if err != nil {
		return AcfItem{}, false
	}
	sec := kvChild(root, "WorkshopItemDetails")
	if sec == nil {
		return AcfItem{}, false
	}
	entry := kvChild(sec, modid)
	if entry == nil {
		return AcfItem{}, false
	}
	get := func(keys ...string) string {
		for _, k := range keys {
			if c := kvChild(entry, k); c != nil && c.val != "" {
				return c.val
			}
		}
		return ""
	}
	it := AcfItem{}
	it.Size, _ = strconv.ParseInt(get("size"), 10, 64)
	it.TimeUpdated, _ = strconv.ParseInt(get("latest_timeupdated", "timeupdated"), 10, 64)
	it.Manifest = get("latest_manifest", "manifest")
	it.Ugchandle = get("ugchandle")
	return it, true
}

// UpsertWorkshopInstalled 把模组写入/更新到 WorkshopItemsInstalled,其余内容原样保留。
// ACF 文件不存在时创建最小骨架(全新环境游戏未首启也能登记)。
func UpsertWorkshopInstalled(acfPath, modid string, item AcfItem) error {
	var root *kvNode
	data, err := os.ReadFile(acfPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		root = &kvNode{key: "AppWorkshop", isBlock: true, children: []*kvNode{
			{key: "appid", val: "322330"},
		}}
	} else {
		root, err = ParseKV(string(data))
		if err != nil {
			return err
		}
	}
	installed := kvChild(root, "WorkshopItemsInstalled")
	if installed == nil {
		installed = &kvNode{key: "WorkshopItemsInstalled", isBlock: true}
		root.children = append(root.children, installed)
	}
	entry := kvChild(installed, modid)
	if entry == nil {
		entry = &kvNode{key: modid, isBlock: true}
		installed.children = append(installed.children, entry)
	}
	kvSet(entry, "size", strconv.FormatInt(item.Size, 10))
	kvSet(entry, "timeupdated", strconv.FormatInt(item.TimeUpdated, 10))
	kvSet(entry, "manifest", item.Manifest)
	if item.Ugchandle != "" {
		kvSet(entry, "ugchandle", item.Ugchandle)
	}
	if err := os.MkdirAll(filepath.Dir(acfPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(acfPath, []byte(WriteKV(root)), 0644)
}

// RegisterInstalled 向游戏侧所有 ACF 登记一条安装记录(面板下载归位后调用)。
// 全部路径写失败才返回 error(部分成功 log 后算过),不再静默吞。
func RegisterInstalled(modid string, item AcfItem) error {
	paths := gameAcfPaths()
	if len(paths) == 0 {
		return errors.New("无法确定游戏侧 ACF 路径(配置不完整)")
	}
	okCount := 0
	for _, p := range paths {
		if err := UpsertWorkshopInstalled(p, modid, item); err != nil {
			log.Println("[acf] 登记失败", modid, p, err)
			continue
		}
		log.Println("[acf] 已登记", modid, "manifest=", item.Manifest, "ugchandle=", item.Ugchandle)
		okCount++
	}
	if okCount == 0 {
		return errors.New("游戏侧 ACF 登记全部失败")
	}
	return nil
}

// RegisterInstalledBest CDN 安装路径没有 SteamCMD 的 manifest,优先取游戏自己的查询缓存
// (WorkshopItemDetails),其次用 CDN URL 里的 ugchandle 兜底,最后按 manifest=-1 登记。
func RegisterInstalledBest(modid, dir, fileURL string, timeUpdated int64) error {
	item := AcfItem{Size: dirSizeOnDisk(dir), TimeUpdated: timeUpdated}
	for _, p := range gameAcfPaths() {
		if d, ok := ReadAcfDetail(p, modid); ok && d.Manifest != "" {
			item.Manifest = d.Manifest
			item.Ugchandle = d.Ugchandle
			if d.TimeUpdated > 0 {
				item.TimeUpdated = d.TimeUpdated
			}
			break
		}
	}
	if item.Manifest == "" {
		item.Ugchandle = UgchandleFromURL(fileURL)
		item.Manifest = "-1"
	}
	return RegisterInstalled(modid, item)
}

// steamcmdInstalledEntry 从 SteamCMD 的 ACF 读取刚下载完成的安装记录(带真实 manifest)
func steamcmdInstalledEntry(steamcmdDir, modid string) (AcfItem, bool) {
	return ReadAcfItem(filepath.Join(steamcmdDir, "steamapps", "workshop", "appworkshop_322330.acf"), "WorkshopItemsInstalled", modid)
}

// dirSizeOnDisk 目录占用字节数(ACF 的 size 字段)
func dirSizeOnDisk(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// UgchandleFromURL 从 cdn.steamusercontent.com/ugc/<handle>/<hash>/ 提取 ugchandle
func UgchandleFromURL(fileURL string) string {
	const marker = "/ugc/"
	i := strings.Index(fileURL, marker)
	if i < 0 {
		return ""
	}
	rest := fileURL[i+len(marker):]
	if j := strings.IndexAny(rest, "/?&"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// WriteLegacyBin 把 legacy(旧式工坊)模组的原始 zip 落成 <ugchandle>_legacy.bin。
// 游戏对这类模组每次启动都走"下载→解包 <ugchandle>_legacy.bin"流程,该文件缺失时
// 解包失败(UMFD failed to open file),模组整个不启用。面板安装/修复时预置此文件,
// 游戏解包即成功。zipBytes 为模组原始文件(CDN file_url 下载的内容),须与磁盘模组内容同版本。
func WriteLegacyBin(modDir, ugchandle string, zipBytes []byte) error {
	if ugchandle == "" || len(zipBytes) == 0 {
		return errors.New("legacy bin 参数不完整")
	}
	dest := filepath.Join(modDir, ugchandle+"_legacy.bin")
	return os.WriteFile(dest, zipBytes, 0644)
}

// steamappsRoot 返回 <mod_download_path>/steamapps
func steamappsRoot() string {
	cfg := dstConfigUtils.GetDstConfig()
	return filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps")
}

// EnsureSteamLibraryLayout 补齐 steamclient 工坊下载所需的库布局。
// 缺 libraryfolders.vdf 时下载报 "Staging library folder not found / Install library folder not found"
// 直接失败,待下载模组永远不会启用。
func EnsureSteamLibraryLayout() error {
	root := steamappsRoot() // <library>/steamapps
	lib := filepath.Dir(root)
	vdf := filepath.Join(root, "libraryfolders.vdf")
	for _, d := range []string{
		root,
		filepath.Join(root, "staging"),
		filepath.Join(root, "temp"),
		filepath.Join(root, "downloading"),
		filepath.Join(root, "workshop", "content"),
		filepath.Join(root, "workshop", "downloads"),
		filepath.Join(root, "workshop", "temp"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(vdf); err == nil {
		return nil
	}
	// 与 steamcmd 的 libraryfolders.vdf 同构,path 指向库根
	content := "\"libraryfolders\"\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"" +
		strings.ReplaceAll(lib, `\`, `\\`) +
		"\"\n\t\t\"label\"\t\t\"\"\n\t\t\"contentid\"\t\t\"0\"\n\t\t\"totalsize\"\t\t\"0\"\n\t\t\"update_clean_bytes_tally\"\t\t\"0\"\n\t\t\"time_last_update_verified\"\t\t\"0\"\n\t\t\"apps\"\n\t\t{\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile(vdf, []byte(content), 0644); err != nil {
		return err
	}
	log.Println("[acf] 已补齐库布局:", vdf)
	return nil
}

// EnsureGameAcfEntries 自愈式补登记:本地模组目录完整但 ACF 缺登记/登记过期时,用
// WorkshopItemDetails(游戏自己的查询缓存)或 SteamCMD 记录补上。幂等,可反复调用。
// 返回补登记数量。
func EnsureGameAcfEntries() int {
	cfg := dstConfigUtils.GetDstConfig()
	modsRoot := filepath.Join(dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod), "steamapps", "workshop", "content", "322330")
	entries, err := os.ReadDir(modsRoot)
	if err != nil {
		return 0
	}
	fixed := 0
	for _, acfPath := range gameAcfPaths() {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			modid := e.Name()
			dir := filepath.Join(modsRoot, modid)
			if !modDirComplete(dir) {
				continue
			}
			installed, hasInstalled := ReadAcfItem(acfPath, "WorkshopItemsInstalled", modid)
			detail, hasDetail := ReadAcfDetail(acfPath, modid)

			item := AcfItem{Size: dirSizeOnDisk(dir)}
			switch {
			case hasDetail && detail.Manifest != "":
				item.Manifest = detail.Manifest
				item.TimeUpdated = detail.TimeUpdated
				item.Ugchandle = detail.Ugchandle
			case hasInstalled && installed.Manifest != "" && installed.Manifest != "-1":
				continue // 已有有效登记,不动
			default:
				if sc, ok := steamcmdInstalledEntry(steamcmdRoot(), modid); ok && (sc.Manifest != "" && sc.Manifest != "-1" || sc.Ugchandle != "") {
					item.Manifest = sc.Manifest
					item.TimeUpdated = sc.TimeUpdated
					item.Ugchandle = sc.Ugchandle
				} else if hasInstalled && installed.Ugchandle != "" {
					item.Manifest = installed.Manifest
					item.TimeUpdated = installed.TimeUpdated
					item.Ugchandle = installed.Ugchandle
				} else {
					log.Println("[acf] 缺少版本信息,跳过登记:", modid)
					continue
				}
			}
			if item.Manifest == "" {
				item.Manifest = "-1"
			}
			// 已登记且一致则不动
			if hasInstalled && installed.Manifest == item.Manifest && installed.Ugchandle == item.Ugchandle {
				continue
			}
			if err := UpsertWorkshopInstalled(acfPath, modid, item); err != nil {
				log.Println("[acf] 补登记失败", modid, err)
				continue
			}
			log.Println("[acf] 补登记", modid, "manifest=", item.Manifest, "ugchandle=", item.Ugchandle)
			fixed++
		}
	}
	return fixed
}

// verifyGameReady 下载收尾验收:确认游戏侧状态就绪——ACF 登记有效(manifest≠-1),
// 或 manifest=-1 且 <ugchandle>_legacy.bin 在位(游戏走解包流程能成功)。
// 不满足返回带原因的 error,调用方应整体报错,不留"面板说好了、游戏不认"的灰色状态。
func verifyGameReady(modid, modDir string) error {
	paths := gameAcfPaths()
	if len(paths) == 0 {
		return errors.New("无法确定游戏侧 ACF 路径(配置不完整),无法保证游戏可启用该模组")
	}
	for _, p := range paths {
		it, ok := ReadAcfItem(p, "WorkshopItemsInstalled", modid)
		if !ok {
			return errors.New("游戏侧 ACF 未登记(" + p + "),游戏会判待下载而不启用")
		}
		if it.Manifest != "" && it.Manifest != "-1" {
			continue // 真 manifest,游戏直接认领
		}
		if it.Ugchandle != "" && fileExists(filepath.Join(modDir, it.Ugchandle+"_legacy.bin")) {
			continue // legacy 流程,解包素材在位
		}
		return errors.New("ACF 登记无效(manifest=-1)且缺 " + it.Ugchandle + "_legacy.bin(" + p + "),游戏解包会失败(UMFD)")
	}
	return nil
}

// verifyGameReadyFn 验收入口(可替换,便于测试收尾接线)
var verifyGameReadyFn = verifyGameReady

// steamcmdRoot SteamCMD 安装目录
func steamcmdRoot() string {
	cfg := dstConfigUtils.GetDstConfig()
	return dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
}

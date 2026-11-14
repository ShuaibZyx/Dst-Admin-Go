package api

import (
	"archive/zip"
	mod2 "dst-admin-go/service/mod"
	"dst-admin-go/utils/dstConfigUtils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type OneClickInstallApi struct{}

var oneclickSession = NewLiveSession()

// oneclickTargets 一键下载的目标目录（全部来自配置）
func oneclickTargets(cfg dstConfigUtils.DstConfig) (steamcmdDir, serverDir, modDir string) {
	steamcmdDir = dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
	serverDir = dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Force_install_dir, dstConfigUtils.DefaultServerDir)
	if cfg.Beta == 1 {
		serverDir += "-beta"
	}
	modDir = dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Mod_download_path, dstConfigUtils.DefaultMod)
	return steamcmdDir, serverDir, modDir
}

// Install 一键下载系统组件：建目录 → 装 SteamCMD（如缺）→ SteamCMD 下载/更新服务器 → 模组目录就绪
func (o *OneClickInstallApi) Install(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	if cfg.Parent_path == "" {
		ctx.JSON(http.StatusOK, voResponse(400, "请先在系统设置中选择根目录"))
		return
	}
	steamcmdDir, serverDir, modDir := oneclickTargets(cfg)

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")

	oneclickSession.Start(func(send func(string)) {
		runOneClick(cfg, steamcmdDir, serverDir, modDir, send)
	})
	// 已在运行则本次仅"接上"：历史 + 实时跟随，不重复启动
	backlog, ch, _ := oneclickSession.Attach()
	for _, line := range backlog {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
	for line := range ch {
		_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", line)
		ctx.Writer.Flush()
	}
	log.Println("oneclick install stream end")
}

// voResponse 简单错误响应（避免在 SSE 头之后写 JSON 的场景）
func voResponse(code int, msg string) gin.H {
	return gin.H{"code": code, "msg": msg, "data": nil}
}

func runOneClick(cfg dstConfigUtils.DstConfig, steamcmdDir, serverDir, modDir string, send func(string)) {

	// 1. 目录（模组目录在下载模组时创建）
	_ = modDir
	send("创建目录： " + steamcmdDir + " / " + serverDir)
	for _, d := range []string{steamcmdDir, serverDir} {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			send("[失败] 创建目录 " + d + " : " + err.Error())
			return
		}
	}

	// 2. SteamCMD（缺失才下载）
	steamcmdExe := steamcmdExePath(steamcmdDir)
	if _, err := os.Stat(steamcmdExe); err != nil {
		send("下载 SteamCMD ...")
		if err := installSteamcmdTo(steamcmdDir, send); err != nil {
			send("[失败] SteamCMD 下载/安装失败: " + err.Error())
			return
		}
		send("SteamCMD 安装完成")
	} else {
		send("SteamCMD 已存在，跳过下载")
	}

	// 3. 服务器下载/更新（SteamCMD app_update 343050）
	send("开始下载/更新饥荒专用服务器（这一步耗时较长，请耐心等待）...")
	betaArgs := []string{"+force_install_dir", serverDir, "+login", "anonymous", "+app_update", "343050"}
	if cfg.Beta == 1 {
		betaArgs = append(betaArgs, "-beta", "updatebeta")
	}
	betaArgs = append(betaArgs, "validate", "+quit")
	err := mod2.RunSteamcmdStream(steamcmdDir, betaArgs, send)
	if err != nil {
		// SteamCMD 首次会话偶发 Missing configuration，稍候重试一次
		send("[提示] 下载未成功，10 秒后自动重试一次...")
		time.Sleep(10 * time.Second)
		err = mod2.RunSteamcmdStream(steamcmdDir, betaArgs, send)
	}
	if err != nil {
		send("[失败] 服务器下载失败: " + err.Error())
		return
	}
	send("完成！SteamCMD 与专用服务器已就绪。")
}

func steamcmdExePath(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "steamcmd.exe")
	}
	if _, err := os.Stat(filepath.Join(dir, "steamcmd.sh")); err == nil {
		return filepath.Join(dir, "steamcmd.sh")
	}
	return filepath.Join(dir, "steamcmd")
}

// installSteamcmdTo 下载并解压 SteamCMD 到目标目录
func installSteamcmdTo(dir string, send func(string)) error {
	if runtime.GOOS == "windows" {
		zipPath := filepath.Join(os.TempDir(), "steamcmd.zip")
		if err := downloadFile("https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip", zipPath, send); err != nil {
			return err
		}
		defer os.Remove(zipPath)
		return unzipTo(zipPath, dir)
	}
	tarPath := filepath.Join(os.TempDir(), "steamcmd_linux.tar.gz")
	if err := downloadFile("https://steamcdn-a.akamaihd.com/installer/steamcmd_linux.tar.gz", tarPath, send); err != nil {
		return err
	}
	defer os.Remove(tarPath)
	cmd := exec.Command("tar", "-xzf", tarPath, "-C", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, tailLines(string(out), 3))
	}
	return nil
}

func downloadFile(url, dst string, send func(string)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, url)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	send(fmt.Sprintf("已下载 %.1f MB", float64(written)/1024/1024))
	return nil
}

func unzipTo(zipPath, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		p := filepath.Join(dst, filepath.FromSlash(f.Name))
		if rel, err := filepath.Rel(dst, p); err != nil || len(rel) > 0 && rel[0] == '.' {
			return fmt.Errorf("zip 路径非法: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, os.ModePerm); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(p)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func tailLines(s string, n int) string {
	lines := splitNonEmpty(s)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := ""
	for _, l := range lines {
		out += l + " | "
	}
	return out
}

func splitNonEmpty(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := trimSpace(s[start:i])
			if line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := trimSpace(s[start:])
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// InstallStatus 组件安装状态：SteamCMD 与专用服务器是否已就位
func (o *OneClickInstallApi) InstallStatus(ctx *gin.Context) {
	cfg := dstConfigUtils.GetDstConfig()
	sc, srv, _ := oneclickTargets(cfg)
	steamOK := false
	for _, n := range []string{"steamcmd.exe", "steamcmd.sh", "steamcmd"} {
		if info, err := os.Stat(filepath.Join(sc, n)); err == nil && !info.IsDir() {
			steamOK = true
			break
		}
	}
	serverOK := false
	for _, b := range []string{"bin", "bin64"} {
		entries, err := os.ReadDir(filepath.Join(srv, b))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "dontstarve_dedicated_server_nullrenderer") {
				serverOK = true
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success",
		"data": gin.H{"steamcmd": steamOK, "server": serverOK, "installing": oneclickSession.IsRunning()}})
}

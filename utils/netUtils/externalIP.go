// 直连代码 IP 解析三态（对应鸽子鸽"自定义外网 ip"+自动探测；官方文档 NAT/FRP 场景）
package netUtils

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	ModeAutoDetect = "auto_detect"
	ModeManual     = "manual"
	ModeBind       = "bind"
)

// ResolveIP 按模式解析对外 IP。
// manual: 使用给定 IP/域名；bind: 本机对外网卡地址；auto_detect: 探测公网出口（失败报错由调用方降级）。
func ResolveIP(mode, manualIP string) (string, error) {
	switch mode {
	case ModeManual:
		if strings.TrimSpace(manualIP) == "" {
			return "", errors.New("manual 模式需要 external_ip")
		}
		return strings.TrimSpace(manualIP), nil
	case ModeBind:
		return bindIP()
	case ModeAutoDetect:
		if ip, err := detectViaDnspod(); err == nil && ip != "" {
			return ip, nil
		}
		return detectViaHTTP()
	default:
		return "", fmt.Errorf("未知 IP 模式 %q", mode)
	}
}

func bindIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		addrs, aerr := net.InterfaceAddrs()
		if aerr == nil {
			for _, a := range addrs {
				if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
		return "", err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// detectViaDnspod 鸽子鸽同款技巧：向 DNSPod 发 GET，从响应观测源地址（失败回退）
func detectViaDnspod() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ns1.dnspod.net/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// 部分 HTTP 服务会在响应/头中回显源地址；否则走通用回退
	if xr := resp.Header.Get("X-Real-IP"); xr != "" {
		return xr, nil
	}
	return "", errors.New("dnspod 未回显源地址")
}

func detectViaHTTP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))
	if net.ParseIP(ip) == nil {
		return "", errors.New("ipify 返回非 IP")
	}
	return ip, nil
}

// FormatCConnect 生成游戏控制台直连命令（externalPort 为 NAT/FRP 映射后的端口）
func FormatCConnect(ip string, port int, password string) string {
	if password != "" {
		return fmt.Sprintf(`c_connect("%s", %d, "%s")`, ip, port, password)
	}
	return fmt.Sprintf(`c_connect("%s", %d)`, ip, port)
}

// FormatSteamURL 生成 steam:// 一键连接链接
func FormatSteamURL(ip string, port int, password string) string {
	base := fmt.Sprintf(`steam://rungameid/322330//+connect %s:%d`, ip, port)
	if password != "" {
		base += fmt.Sprintf(` +password "%s"`, password)
	}
	return base
}

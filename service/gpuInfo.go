package service

import (
	"os/exec"
	"strconv"
	"strings"
)

// GpuInfo 显存使用（单位：MB）。数据源：nvidia-smi 优先（精确），wmic 兜底（仅总量）。
type GpuInfo struct {
	UsedMB  float64 `json:"usedMB"`
	TotalMB float64 `json:"totalMB"`
}

// GetGpuInfo 采集显存使用。取不到返回零值（前端显示 --）。
// 数据源优先级:nvidia-smi(精确) > wmic(仅总量,旧版 Windows) >
// PowerShell CIM(仅总量,Win11 24H2+ 起 wmic 被移除,Get-CimInstance 恒在)。
func GetGpuInfo() GpuInfo {
	if info, ok := gpuFromNvidiaSmi(); ok {
		return info
	}
	if info := gpuFromWmic(); info.TotalMB > 0 {
		return info
	}
	return gpuFromPowerShell()
}

// gpuFromNvidiaSmi nvidia-smi --query-gpu=memory.used,memory.total（MB）
func gpuFromNvidiaSmi() (GpuInfo, bool) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return GpuInfo{}, false
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return GpuInfo{}, false
	}
	used, errU := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	total, errT := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errU != nil || errT != nil || total <= 0 {
		return GpuInfo{}, false
	}
	return GpuInfo{UsedMB: used, TotalMB: total}, true
}

// gpuFromWmic win32_VideoController.AdapterRAM（字节）——只拿到总量，使用量按未知（前端可显示总量）
func gpuFromWmic() GpuInfo {
	out, err := exec.Command("wmic", "path", "win32_VideoController", "get", "AdapterRAM").Output()
	if err != nil {
		return GpuInfo{}
	}
	if bytes, ok := parseFirstPositive(string(out)); ok {
		return GpuInfo{TotalMB: bytes / 1024 / 1024}
	}
	return GpuInfo{}
}

// gpuFromPowerShell wmic 的替代:Win11 24H2+ 移除了 wmic,但 Get-CimInstance 随 PowerShell 恒在。
// 同样只拿 AdapterRAM 总量（WMI 该字段本身有 32 位上限，与 wmic 时代一致，不算回归）。
func gpuFromPowerShell() GpuInfo {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_VideoController | ForEach-Object { $_.AdapterRAM }").Output()
	if err != nil {
		return GpuInfo{}
	}
	if bytes, ok := parseFirstPositive(string(out)); ok {
		return GpuInfo{TotalMB: bytes / 1024 / 1024}
	}
	return GpuInfo{}
}

// parseFirstPositive 按行取首个正数;容错跳过表头/空行/非数值行(wmic 有 AdapterRAM 表头)
func parseFirstPositive(out string) (float64, bool) {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		v := strings.TrimSpace(line)
		if v == "" {
			continue
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

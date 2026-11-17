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
func GetGpuInfo() GpuInfo {
	if info, ok := gpuFromNvidiaSmi(); ok {
		return info
	}
	return gpuFromWmic()
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
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		v := strings.TrimSpace(line)
		if v == "" || strings.EqualFold(v, "AdapterRAM") {
			continue
		}
		bytes, err := strconv.ParseFloat(v, 64)
		if err == nil && bytes > 0 {
			return GpuInfo{TotalMB: bytes / 1024 / 1024}
		}
	}
	return GpuInfo{}
}

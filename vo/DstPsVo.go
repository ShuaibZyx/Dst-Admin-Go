package vo

// import (
// 	"dst-admin-go/constant"
// 	"dst-admin-go/utils/systemUtils"
// )

type DstPsVo struct {
	CpuUage string `json:"cpuUage"`
	MemUage string `json:"memUage"`
	VSZ     string `json:"VSZ"`
	RSS     string `json:"RSS"`
	// Elapsed 进程运行时长文本(如 "2 小时 15 分");无进程为空,前端显示"未运行/未知"
	Elapsed string `json:"elapsed"`
}

func NewDstPsVo() *DstPsVo {
	return &DstPsVo{}
}

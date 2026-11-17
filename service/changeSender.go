package service

// ChangeSender 载荷变更检测:同内容只推一次。
// 用于 SSE 状态心跳(载荷含各层 Lua 全文约 20KB,重复推送既费带宽又挤前端渲染)。
type ChangeSender struct {
	last string
}

// ShouldPush 载荷与上次不同则放行;空载荷不推送。
func (c *ChangeSender) ShouldPush(payload string) bool {
	if payload == "" || payload == c.last {
		return false
	}
	c.last = payload
	return true
}

package service

import (
	"strings"
	"testing"
)

// 进度行格式:[进度:NN%] 文本(前端进度条从启动日志流解析),百分比夹取 0-100。
func TestStartProgressLine(t *testing.T) {
	if got := progressLine(32, "模组同步 3/8"); got != "[进度:32%] 模组同步 3/8" {
		t.Fatalf("got %q", got)
	}
	if got := progressLine(0, "准备启动"); got != "[进度:0%] 准备启动" {
		t.Fatalf("0%% 边界, got %q", got)
	}
	if got := progressLine(100, "已启动"); got != "[进度:100%] 已启动" {
		t.Fatalf("100%% 边界, got %q", got)
	}
	if got := progressLine(-5, "x"); got != "[进度:0%] x" {
		t.Fatalf("负数应夹取到 0, got %q", got)
	}
	if got := progressLine(150, "x"); got != "[进度:100%] x" {
		t.Fatalf("超百应夹取到 100, got %q", got)
	}
}

// 进度行经 Logf 落盘时不得被二次格式化污染(%] 会被 Sprintf 当动词吃掉)。
// 实况回归:打点一度输出 "[进度:5%!](MISSING)"。
func TestProgressNoFormatPollution(t *testing.T) {
	e := NewEventLog(10)
	e.Progress(32, "模组同步 %d/%d", 3, 8)
	last := e.lines[len(e.lines)-1]
	if strings.Contains(last, "%!") || strings.Contains(last, "MISSING") {
		t.Fatalf("进度行被 Logf 格式化污染: %q", last)
	}
	if !strings.Contains(last, "[进度:32%] 模组同步 3/8") {
		t.Fatalf("got %q", last)
	}
}

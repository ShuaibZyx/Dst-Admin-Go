package schedule

import (
	"errors"
	"testing"
)

// 定时"停服→更新→启动"任务:panic/失败也须确保启动,避免"已停服未启动"半成品状态。

func TestRunStopStartAlwaysStarts(t *testing.T) {
	cases := []struct {
		name string
		task func() error
	}{
		{"成功", func() error { return nil }},
		{"失败", func() error { return errors.New("更新失败") }},
		{"panic", func() error { panic("boom") }},
	}
	for _, tc := range cases {
		count := 0
		runStopStart(tc.task, func() { count++ })
		if count != 1 {
			t.Fatalf("%s: start 应恰好调用一次,实际 %d", tc.name, count)
		}
	}
}

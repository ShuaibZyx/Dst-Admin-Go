package schedule

import "log"

// runStopStart 执行"停服→更新→启动"定时任务:无论成功/失败/panic 均确保启动,
// 避免 UpdateGame 停服后中途出错留下"已停服未启动"半成品状态(cron 任务健壮性)。
func runStopStart(update func() error, start func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("定时更新任务 panic:", r)
		}
		start()
	}()
	if err := update(); err != nil {
		log.Println("更新失败:", err)
	}
}

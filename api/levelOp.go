package api

import "sync"

// levelOps 世界启停操作互斥表："cluster/level" -> 进行中的操作("start"/"stop")。
// 启停已异步化(API 秒回),同世界必须防重入——启动中再点启动/停止会让
// StopLevel/LaunchLevel 交叉打架(句柄/进程状态混乱)。
var levelOps sync.Map

// beginLevelOp 尝试开始操作;已有操作进行中返回 false
func beginLevelOp(key, op string) bool {
	_, loaded := levelOps.LoadOrStore(key, op)
	return !loaded
}

// endLevelOp 释放操作权
func endLevelOp(key string) {
	levelOps.Delete(key)
}

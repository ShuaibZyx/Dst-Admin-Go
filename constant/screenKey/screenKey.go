package screenKey

// Key screen 会话名。形参顺序与调用方传参一致 (clusterName, levelName)；
// 输出格式不变（历史格式 DST_8level_<cluster>_<level>，改了会话名会与运行中的 screen 会话失联）。
func Key(clusterName, levelName string) string {
	return "DST_8level_" + clusterName + "_" + levelName
}

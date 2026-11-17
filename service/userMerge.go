package service

// UserInfo 用户资料（password.txt 的 4 字段）
type UserInfo struct {
	Username    string
	Password    string
	DisplayName string
	PhotoURL    string
}

// MergeUserInfo 部分更新：空字段保留旧值——资料与密码分开保存时互不覆盖。
func MergeUserInfo(old, patch UserInfo) UserInfo {
	out := old
	if patch.Username != "" {
		out.Username = patch.Username
	}
	if patch.Password != "" {
		out.Password = patch.Password
	}
	if patch.DisplayName != "" {
		out.DisplayName = patch.DisplayName
	}
	if patch.PhotoURL != "" {
		out.PhotoURL = patch.PhotoURL
	}
	return out
}

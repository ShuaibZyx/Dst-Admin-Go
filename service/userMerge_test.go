package service

import "testing"

// 用户信息部分更新：空字段保留旧值（资料/密码分开保存时互不覆盖）。
func TestMergeUserInfo(t *testing.T) {
	old := UserInfo{Username: "u1", Password: "p1", DisplayName: "d1", PhotoURL: "a1"}

	// 只改密码（资料字段为空保留）
	got := MergeUserInfo(old, UserInfo{Password: "p2"})
	if got.Password != "p2" || got.Username != "u1" || got.DisplayName != "d1" || got.PhotoURL != "a1" {
		t.Fatalf("只改密码应保留其他字段, got %+v", got)
	}

	// 只改资料（密码为空保留）
	got = MergeUserInfo(old, UserInfo{DisplayName: "d2", PhotoURL: "a2"})
	if got.Password != "p1" || got.DisplayName != "d2" || got.PhotoURL != "a2" || got.Username != "u1" {
		t.Fatalf("只改资料应保留密码, got %+v", got)
	}
}

package service

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"dst-admin-go/config/database"
	"dst-admin-go/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Steam 订阅页 HTML 解析：两种 ID 格式（filedetails 链接 / JSON 转义 publishedfileid）
func TestParseWorkshopFileIds(t *testing.T) {
	html := `
	<a href="https://steamcommunity.com/sharedfiles/filedetails/?id=111222333">A</a>
	<span>publishedfileid\":444555666</span>
	<a href="/sharedfiles/filedetails/?id=777888999">B</a>
	<span>publishedfileid\":111222333</span>
	`
	ids := ParseWorkshopFileIds(html)
	if len(ids) != 3 {
		t.Fatalf("应去重提取 3 个, got %v", ids)
	}
	want := map[string]bool{"111222333": true, "444555666": true, "777888999": true}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("未知 id %q", id)
		}
	}
}

// 单页抓取需容错重试：空响应/网络错误不应静默丢页（代理不稳定时缺页=丢订阅）
func TestFetchPageWithRetry(t *testing.T) {
	// 空响应后成功
	calls := 0
	do := func(u string) (string, error) {
		calls++
		if calls == 1 {
			return "", nil // 空响应（实测 page 1 返回 0 字节）
		}
		return "<a href='filedetails/?id=123'>x</a>", nil
	}
	html, err := fetchPageWithRetry(do, "u", 3)
	if err != nil || !strings.Contains(html, "123") || calls != 2 {
		t.Fatalf("空响应重试: html=%q err=%v calls=%d", html, err, calls)
	}
	// 网络错误后成功
	calls = 0
	do = func(u string) (string, error) {
		calls++
		if calls == 1 {
			return "", io.ErrUnexpectedEOF
		}
		return "ok-id-456", nil
	}
	html, err = fetchPageWithRetry(do, "u", 3)
	if err != nil || html != "ok-id-456" {
		t.Fatalf("网络错误重试: html=%q err=%v", html, err)
	}
	// 全部失败必须报错（不再静默返回空）
	do = func(u string) (string, error) { return "", nil }
	if _, err = fetchPageWithRetry(do, "u", 2); err == nil {
		t.Fatal("全部空响应应返回错误")
	}
	// 首次成功不重试
	calls = 0
	do = func(u string) (string, error) { calls++; return "x", nil }
	if _, err = fetchPageWithRetry(do, "u", 3); err != nil || calls != 1 {
		t.Fatalf("首次成功不应重试: calls=%d err=%v", calls, err)
	}
}

// Cookie 输入解析：纯 cookie / curl -b 双引号 / 单引号 / cmd 脱字符 / --cookie 形式
func TestExtractCookie(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sessionid=abc; steamLoginSecure=xyz", "sessionid=abc; steamLoginSecure=xyz"},
		{`curl --url "https://x" -b "sessionid=abc; steamLoginSecure=xyz" -H "UA"`, "sessionid=abc; steamLoginSecure=xyz"},
		{"curl -b 'sessionid=abc' -H 'x'", "sessionid=abc"},
		{`curl --url ^"https://x^" -b ^"sessionid=abc; steamCountry=HK^%^7C170^" -H ^"UA^"`, "sessionid=abc; steamCountry=HK%7C170"},
		{"curl --cookie \"sessionid=abc\" ", "sessionid=abc"},
	}
	for _, c := range cases {
		if got := ExtractCookie(c.in); got != c.want {
			t.Errorf("ExtractCookie(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 删除订阅记录：按 modid 删除并返回是否删到
func TestDeleteSubscriptionRecord(t *testing.T) {
	db := setupSubTestDB(t)
	if err := db.Create(&model.ModSubscription{Modid: "111", Name: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	deleted, err := DeleteSubscriptionRecord("111")
	if err != nil || !deleted {
		t.Fatalf("删除应成功: deleted=%v err=%v", deleted, err)
	}
	var n int64
	db.Model(&model.ModSubscription{}).Where("modid = ?", "111").Count(&n)
	if n != 0 {
		t.Fatalf("记录应已删除, 剩余 %d", n)
	}
	deleted, err = DeleteSubscriptionRecord("111")
	if err != nil || deleted {
		t.Fatalf("重复删除应返回 false: deleted=%v err=%v", deleted, err)
	}
}


// 订阅测试 DB 脚手架（绝对路径，避开 go test 工作目录在包目录的坑）
func setupSubTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ModSubscription{}); err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	sqlDB, _ := db.DB()
	t.Cleanup(func() {
		database.DB = old
		_ = sqlDB.Close()
	})
	return db
}

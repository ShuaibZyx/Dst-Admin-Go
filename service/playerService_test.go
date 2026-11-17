package service

import "testing"

// C7: 日志行 [] 组不足 6 时须返回 nil(防 result[1..5] 索引 panic)。

func TestParsePlayerVOSufficientGroups(t *testing.T) {
	line := `xx player: {[ID] [7] [100] [KU_123] [Name] [winona]} yy`
	vo := parsePlayerVO(line)
	if vo == nil {
		t.Fatal("6 组应解析出玩家")
	}
	if vo.Key != "7" || vo.Day != "100" || vo.KuId != "KU_123" || vo.Name != "Name" || vo.Role != "winona" {
		t.Fatalf("字段解析错误: %+v", vo)
	}
}

func TestParsePlayerVOInsufficientGroupsReturnsNil(t *testing.T) {
	for _, line := range []string{
		`player: {[ID] [7]}`,
		`player: {[ID] [7] [100] [KU_123] [Name]}`, // 5 组,缺 Role
		`player: {no brackets}`,
		`plain line`,
	} {
		vo := parsePlayerVO(line)
		if vo != nil {
			t.Fatalf("[] 组不足/格式不符应返回 nil 防 panic,行 %q 解析出 %+v", line, vo)
		}
	}
}

// 玩家名含 ] } [ 等特殊字符时不能丢人/错位;名字含 "Host" 不能被误杀。

func TestParsePlayerVONameWithSpecialChars(t *testing.T) {
	cases := []struct {
		line, name, role string
	}{
		{`xx player: {[ID] [7] [100] [KU_123] [a]b[c] [winona]} yy`, "a]b[c", "winona"},
		{`player: {[ID] [7] [100] [KU_123] [weird}name] [wilson]}`, "weird}name", "wilson"},
		{`player: {[ID] [7] [100] [KU_123] [x] [y] [wanda]}`, "x] [y", "wanda"},
		{`player: {[ID] [7] [100] [KU_123] [Hello World 42] [wanda]}`, "Hello World 42", "wanda"},
	}
	for _, c := range cases {
		vo := parsePlayerVO(c.line)
		if vo == nil {
			t.Fatalf("应解析出玩家(名字 %q 不应导致丢人): %q", c.name, c.line)
		}
		if vo.Name != c.name || vo.Role != c.role {
			t.Fatalf("字段解析错: got name=%q role=%q, want name=%q role=%q (line %q)", vo.Name, vo.Role, c.name, c.role, c.line)
		}
		if vo.KuId != "KU_123" {
			t.Fatalf("KuId 解析错: %+v", vo)
		}
	}
}

func TestMatchPlayerLineFilters(t *testing.T) {
	const id = "req-123"
	// 名字含 Host 的真实玩家数据行必须通过
	if vo := matchPlayerLine(`player: {[req-123] [7] [100] [KU_123] [GhostHost] [wilson]}`, id); vo == nil || vo.Name != "GhostHost" {
		t.Fatalf("名字含 Host 的玩家不应被过滤: %+v", vo)
	}
	// 命令回显行必须跳过
	if vo := matchPlayerLine(`RemoteCommandInput: player: {[req-123] [7] [100] [KU_123] [Name] [wilson]}`, id); vo != nil {
		t.Fatalf("命令回显行应跳过: %+v", vo)
	}
	// 别的请求 id / 无 KU 的行必须跳过
	if vo := matchPlayerLine(`player: {[req-999] [7] [100] [KU_123] [Name] [wilson]}`, id); vo != nil {
		t.Fatalf("其他请求的行应跳过: %+v", vo)
	}
	if vo := matchPlayerLine(`player: {[req-123] [7] [100] [NOKU] [Name] [wilson]}`, id); vo != nil {
		t.Fatalf("无 KU 的行应跳过: %+v", vo)
	}
}

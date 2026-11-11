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

package service

import (
	"strings"
	"testing"
)

// A3: screen 命令的会话 key 与 lua 命令均须转义,防 clusterName/levelName/command 注入。

func TestScreenStuffCmdEscapesKeyAndLua(t *testing.T) {
	cmd := screenStuffCmd(`evil"$(reboot)`, `lv"`, `c_announce("hi")`)

	// key 中的 " $ 须被转义(转义形态为 \" 与 \$,非裸奔)
	if !strings.Contains(cmd, `\$(reboot)`) {
		t.Fatalf("key 中的命令替换未转义: %q", cmd)
	}
	if !strings.Contains(cmd, `\"`) {
		t.Fatalf("key 中的引号未转义: %q", cmd)
	}
	// lua 命令中的 " 须转义(转义形态为 \")
	if !strings.Contains(cmd, `c_announce(\"hi\")`) {
		t.Fatalf("lua 命令中的引号未转义: %q", cmd)
	}
	// 命令仍以 screen -S 开头 + -X stuff 结构完整
	if !strings.HasPrefix(cmd, `screen -S "`) {
		t.Fatalf("命令应以 screen -S 开头: %q", cmd)
	}
	if !strings.Contains(cmd, `-X stuff "`) {
		t.Fatalf("命令应含 -X stuff: %q", cmd)
	}
}

func TestScreenStuffCmdPlainArgs(t *testing.T) {
	cmd := screenStuffCmd("mycluster", "Master", "c_save()")
	// 正常参数:key 含 DST_8level_Master_mycluster,结构完整
	if !strings.Contains(cmd, "DST_8level_mycluster_Master") {
		t.Fatalf("key 未正确生成: %q", cmd)
	}
	if !strings.Contains(cmd, "c_save()") {
		t.Fatalf("lua 命令丢失: %q", cmd)
	}
}

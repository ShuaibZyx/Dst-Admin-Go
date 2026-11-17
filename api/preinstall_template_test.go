package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ini/ini"
	lua "github.com/yuin/gopher-lua"
)

// 世界模板配置骨架：海难=Shipwrecked+Volcano；四层世界=Master+Caves+Shipwrecked+Volcano。
// 模板不含存档数据，level.json 的 file 必须与层目录名一致（UsePreinstall 按目录整档替换）。

var templateRoot = filepath.Join("..", "static", "preinstall")

func readTemplateLevelFiles(t *testing.T, template string) map[string][]string {
	t.Helper()
	root := filepath.Join(templateRoot, template)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("模板目录不存在: %s", root)
	}
	if _, err := os.Stat(filepath.Join(root, "cluster.ini")); err != nil {
		t.Fatalf("缺少 cluster.ini: %s", template)
	}
	raw, err := os.ReadFile(filepath.Join(root, "level.json"))
	if err != nil {
		t.Fatalf("level.json 读取失败: %s: %v", template, err)
	}
	// 最小断言:file 值即层目录名(与 UsePreinstall/GetLevelConfig 语义一致)
	files := map[string][]string{}
	for _, seg := range strings.Split(string(raw), "\"file\"") {
		_ = seg
	}
	for _, name := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
		if strings.Contains(string(raw), `"file": "`+name+`"`) {
			for _, f := range []string{"server.ini", "leveldataoverride.lua", "modoverrides.lua"} {
				p := filepath.Join(root, name, f)
				b, err := os.ReadFile(p)
				if err != nil || len(b) == 0 {
					t.Fatalf("层文件缺失或为空: %s/%s/%s (err=%v)", template, name, f, err)
				}
				files[name] = append(files[name], f)
			}
		}
	}
	return files
}

func TestPreinstallTemplates_Layout(t *testing.T) {
	sw := readTemplateLevelFiles(t, "海难")
	if _, ok := sw["Shipwrecked"]; !ok {
		t.Fatalf("海难模板应含 Shipwrecked 层, 实际: %v", sw)
	}
	if _, ok := sw["Volcano"]; !ok {
		t.Fatalf("海难模板应含 Volcano 层, 实际: %v", sw)
	}
	if _, ok := sw["Master"]; ok {
		t.Fatalf("海难模板不应含 Master 层")
	}

	fl := readTemplateLevelFiles(t, "四层世界")
	for _, name := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
		if _, ok := fl[name]; !ok {
			t.Fatalf("四层世界应含 %s 层, 实际: %v", name, fl)
		}
	}
}

func parseLuaTable(t *testing.T, path string) lua.LValue {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	L := lua.NewState()
	defer L.Close()
	fn, err := L.LoadString(string(b))
	if err != nil {
		t.Fatalf("Lua 语法错误 %s: %v", path, err)
	}
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		t.Fatalf("Lua 执行失败 %s: %v", path, err)
	}
	ret := L.Get(-1)
	if ret.Type() != lua.LTTable {
		t.Fatalf("%s 应返回 table, 实际 %s", path, ret.Type())
	}
	return ret
}

func TestPreinstallTemplates_LuaParses(t *testing.T) {
	for _, tpl := range []string{"海难", "四层世界"} {
		root := filepath.Join(templateRoot, tpl)
		raw, err := os.ReadFile(filepath.Join(root, "level.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
			if !strings.Contains(string(raw), `"file": "`+name+`"`) {
				continue
			}
			parseLuaTable(t, filepath.Join(root, name, "leveldataoverride.lua"))
			modRaw, err := os.ReadFile(filepath.Join(root, name, "modoverrides.lua"))
			if err != nil {
				t.Fatal(err)
			}
			modStr := string(modRaw)
			if name == "Master" || name == "Caves" {
				if strings.TrimSpace(modStr) != "return {}" {
					t.Fatalf("%s/%s 的 modoverrides 应为 return {}, 实际: %s", tpl, name, strings.TrimSpace(modStr))
				}
			} else if !strings.Contains(modStr, "workshop-") {
				t.Fatalf("%s/%s 的 modoverrides 应含 workshop- 行(IA 模组), 实际: %s", tpl, name, modStr)
			}
		}
	}
}

func TestPreinstallTemplates_ShardConfig(t *testing.T) {
	for _, tpl := range []string{"海难", "四层世界"} {
		root := filepath.Join(templateRoot, tpl)
		raw, err := os.ReadFile(filepath.Join(root, "level.json"))
		if err != nil {
			t.Fatalf("%s level.json 读取失败: %v", tpl, err)
		}
		used := map[string]string{} // port -> level
		for _, name := range []string{"Master", "Caves", "Shipwrecked", "Volcano"} {
			if !strings.Contains(string(raw), `"file": "`+name+`"`) {
				continue
			}
			cfg, err := ini.Load(filepath.Join(root, name, "server.ini"))
			if err != nil {
				t.Fatalf("%s/%s server.ini 解析失败: %v", tpl, name, err)
			}
			net := cfg.Section("NETWORK")
			if net == nil {
				t.Fatalf("%s/%s server.ini 缺少 [NETWORK]", tpl, name)
			}
			ports := []string{net.Key("server_port").String()}
			if steam := cfg.Section("STEAM"); steam != nil {
				ports = append(ports, steam.Key("master_server_port").String(), steam.Key("authentication_port").String())
			}
			ports = append(ports, cfg.Section("SHARD").Key("id").String())
			for _, v := range ports {
				if v == "" {
					continue
				}
				if prev, dup := used[v]; dup {
					t.Fatalf("%s 端口/分片id %s 冲突: %s 与 %s", tpl, v, prev, name)
				}
				used[v] = name
			}
			if id := cfg.Section("SHARD").Key("id").String(); id == "" {
				t.Fatalf("%s/%s server.ini 缺少 [SHARD] id", tpl, name)
			}
		}
	}
}

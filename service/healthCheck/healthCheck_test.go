package healthCheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 构造最小 cluster 目录：Master + Caves 两个世界，可指定端口
func writeCluster(t *testing.T, masterPort, cavesPort string, withToken bool) string {
	t.Helper()
	base := t.TempDir()
	mk := func(level, port string) {
		dir := filepath.Join(base, level)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		ini := "[SHARD]\nname = " + level + "\nis_master = " + map[string]string{"Master": "true", "Caves": "false"}[level] + "\n\n[NETWORK]\nserver_port = " + port + "\n"
		if err := os.WriteFile(filepath.Join(dir, "server.ini"), []byte(ini), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Master", masterPort)
	mk("Caves", cavesPort)
	clusterIni := "[GAMEPLAY]\ngame_mode = survival\n\n[NETWORK]\ncluster_name = test\ncluster_password =\ncluster_description =\n\n[SHARD]\nshard_enabled = true\nbind_ip = 127.0.0.1\nmaster_ip = 127.0.0.1\nmaster_port = 10888\ncluster_key = testkey\n"
	if err := os.WriteFile(filepath.Join(base, "cluster.ini"), []byte(clusterIni), 0644); err != nil {
		t.Fatal(err)
	}
	if withToken {
		_ = os.WriteFile(filepath.Join(base, "cluster_token.txt"), []byte("pds-g^KU_test^test"), 0644)
	}
	return base
}

func find(items []HealthItem, item string) *HealthItem {
	// 同一检查项可有多条（每 level），取最严重结果
	rank := map[string]int{StatusOk: 0, StatusWarn: 1, StatusError: 2}
	var worst *HealthItem
	for i := range items {
		if items[i].Item == item {
			if worst == nil || rank[items[i].Status] > rank[worst.Status] {
				worst = &items[i]
			}
		}
	}
	return worst
}

// 计划 Task 4 Step 1：端口冲突必须报 error，且消息含冲突端口
func TestRunReportsPortConflict(t *testing.T) {
	base := writeCluster(t, "10999", "10999", true)
	items := Run(base)
	it := find(items, ItemPortConflict)
	if it == nil {
		t.Fatalf("应报告 %s，实际: %+v", ItemPortConflict, items)
	}
	if it.Status != StatusError {
		t.Fatalf("端口冲突应为 error 级，实际 %s", it.Status)
	}
	if !strings.Contains(it.Message, "10999") {
		t.Fatalf("消息应含冲突端口 10999，实际 %q", it.Message)
	}
}

func TestRunCleanClusterPassesPortCheck(t *testing.T) {
	base := writeCluster(t, "10999", "10998", true)
	items := Run(base)
	it := find(items, ItemPortConflict)
	if it == nil || it.Status != StatusOk {
		t.Fatalf("端口不冲突应为 ok，实际 %+v", it)
	}
}

// Step 4 补测：token 缺失
func TestRunWarnsWhenTokenMissing(t *testing.T) {
	base := writeCluster(t, "10999", "10998", false)
	items := Run(base)
	it := find(items, ItemToken)
	if it == nil {
		t.Fatalf("应报告 %s", ItemToken)
	}
	if it.Status != StatusWarn {
		t.Fatalf("token 缺失应为 warn，实际 %s", it.Status)
	}
}

// Step 4 补测：modoverrides 语法错（括号不配平）
func TestRunWarnsOnBadModoverrides(t *testing.T) {
	base := writeCluster(t, "10999", "10998", true)
	if err := os.WriteFile(filepath.Join(base, "Master", "modoverrides.lua"), []byte("return { [\"workshop-1\"]={ enabled=true "), 0644); err != nil {
		t.Fatal(err)
	}
	items := Run(base)
	it := find(items, ItemModoverrides)
	if it == nil {
		t.Fatalf("应报告 %s", ItemModoverrides)
	}
	if it.Status != StatusError {
		t.Fatalf("modoverrides 语法错应为 error，实际 %s", it.Status)
	}
}

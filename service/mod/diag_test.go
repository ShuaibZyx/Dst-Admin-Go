package mod

import (
	"os"
	"dst-admin-go/utils/dstConfigUtils"
	"path/filepath"
	"testing"
)

// 诊断：只读校验真实配置下的模组清单/状态/steamcmd 路径（不下载）
func TestDiagSyncPlan(t *testing.T) {
	restore := dstConfigUtils.SetDstConfigPathForTest(`D:\New World\dst-admin-shuaib\dst-admin-go\dst_config`)
	defer restore()
	cfg := dstConfigUtils.GetDstConfig()
	ids := ClusterModIDs(cfg.Cluster)
	t.Logf("cluster=%q ids=%d", cfg.Cluster, len(ids))
	if len(ids) == 0 {
		t.Fatal("ClusterModIDs 为空 ← 秒完成的头号嫌疑")
	}
	st := VerifyClusterMods(cfg.Cluster)
	ok, miss, broken := 0, 0, 0
	for _, s := range st {
		switch s.Status {
		case "ok":
			ok++
		case "missing":
			miss++
		case "broken":
			broken++
		}
	}
	t.Logf("校验: ok=%d missing=%d broken=%d", ok, miss, broken)
	sc := dstConfigUtils.ResolvePath(cfg.Parent_path, cfg.Steamcmd, dstConfigUtils.DefaultSteamcmd)
	t.Logf("steamcmdDir=%s exe=%s", sc, steamcmdExePath(sc))
	if _, err := os.Stat(steamcmdExePath(sc)); err != nil {
		t.Fatalf("steamcmd 可执行不存在 ← 全部秒失败的嫌疑: %v", err)
	}
	_ = filepath.Separator
}

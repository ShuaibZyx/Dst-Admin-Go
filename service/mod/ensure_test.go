package mod

import (
	"dst-admin-go/utils/dstConfigUtils"
	"os"
	"path/filepath"
	"testing"
)

func TestClusterModIDs(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "worlds", "MySave")
	for _, name := range []string{"Master", "Caves"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mo := `return {\n  ["workshop-1185229307"] = { enabled = true },\n  ["workshop-2929437733"] = { enabled = false },\n}`
	if err := os.WriteFile(filepath.Join(base, "Master", "modoverrides.lua"), []byte(mo), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "Caves", "modoverrides.lua"), []byte(mo), 0644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "dst_config")
	cfg := "parent_path=" + parent + "\nconf_dir=worlds\ncluster=MySave\nbin=32\nbeta=0\n"
	if err := os.WriteFile(p, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	restore := dstConfigUtils.SetDstConfigPathForTest(p)
	t.Cleanup(restore)

	ids := ClusterModIDs("MySave")
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got["1185229307"] || !got["2929437733"] || len(ids) != 2 {
		t.Fatalf("ClusterModIDs = %v, want 两个去重后的 id", ids)
	}
}

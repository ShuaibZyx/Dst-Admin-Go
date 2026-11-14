package service

import (
	"dst-admin-go/utils/dstConfigUtils"
	"path/filepath"
	"testing"
)

func TestDeleteSave(t *testing.T) {
	setupSaveTest(t)
	base := filepath.Join(dstConfigUtils.GetDstConfig().Parent_path, "worlds")

	// 建两个存档：Del1（干净）、../evil（非法名）
	mkCluster(t, filepath.Join(base, "Del1"))
	mkCluster(t, filepath.Join(base, "Keep"))

	// 非法名拒绝
	if err := DeleteSave("../evil"); err == nil {
		t.Fatal("非法存档名应拒绝")
	}
	if err := DeleteSave("a/b"); err == nil {
		t.Fatal("带分隔符应拒绝")
	}
	// 正常删除
	if err := DeleteSave("Del1"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if dirExists(filepath.Join(base, "Del1")) {
		t.Fatal("Del1 应已删除")
	}
	if !dirExists(filepath.Join(base, "Keep")) {
		t.Fatal("Keep 不应被误删")
	}
	// 不存在
	if err := DeleteSave("Nope"); err == nil {
		t.Fatal("不存在的存档应报错")
	}
}

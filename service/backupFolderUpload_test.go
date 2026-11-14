package service

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func makeFolderUpload(t *testing.T, rels map[string]string) *multipart.Form {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for rel, content := range rels {
		fw, err := w.CreateFormFile("files/"+rel, rel)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, content); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm
}

func TestAssembleSaveFolder(t *testing.T) {
	destBase := t.TempDir()
	form := makeFolderUpload(t, map[string]string{
		"MySave/cluster.ini":     "ini",
		"MySave/Master/save/x":   "save-data",
		"MySave/Caves/save/y":    "cave-data",
	})

	name, err := AssembleSaveFolder(form, destBase)
	if err != nil {
		t.Fatal(err)
	}
	if name != "MySave" {
		t.Fatalf("name = %q, want MySave", name)
	}
	for rel, want := range map[string]string{
		"cluster.ini":       "ini",
		"Master/save/x":     "save-data",
		"Caves/save/y":      "cave-data",
	} {
		b, err := os.ReadFile(filepath.Join(destBase, name, filepath.FromSlash(rel)))
		if err != nil || string(b) != want {
			t.Errorf("落盘文件 %s = %q, err=%v", rel, b, err)
		}
	}
}

func TestAssembleSaveFolderRejectsTraversal(t *testing.T) {
	destBase := t.TempDir()
	form := makeFolderUpload(t, map[string]string{
		"../evil.txt": "x",
	})
	if _, err := AssembleSaveFolder(form, destBase); err == nil {
		t.Fatal("路径穿越应拒绝")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(destBase), "evil.txt")); err == nil {
		t.Fatal("不应写出目录外文件")
	}
}

package service

import (
	"dst-admin-go/utils/fileUtils"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AssembleSaveFolder 把文件夹上传的文件按相对路径组装成一个存档目录（<destBase>/<存档名>/...）。
// 返回存档名。拒绝路径穿越与多顶层目录。
func AssembleSaveFolder(form *multipart.Form, destBase string) (string, error) {
	// 相对路径随字段名传输（multipart 会把 filename 裁成 basename）：字段名 = files/<相对路径>
	type uploadItem struct {
		rel string
		fh  *multipart.FileHeader
	}
	var items []uploadItem
	for field, fhs := range form.File {
		if !strings.HasPrefix(field, "files/") {
			continue
		}
		for _, fh := range fhs {
			items = append(items, uploadItem{rel: strings.TrimPrefix(field, "files/"), fh: fh})
		}
	}
	if len(items) == 0 {
		return "", errors.New("没有上传文件")
	}
	tmp, err := os.MkdirTemp("", "save-upload-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	top := ""
	for _, it := range items {
		fh := it.fh
		rel := filepath.Clean(filepath.FromSlash(it.rel))
		if rel == "." || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", errors.New("非法路径: " + fh.Filename)
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return "", errors.New("缺少顶层目录: " + rel)
		}
		if top == "" {
			top = parts[0]
		} else if parts[0] != top {
			return "", errors.New("一次只能上传一个存档文件夹")
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return "", err
		}
		if err := copyUploadedFile(fh, dst); err != nil {
			return "", err
		}
	}

	name := top
	if fileUtils.Exists(filepath.Join(destBase, name)) {
		name = top + "-" + time.Now().Format("20060102-150405")
	}
	safeName, err := fileUtils.SafeJoin(destBase, name)
	if err != nil {
		return "", err
	}
	if err := os.Rename(filepath.Join(tmp, top), safeName); err != nil {
		// 跨卷兜底：复制
		if err := copyTreeDir(filepath.Join(tmp, top), safeName); err != nil {
			return "", err
		}
	}
	return name, nil
}

func copyTreeDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTreeDir(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyOneFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyOneFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyUploadedFile(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

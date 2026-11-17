package fileUtils

import (
	"bufio"
	"dst-admin-go/utils/systemUtils"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil // C1: 权限错误等不应被当作“不存在”导致误创建/截断
}

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func IsFile(path string) bool {
	return !IsDir(path)
}

func CreateDir(dirName string) bool {
	if dirName == "" {
		return false
	}
	if Exists(dirName) {
		return false
	}
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func CreateFile(fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	return err
}

func WriterTXT(filename, content string) error {
	// 写入文件
	// 判断文件是否存在
	var file *os.File
	if _, err := os.Stat(filename); os.IsNotExist(err) {

		file, err = os.Create(filename)
		if err != nil {
			fmt.Println(err)
		}

	} else {
		//O_APPEND
		file, err = os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	_, err2 := w.WriteString(content)
	if err2 != nil {
		return err2
	}
	w.Flush()
	file.Sync()
	return nil

}

func ReadLnFile(filePath string) ([]string, error) {

	//打开文件
	fi, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer fi.Close()

	buf := bufio.NewScanner(fi)
	// 循环读取
	var lineArr []string
	for {
		if !buf.Scan() {
			break //文件读完了,退出for
		}
		line := buf.Text() //获取每一行
		lineArr = append(lineArr, line)
	}

	return lineArr, nil
}

func ReadFile(filePath string) (string, error) {

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("File reading error: ", err)
		return "", err
	}
	return string(data), err
}

func WriterLnFile(filename string, lines []string) error {

	var file *os.File
	if _, err := os.Stat(filename); os.IsNotExist(err) {

		file, err = os.Create(filename)
		if err != nil {
			fmt.Println(err)
		}

	} else {
		//O_APPEND
		file, err = os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	for _, v := range lines {
		fmt.Fprintln(w, v)
	}
	return w.Flush()
}

func ReverseRead(filename string, lineNum uint) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	//获取文件大小
	fs, err := file.Stat()
	if err != nil {
		return nil, err // C2: Stat 失败不得继续
	}
	fileSize := fs.Size()

	var offset int64 = -1   //偏移量，初始化为-1，若为0则会读到EOF
	char := make([]byte, 1) //用于读取单个字节
	lineStr := ""           //存放一行的数据
	buff := make([]string, 0, 100)
	for (-offset) <= fileSize {
		//通过Seek函数从末尾移动游标然后每次读取一个字节
		file.Seek(offset, io.SeekEnd)
		_, err := file.Read(char)
		if err != nil {
			return buff, err
		}
		if char[0] == '\n' {
			// offset--  //windows跳过'\r'
			lineNum-- //到此读取完一行
			buff = append(buff, lineStr)
			lineStr = ""
			if lineNum == 0 {
				return buff, nil
			}
		} else {
			lineStr = string(char) + lineStr
		}
		offset--
	}
	buff = append(buff, lineStr)
	return buff, nil
}

func DeleteFile(path string) error {
	err := os.Remove(path)
	if err != nil {
		log.Printf("remove "+path+" : %v\n", err)
		return err
	}
	return nil
}

func DeleteDir(path string) error {
	home, err := systemUtils.Home()
	if err != nil {
		return err
	}
	if path == filepath.Join(home, ".klei/DoNotStarveTogether") {
		return errors.New(".klei/DoNotStarveTogether not allow to delete")
	}
	err = os.RemoveAll(path)
	if err != nil {
		log.Printf("remove err "+path+" : %v\n", err)
	}
	return err
}

// Rename 移动文件/目录到新路径。
// os.Rename 跨卷必然失败（Windows 不同盘符、Unix EXDEV）——回退为流式复制+删除，
// 使面板/存档/备份分处不同磁盘时（如 D 盘面板配 C 盘存档）仍可移动，不再依赖"全部放 C 盘"。
func Rename(filePath, newName string) error {
	if err := os.Rename(filePath, newName); err == nil {
		return nil
	}
	if err := copyTo(filePath, newName); err != nil {
		return err
	}
	return os.RemoveAll(filePath)
}

// copyTo 递归复制 src 到 dst（dst 是完整目标路径；不同于 Copy 的"复制进目录"语义）
func copyTo(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTo(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func FindWorldDirs(rootPath string) ([]string, error) {
	var dirs []string

	// 遍历目录并列出满足条件的目录
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 如果是目录且名称包含 master 或 caves（不区分大小写）
		if info.IsDir() && (strings.Contains(strings.ToLower(info.Name()), "master") || strings.Contains(strings.ToLower(info.Name()), "caves")) {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return dirs, nil
}

func ListDirectories(root string) ([]string, error) {
	var dirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(dirs, func(i, j int) bool {
		fi, err := os.Stat(dirs[i])
		if err != nil {
			return false
		}
		fj, err := os.Stat(dirs[j])
		if err != nil {
			return false
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	return dirs, nil
}

func CreateFileIfNotExists(path string) error {

	// 检查文件是否存在
	_, err := os.Stat(path)
	if err == nil {
		// 文件已经存在，直接返回
		return nil
	}
	if !os.IsNotExist(err) {
		// 其他错误，返回错误信息
		return err
	}

	// 创建文件所在的目录
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		// 创建目录失败，返回错误信息
		return err
	}

	// 创建文件
	_, err = os.Create(path)
	if err != nil {
		// 创建文件失败，返回错误信息
		return err
	}

	// 创建成功，返回 nil
	return nil
}

func CreateDirIfNotExists(filepath string) {
	if !Exists(filepath) {
		_ = os.MkdirAll(filepath, os.ModePerm) // 递归创建：父目录可能整个不存在（如新选的根目录）
	}
}

func Copy(srcPath, outFileDir string) error {

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// 如果源文件是目录，则递归复制目录
	if srcInfo.IsDir() {
		// 创建目标目录（如果不存在）
		err = os.MkdirAll(outFileDir, srcInfo.Mode())
		if err != nil {
			return err
		}
		// 遍历源目录中的所有文件和子目录，并递归复制它们
		srcDir, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer srcDir.Close()

		files, err := srcDir.Readdir(-1)
		if err != nil {
			return err
		}

		for _, file := range files {
			srcFilePath := filepath.Join(srcPath, file.Name())
			outFilePath := filepath.Join(outFileDir, filepath.Base(srcPath))
			err = Copy(srcFilePath, outFilePath)
			if err != nil {
				return err
			}
		}

		return nil
	}

	return copyHelper(srcPath, outFileDir)
}

func copyHelper(srcPath, outFileDir string) error {
	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标目录（如果不存在）
	err = os.MkdirAll(outFileDir, 0755)
	if err != nil {
		return err
	}

	// 创建目标文件
	outFilePath := filepath.Join(outFileDir, filepath.Base(srcPath))
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 复制数据
	_, err = io.Copy(outFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

// SafeJoin 将用户提供的单段名称拼到 base 下，拒绝任何穿越/分隔符/绝对路径（A6/A9）
func SafeJoin(base, name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid name %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid name %q", name)
	}
	if filepath.IsAbs(name) || strings.Contains(name, ":") {
		return "", fmt.Errorf("invalid name %q", name)
	}
	return filepath.Join(base, name), nil
}

// ValidName 白名单校验：仅 [A-Za-z0-9_-]（A3/A9：level/cluster 名等用户输入）
func ValidName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

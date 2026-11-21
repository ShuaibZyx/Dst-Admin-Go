package pathUtils

import (
	"os"
	"path/filepath"
	"sync"
)

// 双根目录解析:数据根(用户数据)与资源根(随程序发布的 dist/static)。
// 旧实现一切相对 CWD——从哪个目录启动,数据/资源就散在哪(白屏、systemd 无
// WorkingDirectory 时数据写进 /)。现改为统一锚点,一次判定、全局一致。
//
// DataRoot 数据根(password.txt/dst_config/dst-db/session.secret 等):
//  1. DST_ADMIN_HOME 环境变量(显式指定,systemd/容器逃生口)
//  2. CWD 下已有数据标记(dst_config/password.txt/dst-db/config.yml/session.secret)
//     → CWD(现有部署零感知,含靠 cd 切数据目录的多实例用法)
//  3. exe 目录可写 → exe 目录(新部署/无 WorkingDirectory 的服务单元自动落位)
//  4. CWD(只读 exe 的最后退路)
//
// ResRoot 资源根(dist/static):
//  exe 目录带资源 → exe 目录(修"从别处启动白屏");否则 CWD(go run/测试),再退 exe。

var dataMarkers = []string{"dst_config", "password.txt", "dst-db", "config.yml", "session.secret"}

// DataRoot 数据根目录。详见包注释。
func DataRoot() string {
	if v := os.Getenv("DST_ADMIN_HOME"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for _, m := range dataMarkers {
		if exists(filepath.Join(cwd, m)) {
			return cwd
		}
	}
	if exe := exeDir(); dirWritable(exe) {
		return exe
	}
	return cwd
}

// ResRoot 资源根目录。详见包注释。
func ResRoot() string {
	exe := exeDir()
	if hasRes(exe) {
		return exe
	}
	if cwd, err := os.Getwd(); err == nil && hasRes(cwd) {
		return cwd
	}
	return exe
}

// Data 拼数据根下的路径
func Data(elem ...string) string {
	return filepath.Join(append([]string{DataRoot()}, elem...)...)
}

// Res 拼资源根下的路径
func Res(elem ...string) string {
	return filepath.Join(append([]string{ResRoot()}, elem...)...)
}

// ConfigPath config.yml:数据根优先(允许数据目录覆盖),回退资源根(随包发布)。
func ConfigPath() string {
	if p := Data("config.yml"); exists(p) {
		return p
	}
	return Res("config.yml")
}

func hasRes(dir string) bool {
	for _, n := range []string{"dist", "static"} {
		if info, err := os.Stat(filepath.Join(dir, n)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// dirWritable 可写性探测(建临时文件即删);按目录缓存——exe 目录不随运行变化。
// 权限位不可靠(root/ACL/Windows),只有真写一次才算数。
var writableCache sync.Map // dir -> bool

func dirWritable(dir string) bool {
	if v, ok := writableCache.Load(dir); ok {
		return v.(bool)
	}
	ok := false
	if f, err := os.CreateTemp(dir, ".writetest"); err == nil {
		name := f.Name()
		f.Close()
		os.Remove(name)
		ok = true
	}
	writableCache.Store(dir, ok)
	return ok
}

package service

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/process"
)

type ClusterContainer struct {
	container map[string]*LevelInstance
	lock      sync.RWMutex
}

func NewClusterContainer() *ClusterContainer {
	return &ClusterContainer{
		container: map[string]*LevelInstance{},
		lock:      sync.RWMutex{},
	}
}

func levelKey(cluster, levelName string) string {
	return cluster + "_" + levelName
}

func (receiver *ClusterContainer) StartLevel(cluster, levelName string, bin int, steamcmd, dstServerInstall, ugcDirectory, persistent_storage_root, conf_dir string) {
	key := levelKey(cluster, levelName)
	log.Println("正在启动 ", key)

	receiver.lock.Lock()
	value, ok := receiver.container[key]
	if ok && value.dstSeverInstall != dstServerInstall {
		delete(receiver.container, key)
		ok = false
	}
	if !ok {
		value = NewLevelInstance(cluster, levelName, bin, steamcmd, dstServerInstall, ugcDirectory, persistent_storage_root, conf_dir)
		receiver.container[key] = value
	}
	receiver.lock.Unlock()

	value.Start()
}

func (receiver *ClusterContainer) StopLevel(cluster, levelName string) {
	log.Println("正在停止世界", cluster, levelName)
	if value := receiver.get(cluster, levelName); value != nil {
		value.Stop()
	}
}

func (receiver *ClusterContainer) Send(cluster, levelName, message string) {
	if value := receiver.get(cluster, levelName); value != nil {
		value.Send(message)
	}
}

// matchLevelCmdline 判断进程命令行是否为 (cluster, level) 的游戏世界
func matchLevelCmdline(cmdline []string, cluster, level string) bool {
	if len(cmdline) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(cmdline[0]))
	if !strings.Contains(base, "dontstarve_dedicated_server") {
		return false
	}
	joined := strings.Join(cmdline, " ")
	return strings.Contains(joined, "-cluster "+cluster) && strings.Contains(joined, "-shard "+level)
}

// pidCache 短 TTL 缓存：状态轮询每次全量扫进程过重（曾拖垮 /status 至超时）
type pidEntry struct {
	pid int32
	at  time.Time
}

var pidCache sync.Map

// findLevelPid 扫描真实进程找回世界 pid（面板重启后或外部启动的场景）
func findLevelPid(cluster, level string) int32 {
	cacheKey := cluster + "/" + level
	if v, ok := pidCache.Load(cacheKey); ok {
		if e := v.(pidEntry); time.Since(e.at) < 5*time.Second {
			return e.pid
		}
	}
	pid := scanLevelPid(cluster, level)
	pidCache.Store(cacheKey, pidEntry{pid: pid, at: time.Now()})
	return pid
}

func scanLevelPid(cluster, level string) int32 {
	procs, err := process.Processes()
	if err != nil {
		return 0
	}
	for _, p := range procs {
		cl, err := p.CmdlineSlice()
		if err != nil {
			continue
		}
		if matchLevelCmdline(cl, cluster, level) {
			return p.Pid
		}
	}
	return 0
}

func memByPid(pid int32) float64 {
	if pid <= 0 {
		return 0
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return 0
	}
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return 0
	}
	return float64(memInfo.RSS) / (1024 * 1024)
}

func cpuByPid(pid int32) float64 {
	if pid <= 0 {
		return 0
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return 0
	}
	percent, err := p.Percent(time.Second)
	if err != nil {
		return 0
	}
	return percent
}

func (receiver *ClusterContainer) Status(cluster, levelName string) bool {
	value := receiver.get(cluster, levelName)
	if value != nil && value.Status() {
		return true
	}
	// 面板重启/外部启动：按真实进程判定
	return findLevelPid(cluster, levelName) > 0
}

func (receiver *ClusterContainer) MemUsage(cluster, levelName string) float64 {
	if value := receiver.get(cluster, levelName); value != nil {
		return value.GetProcessMemInfo()
	}
	return memByPid(findLevelPid(cluster, levelName))
}

func (receiver *ClusterContainer) CpuUsage(cluster, levelName string) float64 {
	if value := receiver.get(cluster, levelName); value != nil {
		return value.GetProcessCpuInfo()
	}
	return cpuByPid(findLevelPid(cluster, levelName))
}

// Kill 强杀游戏进程（B15：游戏卡死不响应 c_shutdown 时使用）
func (receiver *ClusterContainer) Kill(cluster, levelName string) {
	if value := receiver.get(cluster, levelName); value != nil {
		value.Kill()
	}
}

func (receiver *ClusterContainer) get(cluster, levelName string) *LevelInstance {
	receiver.lock.RLock()
	defer receiver.lock.RUnlock()
	return receiver.container[levelKey(cluster, levelName)]
}

func (receiver *ClusterContainer) Remove(cluster, levelName string) {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()
	delete(receiver.container, levelKey(cluster, levelName))
}

type LevelInstance struct {
	running  atomic.Bool
	lock     sync.Mutex // 保护状态迁移
	sendLock sync.Mutex // 保护 stdin 写入（B13 并发写交错）
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	pid      int

	cluster                 string
	levelName               string
	bin                     int
	steamcmd                string
	dstSeverInstall         string
	ugc_directory           string
	persistent_storage_root string
	conf_dir                string
}

func NewLevelInstance(cluster, levelName string, bin int, steamcmd, dstServerInstall, ugc_directory, persistent_storage_root, conf_dir string) *LevelInstance {
	game := &LevelInstance{
		cluster:                 cluster,
		levelName:               levelName,
		bin:                     bin,
		steamcmd:                steamcmd,
		dstSeverInstall:         dstServerInstall,
		ugc_directory:           ugc_directory,
		persistent_storage_root: persistent_storage_root,
		conf_dir:                conf_dir,
	}
	game.running.Store(false)
	return game
}

func (receiver *LevelInstance) Status() bool {
	return receiver.running.Load()
}

func (receiver *LevelInstance) setStopped() {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()
	receiver.running.Store(false)
	receiver.pid = 0 // I9: 清除 pid，防止误杀复用 PID 的无关进程
}

func (receiver *LevelInstance) Start() {
	receiver.lock.Lock()
	if receiver.running.Load() {
		receiver.lock.Unlock()
		return // A2: 加锁后双检，避免并发 Start 启动两个游戏进程
	}

	tLogsTxt := receiver.cluster + "_" + receiver.levelName + "_log"
	logFile, err := os.Create(tLogsTxt)
	if err != nil {
		receiver.lock.Unlock() // A2: 创建失败必须解锁，否则永久死锁
		log.Println("Error creating log file:", err)
		return
	}

	var args []string
	if receiver.bin == 32 {
		args = append(args, "./dontstarve_dedicated_server_nullrenderer.exe")
	} else {
		args = append(args, "./dontstarve_dedicated_server_nullrenderer_x64.exe")
	}
	args = append(args, "-console")
	args = append(args, "-cluster")
	args = append(args, receiver.cluster)
	args = append(args, "-shard")
	args = append(args, receiver.levelName)
	if receiver.persistent_storage_root != "" {
		args = append(args, "-persistent_storage_root")
		args = append(args, receiver.persistent_storage_root)
	}
	if receiver.conf_dir != "" {
		args = append(args, "-conf_dir")
		args = append(args, receiver.conf_dir)
	}
	if receiver.ugc_directory != "" {
		args = append(args, "-ugc_directory")
		args = append(args, receiver.ugc_directory)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if receiver.bin == 32 {
		cmd.Dir = receiver.dstSeverInstall + "\\bin"
	} else {
		cmd.Dir = receiver.dstSeverInstall + "\\bin64"
	}

	receiver.running.Store(true)
	receiver.lock.Unlock()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Error getting stdin pipe: %v\n", err)
		receiver.setStopped() // A2: 失败回退状态
		_ = logFile.Close()
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error getting stdout pipe: %v\n", err)
		receiver.setStopped()
		_ = logFile.Close()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Error getting stderr pipe: %v\n", err)
		receiver.setStopped()
		_ = logFile.Close()
		return
	}
	receiver.lock.Lock()
	receiver.stdin = stdin
	receiver.stdout = stdout
	receiver.lock.Unlock()

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting command: %v\n", err)
		receiver.setStopped()
		_ = logFile.Close()
		return
	}

	receiver.pid = cmd.Process.Pid

	// B16: 先读完输出再 Wait/关文件，避免截断与关闭管道竞争
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(os.Stdout, logFile), stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(os.Stderr, logFile), stderr)
	}()
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Printf("Error waiting for command: %v\n", err)
	}
	receiver.setStopped()
	_ = logFile.Close()
	log.Println("process exit !!!")
}

func (receiver *LevelInstance) Stop() {
	receiver.lock.Lock()
	running := receiver.running.Load()
	stdin := receiver.stdin
	receiver.lock.Unlock()

	if !running {
		return
	}
	if stdin == nil { // A2: 无 stdin 时不能 Send（nil 写入 panic）
		receiver.Kill()
		return
	}
	if err := receiver.Send("c_shutdown(true)"); err != nil {
		log.Println("stop game error", err)
	}
	// B15: running 由 Start 的 Wait 统一置 false，此处不提前置位
}

// Kill 强杀子进程（B15）
func (receiver *LevelInstance) Kill() {
	receiver.lock.Lock()
	pid := receiver.pid
	receiver.lock.Unlock()
	if pid <= 0 {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		log.Println("kill: process not found", pid)
		return
	}
	// I9: 校验目标确为本游戏进程再杀（防 PID 复用误杀）；名字读不到也保守跳过（M-7）
	if proc, err := process.NewProcess(int32(pid)); err == nil {
		name, nerr := proc.Name()
		if nerr != nil || name == "" {
			log.Println("kill skipped, cannot verify process name")
			return
		}
		if !strings.Contains(strings.ToLower(name), "dontstarve") {
			log.Println("kill skipped, pid reused by:", name)
			return
		}
	}
	if err := p.Kill(); err != nil {
		log.Println("kill error", err)
	}
}

func (receiver *LevelInstance) Send(cmd string) error {
	receiver.sendLock.Lock()
	defer receiver.sendLock.Unlock()
	receiver.lock.Lock()
	stdin := receiver.stdin // M-2: 与 Start 的写同步
	receiver.lock.Unlock()
	if stdin == nil {
		return fmt.Errorf("stdin not ready")
	}
	writer := bufio.NewWriter(stdin)
	log.Println("cmd: ", cmd)
	// B13: 仅还原转义引号（\\\" -> \"），不再全局删除反斜杠（会破坏路径与转义）
	// I1: 原样透传；Lua 转义由构造端 shellUtils.EscapeLuaStr 负责
	input := cmd + "\n"
	_, err := writer.WriteString(input)
	if err != nil {
		log.Printf("Error writing to stdin: %v\n", err)
		return err
	}
	return writer.Flush()
}

func (receiver *LevelInstance) GetProcessMemInfo() float64 {
	receiver.lock.Lock()
	pid := receiver.pid
	receiver.lock.Unlock()
	if pid <= 0 {
		return 0
	}
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		fmt.Println("Error getting process info:", err)
		return 0
	}
	memInfo, err := p.MemoryInfo()
	if err != nil {
		fmt.Println("Error getting memory info:", err)
		return 0
	}
	return float64(memInfo.RSS) / (1024 * 1024)
}

func (receiver *LevelInstance) GetProcessCpuInfo() float64 {
	receiver.lock.Lock()
	pid := receiver.pid
	receiver.lock.Unlock()
	if pid <= 0 {
		return 0
	}
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		fmt.Println("Error getting process info:", err)
		return 0
	}
	cpuPercent, err := p.Percent(time.Second)
	if err != nil {
		fmt.Println("Error getting cpu info:", err)
		return 0
	}
	return cpuPercent
}

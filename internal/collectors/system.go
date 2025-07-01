package collectors

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
)

// NetworkStats 网络接口统计数据
type NetworkStats struct {
	RxBytes uint64 // 接收字节数
	TxBytes uint64 // 发送字节数
}

// NetworkBandwidth 网络带宽数据
type NetworkBandwidth struct {
	Interface     string  // 接口名称
	UpBandwidth   float64 // 上行带宽 (Mbps)
	DownBandwidth float64 // 下行带宽 (Mbps)
}

// ProcessInfo 进程基础信息
type ProcessInfo struct {
	PID       int    // 进程ID
	PPID      int    // 父进程ID
	Name      string // 进程名称
	User      string // 运行用户
	State     string // 进程状态
	StartTime int64  // 启动时间戳
}

// ProcessStats 进程统计信息
type ProcessStats struct {
	ProcessInfo

	// CPU相关
	CPUPercent float64 // CPU使用率百分比
	CPUTime    uint64  // CPU时间(jiffies)

	// 内存相关
	MemoryRSS     uint64  // 物理内存使用(字节)
	MemoryVMS     uint64  // 虚拟内存使用(字节)
	MemoryPercent float64 // 内存使用率百分比

	// 文件和线程
	FileDescriptors int // 打开的文件描述符数量
	Threads         int // 线程数量

	// IO统计(可选，需要详细监控时启用)
	IOReadBytes  uint64 // 读取字节数
	IOWriteBytes uint64 // 写入字节数
	IOReadOps    uint64 // 读取操作数
	IOWriteOps   uint64 // 写入操作数

	// 上下文切换
	ContextSwitchesVoluntary   uint64 // 自愿上下文切换
	ContextSwitchesInvoluntary uint64 // 非自愿上下文切换
}

// SystemCollector 系统指标收集器
type SystemCollector struct {
	config   *config.SystemCollectorConfig
	instance string

	// 网络带宽计算相关
	mu            sync.RWMutex
	lastNetStats  map[string]NetworkStats
	lastNetTime   time.Time
	bandwidthData []NetworkBandwidth

	// 进程监控相关
	lastProcessStats map[int]*ProcessStats // 上次收集的进程统计信息，用于计算CPU使用率
	lastProcessTime  time.Time             // 上次收集进程信息的时间
}

// NewSystemCollector 创建新的系统收集器
func NewSystemCollector(cfg *config.SystemCollectorConfig, instance string) *SystemCollector {
	return &SystemCollector{
		config:           cfg,
		instance:         instance,
		lastNetStats:     make(map[string]NetworkStats),
		lastProcessStats: make(map[int]*ProcessStats),
	}
}

// Collect 收集系统指标
func (c *SystemCollector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	var metrics []client.Metric
	now := time.Now()

	for _, collectorName := range c.config.Collectors {
		switch collectorName {
		case "cpu":
			cpuMetrics, err := c.collectCPU()
			if err != nil {
				continue // 记录错误但继续收集其他指标
			}
			for i := range cpuMetrics {
				cpuMetrics[i].Timestamp = now
			}
			metrics = append(metrics, cpuMetrics...)

		case "memory":
			memMetrics, err := c.collectMemory()
			if err != nil {
				continue
			}
			for i := range memMetrics {
				memMetrics[i].Timestamp = now
			}
			metrics = append(metrics, memMetrics...)

		case "disk":
			diskMetrics, err := c.collectDisk()
			if err != nil {
				continue
			}
			for i := range diskMetrics {
				diskMetrics[i].Timestamp = now
			}
			metrics = append(metrics, diskMetrics...)

		case "network":
			netMetrics, err := c.collectNetwork()
			if err != nil {
				continue
			}
			for i := range netMetrics {
				netMetrics[i].Timestamp = now
			}
			metrics = append(metrics, netMetrics...)

		case "load":
			loadMetrics, err := c.collectLoad()
			if err != nil {
				continue
			}
			for i := range loadMetrics {
				loadMetrics[i].Timestamp = now
			}
			metrics = append(metrics, loadMetrics...)

		case "process":
			processMetrics, err := c.collectProcess()
			if err != nil {
				continue
			}
			for i := range processMetrics {
				processMetrics[i].Timestamp = now
			}
			metrics = append(metrics, processMetrics...)
		}
	}

	// 收集CPU温度（如果启用）
	if c.config.Temperature.Enabled {
		tempMetrics, err := c.collectCPUTemperature()
		if err == nil {
			for i := range tempMetrics {
				tempMetrics[i].Timestamp = now
			}
			metrics = append(metrics, tempMetrics...)
		}
	}

	// 收集网络带宽（如果启用）
	if c.config.Network.Enabled && c.config.Network.BandwidthEnabled {
		bandwidthMetrics, err := c.collectNetworkBandwidth()
		if err == nil {
			for i := range bandwidthMetrics {
				bandwidthMetrics[i].Timestamp = now
			}
			metrics = append(metrics, bandwidthMetrics...)
		}
	}

	return metrics, nil
}

// collectCPUTemperature 收集CPU温度
func (c *SystemCollector) collectCPUTemperature() ([]client.Metric, error) {
	var temperature float64
	var err error

	switch c.config.Temperature.TempSource {
	case "sensors":
		temperature, err = c.getCPUTemperatureFromSensors()
	case "thermal_zone":
		temperature, err = c.getCPUTemperatureFromThermalZone()
	default:
		// 尝试sensors，如果失败则尝试thermal_zone
		temperature, err = c.getCPUTemperatureFromSensors()
		if err != nil {
			temperature, err = c.getCPUTemperatureFromThermalZone()
		}
	}

	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"instance": c.instance,
		"sensor":   "cpu",
	}

	return []client.Metric{
		{
			Name:   "node_cpu_temperature_celsius",
			Value:  temperature,
			Labels: labels,
		},
	}, nil
}

// getCPUTemperatureFromSensors 从sensors命令获取CPU温度
func (c *SystemCollector) getCPUTemperatureFromSensors() (float64, error) {
	// 执行sensors命令，类似原C++实现
	cmd := exec.Command("sh", "-c", c.config.Temperature.SensorsCmd+" | grep 'Core 0' | awk '{print $3}' | cut -c 2-3")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("执行sensors命令失败: %w", err)
	}

	tempStr := strings.TrimSpace(string(output))
	temperature, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析温度值失败: %w", err)
	}

	return temperature, nil
}

// getCPUTemperatureFromThermalZone 从thermal_zone文件获取CPU温度
func (c *SystemCollector) getCPUTemperatureFromThermalZone() (float64, error) {
	data, err := os.ReadFile(c.config.Temperature.ThermalZone)
	if err != nil {
		return 0, fmt.Errorf("读取thermal_zone文件失败: %w", err)
	}

	tempStr := strings.TrimSpace(string(data))
	tempMilliCelsius, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析温度值失败: %w", err)
	}

	// thermal_zone中的温度是毫摄氏度，需要除以1000
	temperature := tempMilliCelsius / 1000.0
	return temperature, nil
}

// collectNetworkBandwidth 收集网络带宽（基于原C++算法）
func (c *SystemCollector) collectNetworkBandwidth() ([]client.Metric, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	currentStats, err := c.getNetworkStats()
	if err != nil {
		return nil, err
	}

	currentTime := time.Now()

	// 如果是第一次收集，保存当前数据并返回
	if len(c.lastNetStats) == 0 {
		c.lastNetStats = currentStats
		c.lastNetTime = currentTime
		return nil, nil
	}

	// 计算时间间隔
	interval := currentTime.Sub(c.lastNetTime).Seconds()
	if interval < 0.00001 {
		return nil, fmt.Errorf("时间间隔过短")
	}

	var metrics []client.Metric
	var bandwidthData []NetworkBandwidth

	// 计算每个接口的带宽
	for interfaceName, currentStat := range currentStats {
		// 检查接口过滤
		if !c.shouldMonitorInterface(interfaceName) {
			continue
		}

		lastStat, exists := c.lastNetStats[interfaceName]
		if !exists {
			continue
		}

		// 计算字节差值
		rxDiff := currentStat.RxBytes - lastStat.RxBytes
		txDiff := currentStat.TxBytes - lastStat.TxBytes

		// 计算带宽 (Mbps) - 基于原C++算法
		downBandwidth := float64(rxDiff*8) / (interval * 1024 * 1024) // 下行带宽
		upBandwidth := float64(txDiff*8) / (interval * 1024 * 1024)   // 上行带宽

		// 保存带宽数据
		bandwidthData = append(bandwidthData, NetworkBandwidth{
			Interface:     interfaceName,
			UpBandwidth:   upBandwidth,
			DownBandwidth: downBandwidth,
		})

		// 生成指标
		labels := map[string]string{
			"instance": c.instance,
			"device":   interfaceName,
		}

		metrics = append(metrics,
			client.Metric{
				Name:   "node_network_bandwidth_up_mbps",
				Value:  upBandwidth,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_bandwidth_down_mbps",
				Value:  downBandwidth,
				Labels: labels,
			},
		)
	}

	// 更新历史数据
	c.lastNetStats = currentStats
	c.lastNetTime = currentTime
	c.bandwidthData = bandwidthData

	return metrics, nil
}

// getNetworkStats 获取网络接口统计数据
func (c *SystemCollector) getNetworkStats() (map[string]NetworkStats, error) {
	netdevFile := filepath.Join(c.config.ProcPath, "net/dev")
	file, err := os.Open(netdevFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", netdevFile, err)
	}
	defer file.Close()

	stats := make(map[string]NetworkStats)
	scanner := bufio.NewScanner(file)

	// 跳过头部两行
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		interfaceName := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)

		stats[interfaceName] = NetworkStats{
			RxBytes: rxBytes,
			TxBytes: txBytes,
		}
	}

	return stats, scanner.Err()
}

// shouldMonitorInterface 检查是否应该监控该接口
func (c *SystemCollector) shouldMonitorInterface(interfaceName string) bool {
	// 排除回环接口
	if c.config.Network.ExcludeLoopback && interfaceName == "lo" {
		return false
	}

	// 如果指定了接口列表，只监控列表中的接口
	if len(c.config.Network.Interfaces) > 0 {
		for _, iface := range c.config.Network.Interfaces {
			if iface == interfaceName {
				return true
			}
		}
		return false
	}

	// 默认监控所有接口（除了被排除的）
	return true
}

// GetNetworkBandwidthData 获取最新的网络带宽数据
func (c *SystemCollector) GetNetworkBandwidthData() []NetworkBandwidth {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 返回副本
	result := make([]NetworkBandwidth, len(c.bandwidthData))
	copy(result, c.bandwidthData)
	return result
}

// collectCPU 收集CPU指标
func (c *SystemCollector) collectCPU() ([]client.Metric, error) {
	statFile := filepath.Join(c.config.ProcPath, "stat")
	file, err := os.Open(statFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", statFile, err)
	}
	defer file.Close()

	var metrics []client.Metric
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		cpuName := fields[0]
		if cpuName == "cpu" {
			cpuName = "total"
		}

		// 解析CPU时间
		user, _ := strconv.ParseFloat(fields[1], 64)
		nice, _ := strconv.ParseFloat(fields[2], 64)
		system, _ := strconv.ParseFloat(fields[3], 64)
		idle, _ := strconv.ParseFloat(fields[4], 64)
		iowait, _ := strconv.ParseFloat(fields[5], 64)
		irq, _ := strconv.ParseFloat(fields[6], 64)
		softirq, _ := strconv.ParseFloat(fields[7], 64)

		labels := map[string]string{
			"instance": c.instance,
			"cpu":      cpuName,
		}

		metrics = append(metrics,
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  user / 100.0, // 转换为秒
				Labels: addModeLabel(labels, "user"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  nice / 100.0,
				Labels: addModeLabel(labels, "nice"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  system / 100.0,
				Labels: addModeLabel(labels, "system"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  idle / 100.0,
				Labels: addModeLabel(labels, "idle"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  iowait / 100.0,
				Labels: addModeLabel(labels, "iowait"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  irq / 100.0,
				Labels: addModeLabel(labels, "irq"),
			},
			client.Metric{
				Name:   "node_cpu_seconds_total",
				Value:  softirq / 100.0,
				Labels: addModeLabel(labels, "softirq"),
			},
		)
	}

	return metrics, scanner.Err()
}

// collectMemory 收集内存指标
func (c *SystemCollector) collectMemory() ([]client.Metric, error) {
	meminfoFile := filepath.Join(c.config.ProcPath, "meminfo")
	file, err := os.Open(meminfoFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", meminfoFile, err)
	}
	defer file.Close()

	memInfo := make(map[string]float64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}

		// 转换为字节（meminfo中的值是KB）
		memInfo[key] = value * 1024
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	labels := map[string]string{"instance": c.instance}

	var metrics []client.Metric
	for key, value := range memInfo {
		metricName := fmt.Sprintf("node_memory_%s_bytes", key)
		metrics = append(metrics, client.Metric{
			Name:   metricName,
			Value:  value,
			Labels: labels,
		})
	}

	return metrics, nil
}

// collectDisk 收集磁盘指标
func (c *SystemCollector) collectDisk() ([]client.Metric, error) {
	var metrics []client.Metric

	// 收集磁盘I/O统计
	diskIOMetrics, err := c.collectDiskIO()
	if err != nil {
		log.Printf("收集磁盘I/O统计失败: %v", err)
	} else {
		metrics = append(metrics, diskIOMetrics...)
	}

	// 收集文件系统使用情况
	filesystemMetrics, err := c.collectFilesystem()
	if err != nil {
		log.Printf("收集文件系统统计失败: %v", err)
	} else {
		metrics = append(metrics, filesystemMetrics...)
	}

	return metrics, nil
}

// collectDiskIO 收集磁盘I/O指标
func (c *SystemCollector) collectDiskIO() ([]client.Metric, error) {
	diskstatsFile := filepath.Join(c.config.ProcPath, "diskstats")
	file, err := os.Open(diskstatsFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", diskstatsFile, err)
	}
	defer file.Close()

	var metrics []client.Metric
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		device := fields[2]

		// 跳过分区设备（只监控主设备）
		if strings.Contains(device, "loop") || len(device) > 3 && device[len(device)-1] >= '0' && device[len(device)-1] <= '9' {
			continue
		}

		labels := map[string]string{
			"instance": c.instance,
			"device":   device,
		}

		// 读取各项指标
		readsCompleted, _ := strconv.ParseFloat(fields[3], 64)
		readBytes, _ := strconv.ParseFloat(fields[5], 64)
		writesCompleted, _ := strconv.ParseFloat(fields[7], 64)
		writeBytes, _ := strconv.ParseFloat(fields[9], 64)

		metrics = append(metrics,
			client.Metric{
				Name:   "node_disk_reads_completed_total",
				Value:  readsCompleted,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_disk_read_bytes_total",
				Value:  readBytes * 512, // 扇区转字节
				Labels: labels,
			},
			client.Metric{
				Name:   "node_disk_writes_completed_total",
				Value:  writesCompleted,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_disk_written_bytes_total",
				Value:  writeBytes * 512,
				Labels: labels,
			},
		)
	}

	return metrics, scanner.Err()
}

// collectFilesystem 收集文件系统使用情况
func (c *SystemCollector) collectFilesystem() ([]client.Metric, error) {
	var metrics []client.Metric

	// 获取挂载点信息
	mountsFile := filepath.Join(c.config.ProcPath, "mounts")
	file, err := os.Open(mountsFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", mountsFile, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	mountPoints := make(map[string]string) // device -> mountpoint

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountpoint := fields[1]
		fstype := fields[2]

		// 跳过虚拟文件系统
		if strings.HasPrefix(device, "/dev/") &&
			!strings.Contains(fstype, "tmpfs") &&
			!strings.Contains(fstype, "devtmpfs") &&
			!strings.Contains(fstype, "sysfs") &&
			!strings.Contains(fstype, "proc") {
			mountPoints[device] = mountpoint
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 为每个挂载点收集使用情况
	for device, mountpoint := range mountPoints {
		fsMetrics, err := c.getFilesystemStats(device, mountpoint)
		if err != nil {
			log.Printf("获取文件系统统计失败 %s: %v", mountpoint, err)
			continue
		}
		metrics = append(metrics, fsMetrics...)
	}

	return metrics, nil
}

// getFilesystemStats 获取文件系统统计信息
func (c *SystemCollector) getFilesystemStats(device, mountpoint string) ([]client.Metric, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mountpoint, &stat); err != nil {
		return nil, fmt.Errorf("获取文件系统统计失败: %w", err)
	}

	labels := map[string]string{
		"instance":   c.instance,
		"device":     device,
		"mountpoint": mountpoint,
		"fstype":     "unknown", // 可以从/proc/mounts获取更准确的信息
	}

	// 计算各项指标
	blockSize := uint64(stat.Bsize)
	totalSize := stat.Blocks * blockSize
	freeSize := stat.Bavail * blockSize

	return []client.Metric{
		{
			Name:   "node_filesystem_size_bytes",
			Value:  float64(totalSize),
			Labels: labels,
		},
		{
			Name:   "node_filesystem_avail_bytes",
			Value:  float64(freeSize),
			Labels: labels,
		},
		{
			Name:   "node_filesystem_free_bytes",
			Value:  float64(stat.Bfree * blockSize),
			Labels: labels,
		},
		{
			Name:   "node_filesystem_files",
			Value:  float64(stat.Files),
			Labels: labels,
		},
		{
			Name:   "node_filesystem_files_free",
			Value:  float64(stat.Ffree),
			Labels: labels,
		},
	}, nil
}

// collectNetwork 收集网络指标
func (c *SystemCollector) collectNetwork() ([]client.Metric, error) {
	netdevFile := filepath.Join(c.config.ProcPath, "net/dev")
	file, err := os.Open(netdevFile)
	if err != nil {
		return nil, fmt.Errorf("打开%s失败: %w", netdevFile, err)
	}
	defer file.Close()

	var metrics []client.Metric
	scanner := bufio.NewScanner(file)

	// 跳过头部两行
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		device := strings.TrimSpace(parts[0])
		if device == "lo" { // 跳过回环接口
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		labels := map[string]string{
			"instance": c.instance,
			"device":   device,
		}

		// 接收字节数和包数
		rxBytes, _ := strconv.ParseFloat(fields[0], 64)
		rxPackets, _ := strconv.ParseFloat(fields[1], 64)
		rxErrors, _ := strconv.ParseFloat(fields[2], 64)
		rxDropped, _ := strconv.ParseFloat(fields[3], 64)

		// 发送字节数和包数
		txBytes, _ := strconv.ParseFloat(fields[8], 64)
		txPackets, _ := strconv.ParseFloat(fields[9], 64)
		txErrors, _ := strconv.ParseFloat(fields[10], 64)
		txDropped, _ := strconv.ParseFloat(fields[11], 64)

		metrics = append(metrics,
			client.Metric{
				Name:   "node_network_receive_bytes_total",
				Value:  rxBytes,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_receive_packets_total",
				Value:  rxPackets,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_receive_errs_total",
				Value:  rxErrors,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_receive_drop_total",
				Value:  rxDropped,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_transmit_bytes_total",
				Value:  txBytes,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_transmit_packets_total",
				Value:  txPackets,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_transmit_errs_total",
				Value:  txErrors,
				Labels: labels,
			},
			client.Metric{
				Name:   "node_network_transmit_drop_total",
				Value:  txDropped,
				Labels: labels,
			},
		)
	}

	return metrics, scanner.Err()
}

// collectLoad 收集系统负载指标
func (c *SystemCollector) collectLoad() ([]client.Metric, error) {
	loadavgFile := filepath.Join(c.config.ProcPath, "loadavg")
	data, err := os.ReadFile(loadavgFile)
	if err != nil {
		return nil, fmt.Errorf("读取%s失败: %w", loadavgFile, err)
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil, fmt.Errorf("loadavg格式错误")
	}

	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)

	labels := map[string]string{"instance": c.instance}

	return []client.Metric{
		{
			Name:   "node_load1",
			Value:  load1,
			Labels: labels,
		},
		{
			Name:   "node_load5",
			Value:  load5,
			Labels: labels,
		},
		{
			Name:   "node_load15",
			Value:  load15,
			Labels: labels,
		},
	}, nil
}

// addModeLabel 添加模式标签
func addModeLabel(labels map[string]string, mode string) map[string]string {
	result := make(map[string]string)
	for k, v := range labels {
		result[k] = v
	}
	result["mode"] = mode
	return result
}

// collectProcess 收集进程指标
func (c *SystemCollector) collectProcess() ([]client.Metric, error) {
	if !c.config.Process.Enabled {
		return nil, nil
	}

	// 获取符合条件的进程列表
	processStats, err := c.getProcessList()
	if err != nil {
		return nil, fmt.Errorf("获取进程列表失败: %w", err)
	}

	var metrics []client.Metric
	now := time.Now()

	// 锁定以更新进程缓存
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算时间间隔（用于CPU使用率计算）
	timeInterval := now.Sub(c.lastProcessTime).Seconds()
	if timeInterval == 0 {
		timeInterval = 1 // 避免除零错误
	}

	// 生成进程统计指标
	for _, proc := range processStats {
		// 基础标签
		labels := map[string]string{
			"instance": c.instance,
			"pid":      strconv.Itoa(proc.PID),
			"name":     proc.Name,
			"user":     proc.User,
			"state":    proc.State,
		}

		// 进程存在指标
		metrics = append(metrics, client.Metric{
			Name:   "process_running",
			Value:  1,
			Labels: labels,
		})

		// CPU使用率
		if proc.CPUPercent >= 0 {
			metrics = append(metrics, client.Metric{
				Name:   "process_cpu_percent",
				Value:  proc.CPUPercent,
				Labels: labels,
			})
		}

		// 内存使用
		metrics = append(metrics,
			client.Metric{
				Name:   "process_memory_rss_bytes",
				Value:  float64(proc.MemoryRSS),
				Labels: labels,
			},
			client.Metric{
				Name:   "process_memory_vms_bytes",
				Value:  float64(proc.MemoryVMS),
				Labels: labels,
			},
		)

		if proc.MemoryPercent >= 0 {
			metrics = append(metrics, client.Metric{
				Name:   "process_memory_percent",
				Value:  proc.MemoryPercent,
				Labels: labels,
			})
		}

		// 文件描述符和线程
		if proc.FileDescriptors > 0 {
			metrics = append(metrics, client.Metric{
				Name:   "process_file_descriptors",
				Value:  float64(proc.FileDescriptors),
				Labels: labels,
			})
		}

		if proc.Threads > 0 {
			metrics = append(metrics, client.Metric{
				Name:   "process_threads_count",
				Value:  float64(proc.Threads),
				Labels: labels,
			})
		}

		// 进程启动时间
		if proc.StartTime > 0 {
			metrics = append(metrics, client.Metric{
				Name:   "process_start_time_seconds",
				Value:  float64(proc.StartTime),
				Labels: labels,
			})
		}

		// 详细监控信息（IO和上下文切换）
		if c.config.Process.CollectDetailed {
			if proc.IOReadBytes > 0 || proc.IOWriteBytes > 0 {
				metrics = append(metrics,
					client.Metric{
						Name:   "process_io_read_bytes_total",
						Value:  float64(proc.IOReadBytes),
						Labels: labels,
					},
					client.Metric{
						Name:   "process_io_write_bytes_total",
						Value:  float64(proc.IOWriteBytes),
						Labels: labels,
					},
				)
			}

			if proc.ContextSwitchesVoluntary > 0 || proc.ContextSwitchesInvoluntary > 0 {
				voluntaryLabels := addModeLabel(labels, "voluntary")
				involuntaryLabels := addModeLabel(labels, "involuntary")

				metrics = append(metrics,
					client.Metric{
						Name:   "process_context_switches_total",
						Value:  float64(proc.ContextSwitchesVoluntary),
						Labels: voluntaryLabels,
					},
					client.Metric{
						Name:   "process_context_switches_total",
						Value:  float64(proc.ContextSwitchesInvoluntary),
						Labels: involuntaryLabels,
					},
				)
			}
		}
	}

	// 生成汇总指标
	userProcessCount := make(map[string]int)
	stateProcessCount := make(map[string]int)

	for _, proc := range processStats {
		userProcessCount[proc.User]++
		stateProcessCount[proc.State]++
	}

	// 按用户分组的进程数量
	for user, count := range userProcessCount {
		metrics = append(metrics, client.Metric{
			Name:  "process_running_count",
			Value: float64(count),
			Labels: map[string]string{
				"instance": c.instance,
				"user":     user,
			},
		})
	}

	// 按状态分组的进程数量
	for state, count := range stateProcessCount {
		metrics = append(metrics, client.Metric{
			Name:  "process_state_count",
			Value: float64(count),
			Labels: map[string]string{
				"instance": c.instance,
				"state":    state,
			},
		})
	}

	// 更新缓存
	newProcessCache := make(map[int]*ProcessStats)
	for _, proc := range processStats {
		procCopy := *proc
		newProcessCache[proc.PID] = &procCopy
	}
	c.lastProcessStats = newProcessCache
	c.lastProcessTime = now

	return metrics, nil
}

// getProcessList 获取符合条件的进程列表
func (c *SystemCollector) getProcessList() ([]*ProcessStats, error) {
	procDir := c.config.ProcPath
	if procDir == "" {
		procDir = "/proc"
	}

	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("读取%s目录失败: %w", procDir, err)
	}

	var processStats []*ProcessStats

	for _, entry := range entries {
		// 只处理数字目录（进程ID）
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// 获取进程详细信息
		procStat, err := c.getProcessInfo(pid)
		if err != nil {
			continue // 进程可能已经退出，跳过
		}

		// 检查是否应该监控此进程
		if !c.shouldMonitorProcess(procStat) {
			continue
		}

		processStats = append(processStats, procStat)
	}

	return processStats, nil
}

// getProcessInfo 获取单个进程的详细信息
func (c *SystemCollector) getProcessInfo(pid int) (*ProcessStats, error) {
	procStat := &ProcessStats{}
	procStat.PID = pid

	// 解析/proc/[pid]/stat文件
	if err := c.parseProcessStat(pid, procStat); err != nil {
		return nil, err
	}

	// 解析/proc/[pid]/status文件
	if err := c.parseProcessStatus(pid, procStat); err != nil {
		return nil, err
	}

	// 如果启用了详细监控，解析IO信息
	if c.config.Process.CollectDetailed {
		c.parseProcessIO(pid, procStat) // IO文件可能不存在，不返回错误
	}

	// 计算CPU使用率
	c.calculateCPUPercent(procStat)

	// 计算内存使用率
	c.calculateMemoryPercent(procStat)

	return procStat, nil
}

// shouldMonitorProcess 检查是否应该监控此进程
func (c *SystemCollector) shouldMonitorProcess(proc *ProcessStats) bool {
	// 如果配置为监控所有进程
	if c.config.Process.MonitorAll {
		return !c.isExcludedProcess(proc)
	}

	// 检查包含列表
	if len(c.config.Process.IncludeNames) > 0 {
		included := false
		for _, pattern := range c.config.Process.IncludeNames {
			if matched, _ := filepath.Match(pattern, proc.Name); matched {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// 检查用户列表
	if len(c.config.Process.IncludeUsers) > 0 {
		included := false
		for _, user := range c.config.Process.IncludeUsers {
			if proc.User == user {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// 检查排除列表
	if c.isExcludedProcess(proc) {
		return false
	}

	// 检查资源阈值
	if proc.CPUPercent < c.config.Process.MinCPUPercent &&
		float64(proc.MemoryRSS)/(1024*1024) < c.config.Process.MinMemoryMB {
		return false
	}

	return true
}

// isExcludedProcess 检查进程是否在排除列表中
func (c *SystemCollector) isExcludedProcess(proc *ProcessStats) bool {
	for _, pattern := range c.config.Process.ExcludeNames {
		if matched, _ := filepath.Match(pattern, proc.Name); matched {
			return true
		}
	}
	return false
}

// parseProcessStat 解析/proc/[pid]/stat文件
func (c *SystemCollector) parseProcessStat(pid int, proc *ProcessStats) error {
	statFile := filepath.Join(c.config.ProcPath, strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statFile)
	if err != nil {
		return fmt.Errorf("读取%s失败: %w", statFile, err)
	}

	line := string(data)

	// 处理进程名可能包含空格和括号的情况
	// 格式: pid (comm) state ppid ...
	startParen := strings.Index(line, "(")
	endParen := strings.LastIndex(line, ")")
	if startParen == -1 || endParen == -1 || endParen <= startParen {
		return fmt.Errorf("stat文件格式错误")
	}

	// 提取进程名
	proc.Name = line[startParen+1 : endParen]

	// 解析其他字段
	fields := strings.Fields(line[endParen+1:])
	if len(fields) < 20 {
		return fmt.Errorf("stat文件字段不足")
	}

	// 字段索引（基于Linux内核文档）
	proc.State = fields[0] // state
	if ppid, err := strconv.Atoi(fields[1]); err == nil {
		proc.PPID = ppid
	}

	// CPU时间（字段13和14：utime和stime）
	if len(fields) > 13 {
		if utime, err := strconv.ParseUint(fields[11], 10, 64); err == nil {
			if stime, err := strconv.ParseUint(fields[12], 10, 64); err == nil {
				proc.CPUTime = utime + stime
			}
		}
	}

	// 线程数（字段19）
	if len(fields) > 19 {
		if threads, err := strconv.Atoi(fields[17]); err == nil {
			proc.Threads = threads
		}
	}

	// 启动时间（字段21）
	if len(fields) > 21 {
		if starttime, err := strconv.ParseUint(fields[19], 10, 64); err == nil {
			// 转换为Unix时间戳（需要系统启动时间）
			proc.StartTime = int64(starttime) // 简化实现，实际应该加上系统启动时间
		}
	}

	// 内存信息作为后备数据（优先使用status文件的数据）
	// 虚拟内存大小（字段22，字节单位）
	if len(fields) > 22 && proc.MemoryVMS == 0 {
		if vsize, err := strconv.ParseUint(fields[20], 10, 64); err == nil {
			proc.MemoryVMS = vsize
		}
	}

	// RSS内存（字段23，单位为页）
	if len(fields) > 23 && proc.MemoryRSS == 0 {
		if rss, err := strconv.ParseUint(fields[21], 10, 64); err == nil {
			pageSize := uint64(4096) // 假设页大小为4KB
			proc.MemoryRSS = rss * pageSize
		}
	}

	return nil
}

// parseProcessStatus 解析/proc/[pid]/status文件
func (c *SystemCollector) parseProcessStatus(pid int, proc *ProcessStats) error {
	statusFile := filepath.Join(c.config.ProcPath, strconv.Itoa(pid), "status")
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return fmt.Errorf("读取%s失败: %w", statusFile, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Uid":
			// 解析用户ID，格式: Real Effective Saved Filesystem
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if uid, err := strconv.Atoi(fields[0]); err == nil {
					proc.User = c.getUserName(uid)
				}
			}
		case "VmRSS":
			// RSS内存，格式: "1234 kB"
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if rss, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
					proc.MemoryRSS = rss * 1024 // 转换为字节
				}
			}
		case "VmSize":
			// 虚拟内存大小，格式: "1234 kB"
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if vms, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
					proc.MemoryVMS = vms * 1024 // 转换为字节
				}
			}
		case "FDSize":
			// 文件描述符数量
			if fd, err := strconv.Atoi(value); err == nil {
				proc.FileDescriptors = fd
			}
		case "voluntary_ctxt_switches":
			// 自愿上下文切换
			if switches, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.ContextSwitchesVoluntary = switches
			}
		case "nonvoluntary_ctxt_switches":
			// 非自愿上下文切换
			if switches, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.ContextSwitchesInvoluntary = switches
			}
		}
	}

	return scanner.Err()
}

// parseProcessIO 解析/proc/[pid]/io文件（可选，某些系统可能没有）
func (c *SystemCollector) parseProcessIO(pid int, proc *ProcessStats) {
	ioFile := filepath.Join(c.config.ProcPath, strconv.Itoa(pid), "io")
	data, err := os.ReadFile(ioFile)
	if err != nil {
		return // IO文件可能不存在，不报错
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "read_bytes":
			if bytes, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.IOReadBytes = bytes
			}
		case "write_bytes":
			if bytes, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.IOWriteBytes = bytes
			}
		case "syscr":
			if ops, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.IOReadOps = ops
			}
		case "syscw":
			if ops, err := strconv.ParseUint(value, 10, 64); err == nil {
				proc.IOWriteOps = ops
			}
		}
	}
}

// calculateCPUPercent 计算CPU使用率
func (c *SystemCollector) calculateCPUPercent(proc *ProcessStats) {
	// 需要上次的数据来计算CPU使用率
	lastProc, exists := c.lastProcessStats[proc.PID]
	if !exists || c.lastProcessTime.IsZero() {
		proc.CPUPercent = -1 // 表示无法计算
		return
	}

	// 计算时间差（秒）
	timeDelta := time.Now().Sub(c.lastProcessTime).Seconds()
	if timeDelta <= 0 {
		proc.CPUPercent = -1
		return
	}

	// 计算CPU时间差（jiffies）
	cpuTimeDelta := float64(proc.CPUTime - lastProc.CPUTime)

	// 假设时钟频率为100Hz（每秒100个jiffies）
	clockTicks := 100.0

	// 计算CPU使用率百分比
	proc.CPUPercent = (cpuTimeDelta / clockTicks) / timeDelta * 100.0

	// 限制在合理范围内
	if proc.CPUPercent < 0 {
		proc.CPUPercent = 0
	} else if proc.CPUPercent > 100 {
		proc.CPUPercent = 100
	}
}

// calculateMemoryPercent 计算内存使用率
func (c *SystemCollector) calculateMemoryPercent(proc *ProcessStats) {
	// 读取系统总内存
	meminfoFile := filepath.Join(c.config.ProcPath, "meminfo")
	data, err := os.ReadFile(meminfoFile)
	if err != nil {
		proc.MemoryPercent = -1
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var totalMem uint64

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mem, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					totalMem = mem * 1024 // 转换为字节
					break
				}
			}
		}
	}

	if totalMem > 0 && proc.MemoryRSS > 0 {
		proc.MemoryPercent = float64(proc.MemoryRSS) / float64(totalMem) * 100.0
	} else {
		proc.MemoryPercent = -1
	}
}

// getUserName 根据UID获取用户名
func (c *SystemCollector) getUserName(uid int) string {
	// 简单实现：读取/etc/passwd文件
	// 在生产环境中可能需要更完善的实现
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return strconv.Itoa(uid) // 返回UID字符串
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 3 {
			if userUID, err := strconv.Atoi(parts[2]); err == nil && userUID == uid {
				return parts[0] // 返回用户名
			}
		}
	}

	return strconv.Itoa(uid) // 如果找不到，返回UID字符串
}

package collectors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ros_exporter/internal/b2"
	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
	"ros_exporter/internal/g1"
)

// BMSData 跨机器人统一的电池数据结构。
//
// G1/Go2/B2 各自的 SDK 暴露的字段结构不同，BMS collector 把它们归一化到这个结构，
// 让 robot_battery_* 系列指标对所有机器人保持一致的标签 schema 和指标名。
//
// B2 路径下 Health 字段不可填（codex C-2：BmsState IDL 没有 health 字段）：保留 0 值。
type BMSData struct {
	Voltage     float64 // 电压 V
	Current     float64 // 电流 A
	SOC         float64 // 电量百分比 0-100
	Temperature float64 // 温度 °C
	Power       float64 // 功率 W = V × A
	Cycles      float64 // 充电周期
	Health      float64 // 电池健康度 % (B2 路径下不可用，为 0)
}

// BMSInterface 内部接口：把不同传输层（unitree SDK / serial / canbus / mock）的电池读取归一化。
type BMSInterface interface {
	Connect() error
	Disconnect() error
	ReadBMSData() (*BMSData, error)
	IsConnected() bool
}

// BMSCollector 输出 robot_battery_* 系列指标。
//
// 对外 metric 名不变，仅底层数据来源改为 internal/b2 和 internal/g1 的 DataSource 抽象。
// 这是为保证 G1 现有部署"零行为变化"硬约束（plan v3 第 3.10 节）。
type BMSCollector struct {
	config    *config.BMSCollectorConfig
	instance  string
	bmsIface  BMSInterface
	connected bool
}

func NewBMSCollector(cfg *config.BMSCollectorConfig, instance string) *BMSCollector {
	collector := &BMSCollector{
		config:   cfg,
		instance: instance,
	}

	switch cfg.InterfaceType {
	case "unitree_sdk":
		collector.bmsIface = NewUnitreeSDKInterface(cfg)
	case "serial":
		collector.bmsIface = NewSerialInterface(cfg)
	case "canbus":
		collector.bmsIface = NewCANInterface(cfg)
	default:
		collector.bmsIface = NewMockInterface(cfg)
	}

	return collector
}

func (c *BMSCollector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	if !c.connected {
		if err := c.bmsIface.Connect(); err != nil {
			return nil, fmt.Errorf("连接BMS失败: %w", err)
		}
		c.connected = true
	}

	if !c.bmsIface.IsConnected() {
		c.connected = false
		return nil, fmt.Errorf("BMS连接断开")
	}

	bmsData, err := c.bmsIface.ReadBMSData()
	if err != nil {
		return nil, fmt.Errorf("读取BMS数据失败: %w", err)
	}

	now := time.Now()
	labels := map[string]string{
		"instance":   c.instance,
		"battery_id": "main",
		"interface":  c.config.InterfaceType,
	}

	return []client.Metric{
		{Name: "robot_battery_voltage_volts", Value: bmsData.Voltage, Labels: labels, Timestamp: now},
		{Name: "robot_battery_current_amperes", Value: bmsData.Current, Labels: labels, Timestamp: now},
		{Name: "robot_battery_soc_percent", Value: bmsData.SOC, Labels: labels, Timestamp: now},
		{Name: "robot_battery_temperature_celsius", Value: bmsData.Temperature, Labels: labels, Timestamp: now},
		{Name: "robot_battery_power_watts", Value: bmsData.Power, Labels: labels, Timestamp: now},
		{Name: "robot_battery_cycles_total", Value: bmsData.Cycles, Labels: labels, Timestamp: now},
		{Name: "robot_battery_health_percent", Value: bmsData.Health, Labels: labels, Timestamp: now},
	}, nil
}

func (c *BMSCollector) Close() error {
	if c.connected && c.bmsIface != nil {
		c.connected = false
		return c.bmsIface.Disconnect()
	}
	return nil
}

// UnitreeSDKInterface 用宇树官方 SDK 读电池（G1 / Go2 / B2 通用入口）。
//
// 内部按 robotType 分派到对应包的 DataSource 抽象。
type UnitreeSDKInterface struct {
	config    *config.BMSCollectorConfig
	connected bool
	robotType string

	g1DS g1.G1DataSource // G1 数据源（按 BMSCollectorConfig.G1DataSource 选 sdk/mock）
	b2DS b2.B2DataSource // B2 数据源（始终用 dds 数据源，因为 BMS 选 unitree_sdk 即意味着要真实 SDK）
}

func NewUnitreeSDKInterface(cfg *config.BMSCollectorConfig) *UnitreeSDKInterface {
	return &UnitreeSDKInterface{
		config:    cfg,
		connected: false,
		robotType: "auto",
	}
}

func (u *UnitreeSDKInterface) Connect() error {
	if u.robotType == "auto" {
		detected, err := u.detectRobotType()
		if err != nil {
			return fmt.Errorf("检测机器人类型失败: %w", err)
		}
		u.robotType = detected
	}
	if u.config != nil && u.config.RobotType != "" && u.config.RobotType != "auto" {
		// 显式配置覆盖自动检测
		u.robotType = u.config.RobotType
	}

	switch u.robotType {
	case "g1":
		return u.connectG1()
	case "go2":
		return u.connectGo2()
	case "b2":
		return u.connectB2()
	default:
		return fmt.Errorf("不支持的机器人类型: %s", u.robotType)
	}
}

func (u *UnitreeSDKInterface) Disconnect() error {
	if !u.connected {
		return nil
	}
	switch u.robotType {
	case "g1":
		return u.disconnectG1()
	case "go2":
		return u.disconnectGo2()
	case "b2":
		return u.disconnectB2()
	default:
		u.connected = false
		return nil
	}
}

func (u *UnitreeSDKInterface) ReadBMSData() (*BMSData, error) {
	if !u.connected {
		return nil, fmt.Errorf("SDK未连接")
	}
	switch u.robotType {
	case "g1":
		return u.readG1BMSData()
	case "go2":
		return u.readGo2BMSData()
	case "b2":
		return u.readB2BMSData()
	default:
		return nil, fmt.Errorf("未知的机器人类型: %s", u.robotType)
	}
}

func (u *UnitreeSDKInterface) IsConnected() bool {
	return u.connected
}

// detectRobotType 三步走：hostname 关键字 → /etc/robot_type → 默认 go2
func (u *UnitreeSDKInterface) detectRobotType() (string, error) {
	if t := u.detectFromSystemInfo(); t != "" {
		return t, nil
	}
	if t := u.detectFromNetworkConfig(); t != "" {
		return t, nil
	}
	if t := u.detectFromSDKResponse(); t != "" {
		return t, nil
	}
	return "go2", nil
}

func (u *UnitreeSDKInterface) detectFromSystemInfo() string {
	if hostname, err := os.Hostname(); err == nil {
		hostname = strings.ToLower(hostname)
		if strings.Contains(hostname, "g1") {
			return "g1"
		}
		if strings.Contains(hostname, "go2") {
			return "go2"
		}
		if strings.Contains(hostname, "b2") {
			return "b2"
		}
	}
	if data, err := os.ReadFile("/etc/robot_type"); err == nil {
		t := strings.TrimSpace(strings.ToLower(string(data)))
		if t == "g1" || t == "go2" || t == "b2" {
			return t
		}
	}
	return ""
}

func (u *UnitreeSDKInterface) detectFromNetworkConfig() string { return "" }
func (u *UnitreeSDKInterface) detectFromSDKResponse() string   { return "" }

// ---------- G1 ----------

func (u *UnitreeSDKInterface) connectG1() error {
	if u.g1DS == nil {
		ds, err := g1.New(u.config)
		if err != nil {
			return fmt.Errorf("创建 G1 数据源失败: %w", err)
		}
		u.g1DS = ds
	}
	if err := u.g1DS.Connect(); err != nil {
		return fmt.Errorf("连接 G1 失败: %w", err)
	}
	u.connected = true
	return nil
}

func (u *UnitreeSDKInterface) disconnectG1() error {
	if u.g1DS != nil {
		_ = u.g1DS.Disconnect()
		_ = u.g1DS.Close()
		u.g1DS = nil
	}
	u.connected = false
	return nil
}

// readG1BMSData 字段映射保持与改造前 internal/types/g1_types.go 完全一致——
// G1 是真实集成不是 stub，必须零行为变化（plan v3 第 3.10 节）。
func (u *UnitreeSDKInterface) readG1BMSData() (*BMSData, error) {
	if u.g1DS == nil {
		return nil, fmt.Errorf("G1 数据源未初始化")
	}
	status, err := u.g1DS.GetBatteryStatus()
	if err != nil {
		return nil, fmt.Errorf("获取 G1 电池状态失败: %w", err)
	}
	return &BMSData{
		Voltage:     status.Voltage,
		Current:     status.Current,
		SOC:         status.Capacity,
		Temperature: status.Temperature,
		Power:       status.Voltage * status.Current,
		Cycles:      float64(status.CycleCount),
		Health:      float64(status.HealthStatus),
	}, nil
}

// ---------- Go2（仍是 stub，不在本次改造范围）----------

func (u *UnitreeSDKInterface) connectGo2() error {
	// TODO: 实际 Go2 SDK 连接
	u.connected = true
	return nil
}

func (u *UnitreeSDKInterface) disconnectGo2() error {
	u.connected = false
	return nil
}

func (u *UnitreeSDKInterface) readGo2BMSData() (*BMSData, error) {
	// TODO: 接通 Go2 真实 SDK；当前返回模拟数据
	return &BMSData{
		Voltage:     24.5,
		Current:     -2.3,
		SOC:         85.6,
		Temperature: 35.2,
		Power:       56.35,
		Cycles:      128,
		Health:      95.8,
	}, nil
}

// ---------- B2 ----------

// b2ConfigFromBMS 从 BMSCollectorConfig 派生一个最小可用的 B2CollectorConfig，
// 让 b2.New 能创建 DataSource。BMS 只读电池，不需要 sport state，但 b2 wrapper 内部
// 会同时订阅 lowstate 和 sportmodestate（codex W-1）。这里给默认 topic 名。
//
// 现场需现场覆盖 topic 名时，应在 collectors.B2 配置块中改（B2 collector 共享同名拓扑）。
// BMS 路径默认走真实 DDS（与 BMS 选 unitree_sdk 的语义一致），
// macOS 测试时通过 RobotType≠"b2" 避开此分支。
func b2ConfigFromBMS(bms *config.BMSCollectorConfig) *config.B2CollectorConfig {
	return &config.B2CollectorConfig{
		DataSource:         "dds",
		NetworkInterface:   bms.NetworkInterface,
		UpdateInterval:     bms.UpdateInterval,
		LowStateTopic:      "rt/lowstate",
		SportStateTopic:    "rt/sportmodestate",
		DDSConnectTimeout:  5 * time.Second,
		DDSStaleThreshold:  5 * time.Second,
		DDSReconnectMinGap: 2 * time.Second,
	}
}

func (u *UnitreeSDKInterface) connectB2() error {
	if u.b2DS == nil {
		ds, err := b2.New(b2ConfigFromBMS(u.config))
		if err != nil {
			return fmt.Errorf("创建 B2 数据源失败: %w", err)
		}
		u.b2DS = ds
	}
	if err := u.b2DS.Connect(); err != nil {
		return fmt.Errorf("连接 B2 失败: %w", err)
	}
	u.connected = true
	return nil
}

func (u *UnitreeSDKInterface) disconnectB2() error {
	if u.b2DS != nil {
		_ = u.b2DS.Disconnect()
		_ = u.b2DS.Close()
		u.b2DS = nil
	}
	u.connected = false
	return nil
}

// readB2BMSData 字段映射：codex C-2 指出 BmsState IDL 不报 health/total_voltage，
// 总电压由 sum(cell_vol) 算，Health 字段保持 0（不假装能读到）。
func (u *UnitreeSDKInterface) readB2BMSData() (*BMSData, error) {
	if u.b2DS == nil {
		return nil, fmt.Errorf("B2 数据源未初始化")
	}
	snap, err := u.b2DS.GetSnapshot()
	if err != nil {
		return nil, fmt.Errorf("获取 B2 snapshot 失败: %w", err)
	}
	if snap == nil || snap.LowState == nil {
		return nil, fmt.Errorf("B2 LowState 尚未收到首包")
	}
	bat := &snap.LowState.Battery

	// 总电压 mV → V（sum 单体电压）
	var sumMv uint64
	for _, v := range bat.CellVoltages {
		sumMv += uint64(v)
	}
	voltage := float64(sumMv) / 1000.0

	// 电流 mA → A（带符号）
	current := float64(bat.Current) / 1000.0

	// 温度多路求平均
	temp := averageTemperature(bat)

	return &BMSData{
		Voltage:     voltage,
		Current:     current,
		SOC:         float64(bat.SOC),
		Temperature: temp,
		Power:       voltage * current,
		Cycles:      float64(bat.Cycle),
		Health:      0, // SDK 不报，留 0（codex C-2）
	}, nil
}

// averageTemperature 把 BQNTC[2] 和 MCUNTC[2] 4 路温度求平均。
// 都为 0 时返回 0；都为有效值时是简单算术平均。
//
// 字段是 uint8（IDL 实际类型），负温度场景无法表示——B2 工业机器人不会在零下运行，
// 0 视为"未读到"哨兵值更稳妥。
func averageTemperature(bat *b2.Battery) float64 {
	var sum uint
	count := 0
	for _, t := range bat.BQNTC {
		if t != 0 {
			sum += uint(t)
			count++
		}
	}
	for _, t := range bat.MCUNTC {
		if t != 0 {
			sum += uint(t)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

// ---------- Serial / CAN / Mock（保持原样，不在改造范围）----------

type SerialInterface struct{ config *config.BMSCollectorConfig }

func NewSerialInterface(cfg *config.BMSCollectorConfig) *SerialInterface {
	return &SerialInterface{config: cfg}
}
func (s *SerialInterface) Connect() error    { return nil }
func (s *SerialInterface) Disconnect() error { return nil }
func (s *SerialInterface) IsConnected() bool { return true }
func (s *SerialInterface) ReadBMSData() (*BMSData, error) {
	return &BMSData{Voltage: 24.2, Current: -1.8, SOC: 82.3, Temperature: 33.8, Power: 43.56, Cycles: 156, Health: 94.2}, nil
}

type CANInterface struct{ config *config.BMSCollectorConfig }

func NewCANInterface(cfg *config.BMSCollectorConfig) *CANInterface {
	return &CANInterface{config: cfg}
}
func (c *CANInterface) Connect() error    { return nil }
func (c *CANInterface) Disconnect() error { return nil }
func (c *CANInterface) IsConnected() bool { return true }
func (c *CANInterface) ReadBMSData() (*BMSData, error) {
	return &BMSData{Voltage: 24.8, Current: -3.1, SOC: 78.9, Temperature: 36.5, Power: 76.88, Cycles: 89, Health: 97.1}, nil
}

type MockInterface struct{ config *config.BMSCollectorConfig }

func NewMockInterface(cfg *config.BMSCollectorConfig) *MockInterface {
	return &MockInterface{config: cfg}
}
func (m *MockInterface) Connect() error    { return nil }
func (m *MockInterface) Disconnect() error { return nil }
func (m *MockInterface) IsConnected() bool { return true }
func (m *MockInterface) ReadBMSData() (*BMSData, error) {
	baseTime := time.Now().Unix()
	return &BMSData{
		Voltage:     24.0 + float64(baseTime%10)/10.0,
		Current:     -2.0 + float64(baseTime%5)/5.0,
		SOC:         80.0 + float64(baseTime%20),
		Temperature: 30.0 + float64(baseTime%15),
		Power:       48.0 + float64(baseTime%20),
		Cycles:      float64(100 + baseTime%50),
		Health:      95.0 + float64(baseTime%5),
	}, nil
}

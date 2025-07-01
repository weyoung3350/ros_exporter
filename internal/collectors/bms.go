package collectors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
	"ros_exporter/internal/types"
)

// BMSData BMS数据结构
type BMSData struct {
	Voltage     float64 // 电压 (V)
	Current     float64 // 电流 (A)
	SOC         float64 // 电量百分比 (%)
	Temperature float64 // 温度 (°C)
	Power       float64 // 功率 (W)
	Cycles      float64 // 充电周期
	Health      float64 // 电池健康度 (%)
}

// BMSInterface BMS接口
type BMSInterface interface {
	Connect() error
	Disconnect() error
	ReadBMSData() (*BMSData, error)
	IsConnected() bool
}

// BMSCollector BMS指标收集器
type BMSCollector struct {
	config    *config.BMSCollectorConfig
	instance  string
	bmsIface  BMSInterface
	connected bool
}

// NewBMSCollector 创建新的BMS收集器
func NewBMSCollector(cfg *config.BMSCollectorConfig, instance string) *BMSCollector {
	collector := &BMSCollector{
		config:   cfg,
		instance: instance,
	}

	// 根据配置创建对应的BMS接口
	switch cfg.InterfaceType {
	case "unitree_sdk":
		collector.bmsIface = NewUnitreeSDKInterface(cfg)
	case "serial":
		collector.bmsIface = NewSerialInterface(cfg)
	case "canbus":
		collector.bmsIface = NewCANInterface(cfg)
	default:
		// 如果接口类型未知，使用模拟接口
		collector.bmsIface = NewMockInterface(cfg)
	}

	return collector
}

// Collect 收集BMS指标
func (c *BMSCollector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	// 确保连接
	if !c.connected {
		if err := c.bmsIface.Connect(); err != nil {
			return nil, fmt.Errorf("连接BMS失败: %w", err)
		}
		c.connected = true
	}

	// 检查连接状态
	if !c.bmsIface.IsConnected() {
		c.connected = false
		return nil, fmt.Errorf("BMS连接断开")
	}

	// 读取BMS数据
	bmsData, err := c.bmsIface.ReadBMSData()
	if err != nil {
		return nil, fmt.Errorf("读取BMS数据失败: %w", err)
	}

	// 转换为指标
	now := time.Now()
	labels := map[string]string{
		"instance":   c.instance,
		"battery_id": "main",
		"interface":  c.config.InterfaceType,
	}

	metrics := []client.Metric{
		{
			Name:      "robot_battery_voltage_volts",
			Value:     bmsData.Voltage,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_current_amperes",
			Value:     bmsData.Current,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_soc_percent",
			Value:     bmsData.SOC,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_temperature_celsius",
			Value:     bmsData.Temperature,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_power_watts",
			Value:     bmsData.Power,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_cycles_total",
			Value:     bmsData.Cycles,
			Labels:    labels,
			Timestamp: now,
		},
		{
			Name:      "robot_battery_health_percent",
			Value:     bmsData.Health,
			Labels:    labels,
			Timestamp: now,
		},
	}

	return metrics, nil
}

// Close 关闭BMS收集器
func (c *BMSCollector) Close() error {
	if c.connected && c.bmsIface != nil {
		c.connected = false
		return c.bmsIface.Disconnect()
	}
	return nil
}

// UnitreeSDKInterface 宇树SDK接口实现
type UnitreeSDKInterface struct {
	config    *config.BMSCollectorConfig
	connected bool
	robotType string       // "go2", "g1", "b2", "auto"
	g1SDK     *types.G1SDK // G1 SDK实例
	b2SDK     *types.B2SDK // B2 SDK实例
}

func NewUnitreeSDKInterface(cfg *config.BMSCollectorConfig) *UnitreeSDKInterface {
	return &UnitreeSDKInterface{
		config:    cfg,
		connected: false,
		robotType: "auto", // 自动检测机器人类型
	}
}

func (u *UnitreeSDKInterface) Connect() error {
	// 检测机器人类型
	if u.robotType == "auto" {
		detectedType, err := u.detectRobotType()
		if err != nil {
			return fmt.Errorf("检测机器人类型失败: %w", err)
		}
		u.robotType = detectedType
	}

	// 根据机器人类型初始化SDK连接
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

	// 根据机器人类型断开连接
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

	// 根据机器人类型读取BMS数据
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

// detectRobotType 检测机器人类型
func (u *UnitreeSDKInterface) detectRobotType() (string, error) {
	// 方法1: 检查系统信息
	if robotType := u.detectFromSystemInfo(); robotType != "" {
		return robotType, nil
	}

	// 方法2: 检查网络接口特征
	if robotType := u.detectFromNetworkConfig(); robotType != "" {
		return robotType, nil
	}

	// 方法3: 尝试连接不同的SDK接口
	if robotType := u.detectFromSDKResponse(); robotType != "" {
		return robotType, nil
	}

	// 默认假设为Go2（向后兼容）
	return "go2", nil
}

// detectFromSystemInfo 从系统信息检测机器人类型
func (u *UnitreeSDKInterface) detectFromSystemInfo() string {
	// 检查主机名
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

	// 检查/etc/robot_type文件（如果存在）
	if data, err := os.ReadFile("/etc/robot_type"); err == nil {
		robotType := strings.TrimSpace(strings.ToLower(string(data)))
		if robotType == "g1" || robotType == "go2" || robotType == "b2" {
			return robotType
		}
	}

	return ""
}

// detectFromNetworkConfig 从网络配置检测机器人类型
func (u *UnitreeSDKInterface) detectFromNetworkConfig() string {
	// G1和Go2可能有不同的默认网络配置
	// 这里可以根据实际情况添加检测逻辑
	return ""
}

// detectFromSDKResponse 从SDK响应检测机器人类型
func (u *UnitreeSDKInterface) detectFromSDKResponse() string {
	// 尝试连接并从响应数据推断机器人类型
	// 这里可以根据实际SDK API的差异来实现
	return ""
}

// connectG1 连接G1机器人
func (u *UnitreeSDKInterface) connectG1() error {
	// 创建G1 SDK实例
	if u.g1SDK == nil {
		u.g1SDK = types.NewG1SDK()
	}

	// 初始化SDK
	sdkConfigPath := ""
	if u.config.SDKConfigPath != "" {
		sdkConfigPath = u.config.SDKConfigPath
	}

	if err := u.g1SDK.Initialize(sdkConfigPath); err != nil {
		return fmt.Errorf("初始化G1 SDK失败: %w", err)
	}

	// 连接到G1机器人
	if err := u.g1SDK.Connect(); err != nil {
		return fmt.Errorf("连接G1机器人失败: %w", err)
	}

	u.connected = true
	return nil
}

// connectGo2 连接Go2机器人
func (u *UnitreeSDKInterface) connectGo2() error {
	// TODO: 实现Go2特定的SDK连接逻辑
	// 这里需要调用宇树Go2 SDK的初始化函数
	// 可以参考现有的C++实现：
	// - unitree::robot::ChannelFactory::Instance()->Init(0, networkInterface)
	// - 订阅LowState消息

	u.connected = true
	return nil
}

// connectB2 连接B2机器人
func (u *UnitreeSDKInterface) connectB2() error {
	// 创建B2 SDK实例
	if u.b2SDK == nil {
		u.b2SDK = types.NewB2SDK()
	}

	// 初始化SDK
	sdkConfigPath := ""
	if u.config.SDKConfigPath != "" {
		sdkConfigPath = u.config.SDKConfigPath
	}

	// 设置网络接口
	networkInterface := "eth0" // 默认接口
	if u.config.NetworkInterface != "" {
		networkInterface = u.config.NetworkInterface
	}

	if err := u.b2SDK.Initialize(sdkConfigPath, networkInterface); err != nil {
		return fmt.Errorf("初始化B2 SDK失败: %w", err)
	}

	// 连接到B2机器人
	if err := u.b2SDK.Connect(); err != nil {
		return fmt.Errorf("连接B2机器人失败: %w", err)
	}

	u.connected = true
	return nil
}

// disconnectG1 断开G1连接
func (u *UnitreeSDKInterface) disconnectG1() error {
	if u.g1SDK != nil {
		if err := u.g1SDK.Disconnect(); err != nil {
			return fmt.Errorf("断开G1连接失败: %w", err)
		}
		u.g1SDK.Cleanup()
		u.g1SDK = nil
	}
	u.connected = false
	return nil
}

// disconnectGo2 断开Go2连接
func (u *UnitreeSDKInterface) disconnectGo2() error {
	// TODO: 实现Go2断开连接逻辑
	u.connected = false
	return nil
}

// disconnectB2 断开B2连接
func (u *UnitreeSDKInterface) disconnectB2() error {
	if u.b2SDK != nil {
		if err := u.b2SDK.Disconnect(); err != nil {
			return fmt.Errorf("断开B2连接失败: %w", err)
		}
		u.b2SDK.Cleanup()
		u.b2SDK = nil
	}
	u.connected = false
	return nil
}

// readG1BMSData 读取G1电池数据
func (u *UnitreeSDKInterface) readG1BMSData() (*BMSData, error) {
	// 使用真实的G1 SDK获取电池数据
	if u.g1SDK == nil {
		return nil, fmt.Errorf("G1 SDK未初始化")
	}

	// 从G1 SDK获取电池状态
	status, err := u.g1SDK.GetBatteryStatus()
	if err != nil {
		return nil, fmt.Errorf("获取G1电池状态失败: %w", err)
	}

	// 转换为BMSData格式
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

// readGo2BMSData 读取Go2电池数据
func (u *UnitreeSDKInterface) readGo2BMSData() (*BMSData, error) {
	// TODO: 实现从Go2读取真实BMS数据
	// 可以参考现有C++实现中的数据获取逻辑
	// state_msg.bms_state().soc() 等

	// 临时返回Go2模拟数据
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

// readB2BMSData 读取B2电池数据
func (u *UnitreeSDKInterface) readB2BMSData() (*BMSData, error) {
	// 使用真实的B2 SDK获取电池数据
	if u.b2SDK == nil {
		return nil, fmt.Errorf("B2 SDK未初始化")
	}

	// 从B2 SDK获取电池状态
	status, err := u.b2SDK.GetBatteryStatus()
	if err != nil {
		return nil, fmt.Errorf("获取B2电池状态失败: %w", err)
	}

	// 转换为BMSData格式（B2的电池规格）
	return &BMSData{
		Voltage:     status.Voltage,                  // 58V标称电压
		Current:     status.Current,                  // 电流
		SOC:         status.Capacity,                 // 电量百分比
		Temperature: status.Temperature,              // 电池温度
		Power:       status.Voltage * status.Current, // 功率
		Cycles:      float64(status.CycleCount),      // 充电周期
		Health:      float64(status.HealthStatus),    // 健康度
	}, nil
}

// SerialInterface 串口接口实现
type SerialInterface struct {
	config *config.BMSCollectorConfig
}

func NewSerialInterface(cfg *config.BMSCollectorConfig) *SerialInterface {
	return &SerialInterface{config: cfg}
}

func (s *SerialInterface) Connect() error {
	// TODO: 实现串口连接逻辑
	// 使用 github.com/tarm/serial 或类似库
	return nil
}

func (s *SerialInterface) Disconnect() error {
	// TODO: 实现串口断开连接逻辑
	return nil
}

func (s *SerialInterface) ReadBMSData() (*BMSData, error) {
	// TODO: 实现从串口读取BMS数据
	// 需要根据具体的BMS协议解析数据
	return &BMSData{
		Voltage:     24.2,
		Current:     -1.8,
		SOC:         82.3,
		Temperature: 33.8,
		Power:       43.56,
		Cycles:      156,
		Health:      94.2,
	}, nil
}

func (s *SerialInterface) IsConnected() bool {
	// TODO: 实现连接状态检查
	return true
}

// CANInterface CAN总线接口实现
type CANInterface struct {
	config *config.BMSCollectorConfig
}

func NewCANInterface(cfg *config.BMSCollectorConfig) *CANInterface {
	return &CANInterface{config: cfg}
}

func (c *CANInterface) Connect() error {
	// TODO: 实现CAN总线连接逻辑
	// 使用 github.com/angelodlfrtr/go-can 或类似库
	return nil
}

func (c *CANInterface) Disconnect() error {
	// TODO: 实现CAN总线断开连接逻辑
	return nil
}

func (c *CANInterface) ReadBMSData() (*BMSData, error) {
	// TODO: 实现从CAN总线读取BMS数据
	// 需要根据具体的CAN协议解析数据
	return &BMSData{
		Voltage:     24.8,
		Current:     -3.1,
		SOC:         78.9,
		Temperature: 36.5,
		Power:       76.88,
		Cycles:      89,
		Health:      97.1,
	}, nil
}

func (c *CANInterface) IsConnected() bool {
	// TODO: 实现连接状态检查
	return true
}

// MockInterface 模拟接口实现（用于测试）
type MockInterface struct {
	config *config.BMSCollectorConfig
}

func NewMockInterface(cfg *config.BMSCollectorConfig) *MockInterface {
	return &MockInterface{config: cfg}
}

func (m *MockInterface) Connect() error {
	return nil
}

func (m *MockInterface) Disconnect() error {
	return nil
}

func (m *MockInterface) ReadBMSData() (*BMSData, error) {
	// 返回模拟的BMS数据，包含一些变化
	baseTime := time.Now().Unix()

	return &BMSData{
		Voltage:     24.0 + float64(baseTime%10)/10.0,
		Current:     -2.0 + float64(baseTime%5)/5.0,
		SOC:         80.0 + float64(baseTime%20),
		Temperature: 30.0 + float64(baseTime%15),
		Power:       48.0 + float64(baseTime%20),
		Cycles:      100 + float64(baseTime%50),
		Health:      95.0 + float64(baseTime%5),
	}, nil
}

func (m *MockInterface) IsConnected() bool {
	return true
}

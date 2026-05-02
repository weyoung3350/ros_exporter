package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Exporter        ExporterConfig        `yaml:"exporter"`
	VictoriaMetrics VictoriaMetricsConfig `yaml:"victoria_metrics"`
	Collectors      CollectorsConfig      `yaml:"collectors"`
}

// ExporterConfig Exporter配置
type ExporterConfig struct {
	PushInterval time.Duration    `yaml:"push_interval"`
	Instance     string           `yaml:"instance"`
	LogLevel     string           `yaml:"log_level"`
	HTTPServer   HTTPServerConfig `yaml:"http_server"`
}

// HTTPServerConfig HTTP服务器配置
type HTTPServerConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Port      int      `yaml:"port"`
	Address   string   `yaml:"address"`
	Endpoints []string `yaml:"endpoints"`
}

// VictoriaMetricsConfig VictoriaMetrics配置
type VictoriaMetricsConfig struct {
	Endpoint    string            `yaml:"endpoint"`
	Timeout     time.Duration     `yaml:"timeout"`
	ExtraLabels map[string]string `yaml:"extra_labels"`
	Retry       RetryConfig       `yaml:"retry"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries  int           `yaml:"max_retries"`
	RetryDelay  time.Duration `yaml:"retry_delay"`
	MaxDelay    time.Duration `yaml:"max_delay"`
	BackoffRate float64       `yaml:"backoff_rate"`
}

// CollectorsConfig 收集器配置
type CollectorsConfig struct {
	System      SystemCollectorConfig      `yaml:"system"`
	BMS         BMSCollectorConfig         `yaml:"bms"`
	ROS         ROSCollectorConfig         `yaml:"ros"`
	B2          B2CollectorConfig          `yaml:"b2"`
	ROSMasterX3 ROSMasterX3CollectorConfig `yaml:"rosmaster_x3"`
}

// SystemCollectorConfig 系统收集器配置
type SystemCollectorConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Collectors []string `yaml:"collectors"`
	ProcPath   string   `yaml:"proc_path"`
	SysPath    string   `yaml:"sys_path"`
	RootfsPath string   `yaml:"rootfs_path"`

	// 温度监控配置
	Temperature TemperatureConfig `yaml:"temperature"`

	// 网络带宽监控配置
	Network NetworkConfig `yaml:"network"`

	// 进程监控配置
	Process ProcessConfig `yaml:"process"`
}

// TemperatureConfig 温度监控配置
type TemperatureConfig struct {
	Enabled     bool   `yaml:"enabled"`
	SensorsCmd  string `yaml:"sensors_cmd"`
	TempSource  string `yaml:"temp_source"`  // "sensors", "thermal_zone"
	ThermalZone string `yaml:"thermal_zone"` // /sys/class/thermal/thermal_zone0/temp
}

// NetworkConfig 网络监控配置
type NetworkConfig struct {
	Enabled          bool     `yaml:"enabled"`
	Interfaces       []string `yaml:"interfaces"`        // 指定监控的网卡接口
	BandwidthEnabled bool     `yaml:"bandwidth_enabled"` // 是否启用实时带宽计算
	ExcludeLoopback  bool     `yaml:"exclude_loopback"`  // 是否排除回环接口
}

// ProcessConfig 进程监控配置
type ProcessConfig struct {
	Enabled         bool     `yaml:"enabled"`          // 是否启用进程监控
	MonitorAll      bool     `yaml:"monitor_all"`      // 是否监控所有进程
	IncludeNames    []string `yaml:"include_names"`    // 包含的进程名（支持正则表达式）
	ExcludeNames    []string `yaml:"exclude_names"`    // 排除的进程名（支持正则表达式）
	IncludeUsers    []string `yaml:"include_users"`    // 包含的用户
	MinCPUPercent   float64  `yaml:"min_cpu_percent"`  // 最小CPU使用率阈值
	MinMemoryMB     float64  `yaml:"min_memory_mb"`    // 最小内存使用阈值(MB)
	CollectDetailed bool     `yaml:"collect_detailed"` // 是否收集详细信息(IO、线程等)
}

// BMSCollectorConfig BMS收集器配置
type BMSCollectorConfig struct {
	Enabled          bool          `yaml:"enabled"`
	InterfaceType    string        `yaml:"interface_type"`    // "unitree_sdk", "serial", "canbus"
	RobotType        string        `yaml:"robot_type"`        // "g1", "go2", "b2", "auto"
	NetworkInterface string        `yaml:"network_interface"` // 网络接口名称，用于DDS通信
	UpdateInterval   time.Duration `yaml:"update_interval"`   // BMS数据更新间隔
	SDKConfigPath    string        `yaml:"sdk_config_path"`   // SDK配置文件路径
	DevicePath       string        `yaml:"device_path"`       // 串口设备路径
	BaudRate         int           `yaml:"baud_rate"`         // 串口波特率
	CanInterface     string        `yaml:"can_interface"`     // CAN接口名称

	// G1DataSource 选择 G1 数据源实现："sdk"（CGO 调真实 unitree SDK）| "mock"（确定性模拟数据）
	// 空字符串时按构建模式自动选：cgo 编译 → sdk，nocgo 编译 → mock。
	// 向后兼容：旧 config.yaml 不写此字段时行为与改造前一致。
	G1DataSource string `yaml:"g1_data_source"`
}

// ROSCollectorConfig ROS收集器配置
type ROSCollectorConfig struct {
	Enabled        bool          `yaml:"enabled"`
	MasterURI      string        `yaml:"master_uri"`
	TopicWhitelist []string      `yaml:"topic_whitelist"`
	TopicBlacklist []string      `yaml:"topic_blacklist"`
	NodeWhitelist  []string      `yaml:"node_whitelist"`
	NodeBlacklist  []string      `yaml:"node_blacklist"`
	ScrapeInterval time.Duration `yaml:"scrape_interval"`
}

// B2CollectorConfig B2 四足机器人专用收集器配置。
//
// 数据源选择由 DataSource 字段决定，不再依赖 SDKConfigPath（v3 移除）。
// 告警阈值字段（MaxJointTemp 等）已移除，改由 VictoriaMetrics/Grafana alert rules 负责，
// 避免在 exporter 和告警系统中双重定义阈值。
type B2CollectorConfig struct {
	Enabled bool `yaml:"enabled"`

	// DataSource 选择 B2 数据源实现："dds"（CGO + unitree_sdk2 直连，仅 cgo+linux）| "mock"。
	// 空字符串默认 "mock"（向后兼容）。
	// macOS / 无 SDK 环境配置 "dds" 启动会报错（dds_stub.go 返回明确错误，不静默 mock）。
	DataSource string `yaml:"data_source"`

	RobotID          string        `yaml:"robot_id"`          // 机器人标识 ID（写入 metric label）
	NetworkInterface string        `yaml:"network_interface"` // DDS 通信网卡（AGX-Thor 现场默认 enP2p1s0）
	UpdateInterval   time.Duration `yaml:"update_interval"`   // collector 拉取 snapshot 的间隔

	// LowStateTopic / SportStateTopic：DDS topic 名，现场用 ros2 topic list 验证后可覆盖默认值。
	// B2 的 sportmodestate 在 unitree_sdk2 B2 示例中没直接订阅过，可能是 lf/sportmodestate 等变体。
	LowStateTopic   string `yaml:"low_state_topic"`
	SportStateTopic string `yaml:"sport_state_topic"`

	// DDS 自愈参数：wrapper 守护线程检测每个 topic 的 last_seen，超 DDSStaleThreshold 视为断流并重建。
	DDSStaleThreshold  time.Duration `yaml:"dds_stale_threshold"`   // 默认 5s
	DDSConnectTimeout  time.Duration `yaml:"dds_connect_timeout"`   // 默认 5s，首包等待超时
	DDSReconnectMinGap time.Duration `yaml:"dds_reconnect_min_gap"` // 默认 2s，重建 subscriber 最小间隔（防抖）

	// 监控开关：collector 按这些选择性输出指标。snapshot 中字段不可用时也不会输出对应指标。
	MonitorMotion  bool `yaml:"monitor_motion"`
	MonitorIMU     bool `yaml:"monitor_imu"`
	MonitorJoints  bool `yaml:"monitor_joints"`
	MonitorBattery bool `yaml:"monitor_battery"` // 是否输出 b2_battery_* 详细指标（cell voltage 等）。robot_battery_* 跨机器人统一指标始终走 BMS collector
	MonitorSafety  bool `yaml:"monitor_safety"`
}

// ROSMasterX3CollectorConfig ROSMaster-X3收集器配置
type ROSMasterX3CollectorConfig struct {
	Enabled       bool          `yaml:"enabled"`
	MasterURI     string        `yaml:"master_uri"`     // ROS Master URI
	RobotID       string        `yaml:"robot_id"`       // 机器人标识ID
	UpdateInterval time.Duration `yaml:"update_interval"` // 数据更新间隔

	// 监控配置
	MonitorMotors     bool `yaml:"monitor_motors"`     // 是否监控电机状态
	MonitorBattery    bool `yaml:"monitor_battery"`    // 是否监控电池状态
	MonitorLidar      bool `yaml:"monitor_lidar"`      // 是否监控激光雷达
	MonitorIMU        bool `yaml:"monitor_imu"`        // 是否监控IMU
	MonitorNavigation bool `yaml:"monitor_navigation"` // 是否监控导航状态
	MonitorCamera     bool `yaml:"monitor_camera"`     // 是否监控相机

	// 话题过滤配置
	TopicWhitelist []string `yaml:"topic_whitelist"` // 话题白名单
	TopicBlacklist []string `yaml:"topic_blacklist"` // 话题黑名单

	// 告警阈值
	MaxMotorTemp      float64 `yaml:"max_motor_temp"`      // 电机最高温度阈值 (°C)
	MaxBatteryTemp    float64 `yaml:"max_battery_temp"`    // 电池最高温度阈值 (°C)
	MinBatteryVoltage float64 `yaml:"min_battery_voltage"` // 电池最低电压阈值 (V)
	MinBatterySOC     float64 `yaml:"min_battery_soc"`     // 电池最低电量阈值 (%)
	MaxLinearVelocity float64 `yaml:"max_linear_velocity"` // 最大线性速度阈值 (m/s)
	MaxAngularVelocity float64 `yaml:"max_angular_velocity"` // 最大角速度阈值 (rad/s)
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Exporter: ExporterConfig{
			PushInterval: 15 * time.Second,
			Instance:     getHostname(),
			LogLevel:     "info",
			HTTPServer: HTTPServerConfig{
				Enabled:   true,
				Port:      9100,
				Address:   "127.0.0.1",
				Endpoints: []string{"health", "status", "metrics"},
			},
		},
		VictoriaMetrics: VictoriaMetricsConfig{
			Endpoint: "http://localhost:8428/api/v1/import/prometheus",
			Timeout:  30 * time.Second,
			ExtraLabels: map[string]string{
				"job": "ros_exporter",
			},
			Retry: RetryConfig{
				MaxRetries:  3,
				RetryDelay:  1 * time.Second,
				MaxDelay:    30 * time.Second,
				BackoffRate: 2.0,
			},
		},
		Collectors: CollectorsConfig{
			System: SystemCollectorConfig{
				Enabled: true,
				Collectors: []string{
					"cpu", "memory", "disk", "network", "load",
				},
				ProcPath:   "/proc",
				SysPath:    "/sys",
				RootfsPath: "/",
				Temperature: TemperatureConfig{
					Enabled:     true,
					SensorsCmd:  "sensors",
					TempSource:  "sensors", // 默认使用sensors命令
					ThermalZone: "/sys/class/thermal/thermal_zone0/temp",
				},
				Network: NetworkConfig{
					Enabled:          true,
					Interfaces:       []string{}, // 空表示监控所有接口
					BandwidthEnabled: true,
					ExcludeLoopback:  true,
				},
				Process: ProcessConfig{
					Enabled:         false,                                                        // 默认禁用，需要时手动启用
					MonitorAll:      false,                                                        // 默认不监控所有进程
					IncludeNames:    []string{},                                                   // 默认无包含列表
					ExcludeNames:    []string{"kthreadd", "ksoftirqd.*", "migration.*", "rcu_.*"}, // 排除内核线程
					IncludeUsers:    []string{},                                                   // 默认无用户过滤
					MinCPUPercent:   1.0,                                                          // 最小CPU使用率1%
					MinMemoryMB:     10.0,                                                         // 最小内存使用10MB
					CollectDetailed: false,                                                        // 默认不收集详细信息
				},
			},
			BMS: BMSCollectorConfig{
				Enabled:          true,
				InterfaceType:    "unitree_sdk",
				RobotType:        "auto", // 自动检测机器人类型
				NetworkInterface: "eth0", // 默认网络接口
				UpdateInterval:   5 * time.Second,
				DevicePath:       "/dev/ttyUSB0",
				BaudRate:         115200,
				CanInterface:     "can0",
			},
			ROS: ROSCollectorConfig{
				Enabled:        true,
				MasterURI:      "http://localhost:11311",
				TopicWhitelist: []string{},
				TopicBlacklist: []string{"/rosout", "/rosout_agg"},
				NodeWhitelist:  []string{},
				NodeBlacklist:  []string{"/rosout"},
				ScrapeInterval: 5 * time.Second,
			},
			B2: B2CollectorConfig{
				Enabled:          false,    // 默认禁用，只在 B2 机器人上启用
				DataSource:       "mock",   // 默认 mock，避免无 DDS 环境因尝试连真 SDK 而启动失败
				RobotID:          "b2-001",
				NetworkInterface: "enP2p1s0", // AGX-Thor 现场主网卡（非 eth0）
				UpdateInterval:   5 * time.Second,

				LowStateTopic:   "rt/lowstate",
				SportStateTopic: "rt/sportmodestate",

				DDSStaleThreshold:  5 * time.Second,
				DDSConnectTimeout:  5 * time.Second,
				DDSReconnectMinGap: 2 * time.Second,

				MonitorMotion:  true,
				MonitorIMU:     true,
				MonitorJoints:  true,
				MonitorBattery: true,
				MonitorSafety:  true,
			},
			ROSMasterX3: ROSMasterX3CollectorConfig{
				Enabled:        false, // 默认禁用，只在ROSMaster-X3机器人上启用
				MasterURI:      "http://localhost:11311",
				RobotID:        "rosmaster-x3-001",
				UpdateInterval: 5 * time.Second,

				// 监控配置
				MonitorMotors:     true,
				MonitorBattery:    true,
				MonitorLidar:      true,
				MonitorIMU:        true,
				MonitorNavigation: true,
				MonitorCamera:     true,

				// 话题过滤配置
				TopicWhitelist: []string{},
				TopicBlacklist: []string{"/rosout", "/rosout_agg", "/tf_static"},

				// 告警阈值
				MaxMotorTemp:       75.0,  // 电机最高温度75°C
				MaxBatteryTemp:     60.0,  // 电池最高温度60°C
				MinBatteryVoltage:  11.0,  // 电池最低电压11V
				MinBatterySOC:      20.0,  // 电池最低电量20%
				MaxLinearVelocity:  2.0,   // 最大线性速度2m/s
				MaxAngularVelocity: 2.0,   // 最大角速度2rad/s
			},
		},
	}
}

// Load 从文件加载配置
func Load(filename string) (*Config, error) {
	// 如果文件不存在，创建默认配置文件
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		defaultCfg := DefaultConfig()
		if err := Save(filename, defaultCfg); err != nil {
			return nil, fmt.Errorf("创建默认配置文件失败: %w", err)
		}
		return defaultCfg, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 标准化配置
	if err := normalizeConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置标准化失败: %w", err)
	}

	return cfg, nil
}

// Save 保存配置到文件
func Save(filename string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// getHostname 获取主机名
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// normalizeConfig 标准化配置，处理需要动态值的配置项
func normalizeConfig(cfg *Config) error {
	// 处理instance字段：如果为空、"auto"或"AUTO"，则使用主机名
	if cfg.Exporter.Instance == "" ||
		cfg.Exporter.Instance == "auto" ||
		cfg.Exporter.Instance == "AUTO" {
		cfg.Exporter.Instance = getHostname()
	}

	return nil
}

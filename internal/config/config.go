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

// ExporterConfig 导出器配置
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
	System SystemCollectorConfig `yaml:"system"`
	BMS    BMSCollectorConfig    `yaml:"bms"`
	ROS    ROSCollectorConfig    `yaml:"ros"`
	B2     B2CollectorConfig     `yaml:"b2"`
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

// B2CollectorConfig B2机器狗专用收集器配置
type B2CollectorConfig struct {
	Enabled          bool          `yaml:"enabled"`
	RobotID          string        `yaml:"robot_id"`          // 机器人标识ID
	NetworkInterface string        `yaml:"network_interface"` // 网络接口名称
	SDKConfigPath    string        `yaml:"sdk_config_path"`   // SDK配置文件路径
	UpdateInterval   time.Duration `yaml:"update_interval"`   // 数据更新间隔

	// 监控配置
	MonitorJoints  bool `yaml:"monitor_joints"`  // 是否监控关节状态
	MonitorSensors bool `yaml:"monitor_sensors"` // 是否监控传感器状态
	MonitorMotion  bool `yaml:"monitor_motion"`  // 是否监控运动状态
	MonitorSafety  bool `yaml:"monitor_safety"`  // 是否监控安全状态

	// 告警阈值
	MaxJointTemp           float64 `yaml:"max_joint_temp"`           // 关节最高温度阈值 (°C)
	MaxLoadWeight          float64 `yaml:"max_load_weight"`          // 最大负载阈值 (kg)
	MaxSpeed               float64 `yaml:"max_speed"`                // 最大速度阈值 (m/s)
	CollisionRiskThreshold float64 `yaml:"collision_risk_threshold"` // 碰撞风险阈值
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
				Enabled:          false, // 默认禁用，只在B2机器人上启用
				RobotID:          "b2-001",
				NetworkInterface: "eth0",
				UpdateInterval:   5 * time.Second,

				// 监控配置
				MonitorJoints:  true,
				MonitorSensors: true,
				MonitorMotion:  true,
				MonitorSafety:  true,

				// 告警阈值
				MaxJointTemp:           80.0,  // 关节温度上限80°C
				MaxLoadWeight:          100.0, // 负载警告阈值100kg（最大120kg）
				MaxSpeed:               5.0,   // 速度警告阈值5m/s（最大6m/s）
				CollisionRiskThreshold: 0.8,   // 碰撞风险阈值0.8
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

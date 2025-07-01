//go:build !cgo
// +build !cgo

package types

import (
	"errors"
	"time"
)

// G1BatteryStatus Go语言版本的电池状态结构 (无CGO版本)
type G1BatteryStatus struct {
	// 基础电池信息
	Voltage     float64   `json:"voltage"`      // 总电压 (V)
	Current     float64   `json:"current"`      // 电流 (A)
	Temperature float64   `json:"temperature"`  // 平均温度 (°C)
	Capacity    float64   `json:"capacity"`     // 剩余容量 (%)
	CycleCount  uint32    `json:"cycle_count"`  // 循环次数
	
	// 单体电压 (40节电池)
	CellVoltages []float64 `json:"cell_voltages"` // 单体电压数组
	
	// 温度传感器 (12个)
	Temperatures []float64 `json:"temperatures"`  // 温度传感器数组
	
	// 状态标志
	IsCharging    bool   `json:"is_charging"`    // 充电状态
	IsDischarging bool   `json:"is_discharging"` // 放电状态
	HealthStatus  uint8  `json:"health_status"`  // 健康状态 (0-100)
	
	// 错误状态
	ErrorCode    uint32 `json:"error_code"`    // 错误代码
	ErrorMessage string `json:"error_message"` // 错误信息
	
	// 时间戳
	Timestamp time.Time `json:"timestamp"` // 数据时间戳
}

// G1SDK Go语言的SDK接口封装 (无CGO版本)
type G1SDK struct {
	initialized bool
	connected   bool
	mockMode    bool
}

// NewG1SDK 创建新的G1SDK实例
func NewG1SDK() *G1SDK {
	return &G1SDK{
		initialized: false,
		connected:   false,
		mockMode:    true, // 无CGO版本使用模拟模式
	}
}

// Initialize 初始化SDK (模拟实现)
func (sdk *G1SDK) Initialize(configPath string) error {
	if sdk.initialized {
		return nil
	}
	
	// 模拟初始化成功
	sdk.initialized = true
	return nil
}

// Cleanup 清理SDK资源
func (sdk *G1SDK) Cleanup() {
	if !sdk.initialized {
		return
	}
	
	sdk.Disconnect()
	sdk.initialized = false
}

// Connect 连接到机器人 (模拟实现)
func (sdk *G1SDK) Connect() error {
	if !sdk.initialized {
		return errors.New("SDK未初始化")
	}
	
	if sdk.connected {
		return nil
	}
	
	// 模拟连接成功
	sdk.connected = true
	return nil
}

// Disconnect 断开连接
func (sdk *G1SDK) Disconnect() error {
	if !sdk.connected {
		return nil
	}
	
	sdk.connected = false
	return nil
}

// IsConnected 检查连接状态
func (sdk *G1SDK) IsConnected() bool {
	return sdk.connected
}

// GetBatteryStatus 获取电池状态 (模拟数据)
func (sdk *G1SDK) GetBatteryStatus() (*G1BatteryStatus, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到机器人")
	}
	
	// 返回模拟的电池状态数据
	status := &G1BatteryStatus{
		Voltage:       25.2,  // 6S电池组电压
		Current:       -2.5,  // 放电电流
		Temperature:   35.0,  // 温度
		Capacity:      87.5,  // 电量87.5%
		CycleCount:    245,   // 循环次数
		IsCharging:    false,
		IsDischarging: true,
		HealthStatus:  92,    // 健康度92%
		ErrorCode:     0,     // 无错误
		ErrorMessage:  "",
		Timestamp:     time.Now(),
	}
	
	// 生成40节单体电压 (3.9V - 4.2V)
	status.CellVoltages = make([]float64, 40)
	for i := 0; i < 40; i++ {
		status.CellVoltages[i] = 4.05 + float64(i%10)*0.01 // 4.05V - 4.14V
	}
	
	// 生成12个温度传感器数据 (30°C - 40°C)
	status.Temperatures = make([]float64, 12)
	for i := 0; i < 12; i++ {
		status.Temperatures[i] = 32.0 + float64(i%8)*0.5 // 32°C - 35.5°C
	}
	
	return status, nil
}

// BatteryMetrics 电池监控指标
type BatteryMetrics struct {
	// 基础指标
	VoltageGauge     float64 `json:"voltage_gauge"`
	CurrentGauge     float64 `json:"current_gauge"`
	TemperatureGauge float64 `json:"temperature_gauge"`
	CapacityGauge    float64 `json:"capacity_gauge"`
	CycleCountGauge  float64 `json:"cycle_count_gauge"`
	HealthGauge      float64 `json:"health_gauge"`
	
	// 状态指标
	ChargingStatus    float64 `json:"charging_status"`
	DischargingStatus float64 `json:"discharging_status"`
	
	// 单体电压指标
	CellVoltageMin  float64 `json:"cell_voltage_min"`
	CellVoltageMax  float64 `json:"cell_voltage_max"`
	CellVoltageAvg  float64 `json:"cell_voltage_avg"`
	CellVoltageDiff float64 `json:"cell_voltage_diff"`
	
	// 温度指标
	TemperatureMin  float64 `json:"temperature_min"`
	TemperatureMax  float64 `json:"temperature_max"`
	TemperatureAvg  float64 `json:"temperature_avg"`
	TemperatureDiff float64 `json:"temperature_diff"`
	
	// 错误指标
	ErrorStatus float64 `json:"error_status"`
	
	// 时间戳
	Timestamp int64 `json:"timestamp"`
}

// ToMetrics 将电池状态转换为监控指标
func (status *G1BatteryStatus) ToMetrics() *BatteryMetrics {
	metrics := &BatteryMetrics{
		VoltageGauge:     status.Voltage,
		CurrentGauge:     status.Current,
		TemperatureGauge: status.Temperature,
		CapacityGauge:    status.Capacity,
		CycleCountGauge:  float64(status.CycleCount),
		HealthGauge:      float64(status.HealthStatus),
		ChargingStatus:   boolToFloat(status.IsCharging),
		DischargingStatus: boolToFloat(status.IsDischarging),
		ErrorStatus:      float64(status.ErrorCode),
		Timestamp:        status.Timestamp.Unix(),
	}
	
	// 计算单体电压统计
	if len(status.CellVoltages) > 0 {
		min, max, sum := status.CellVoltages[0], status.CellVoltages[0], 0.0
		for _, v := range status.CellVoltages {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += v
		}
		metrics.CellVoltageMin = min
		metrics.CellVoltageMax = max
		metrics.CellVoltageAvg = sum / float64(len(status.CellVoltages))
		metrics.CellVoltageDiff = max - min
	}
	
	// 计算温度统计
	if len(status.Temperatures) > 0 {
		min, max, sum := status.Temperatures[0], status.Temperatures[0], 0.0
		for _, t := range status.Temperatures {
			if t < min {
				min = t
			}
			if t > max {
				max = t
			}
			sum += t
		}
		metrics.TemperatureMin = min
		metrics.TemperatureMax = max
		metrics.TemperatureAvg = sum / float64(len(status.Temperatures))
		metrics.TemperatureDiff = max - min
	}
	
	return metrics
}

// boolToFloat 将布尔值转换为浮点数
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// GetHealthLevel 获取电池健康等级
func (status *G1BatteryStatus) GetHealthLevel() string {
	switch {
	case status.HealthStatus >= 90:
		return "优秀"
	case status.HealthStatus >= 80:
		return "良好"
	case status.HealthStatus >= 70:
		return "一般"
	case status.HealthStatus >= 60:
		return "较差"
	default:
		return "危险"
	}
}

// GetCapacityLevel 获取电量等级
func (status *G1BatteryStatus) GetCapacityLevel() string {
	switch {
	case status.Capacity >= 80:
		return "充足"
	case status.Capacity >= 60:
		return "良好"
	case status.Capacity >= 40:
		return "中等"
	case status.Capacity >= 20:
		return "较低"
	default:
		return "危险"
	}
}

// HasCriticalError 检查是否有严重错误
func (status *G1BatteryStatus) HasCriticalError() bool {
	// 检查严重错误条件
	if status.ErrorCode != 0 {
		return true
	}
	if status.Capacity < 10 {
		return true
	}
	if status.HealthStatus < 50 {
		return true
	}
	if status.Temperature > 60 || status.Temperature < -10 {
		return true
	}
	
	// 检查单体电压异常
	for _, v := range status.CellVoltages {
		if v < 3.0 || v > 4.5 {
			return true
		}
	}
	
	return false
} 
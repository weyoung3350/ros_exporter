package types

import (
	"errors"
	"fmt"
	"time"
)

// B2MotionState B2运动状态
type B2MotionState struct {
	CurrentSpeed  float64 `json:"current_speed"`  // 当前速度 (m/s)
	LoadWeight    float64 `json:"load_weight"`    // 当前负载 (kg)
	SlopeAngle    float64 `json:"slope_angle"`    // 坡度角度 (度)
	TerrainType   string  `json:"terrain_type"`   // 地形类型
	WorkMode      string  `json:"work_mode"`      // 工作模式
	GaitMode      string  `json:"gait_mode"`      // 步态模式
	Timestamp     time.Time `json:"timestamp"`    // 时间戳
}

// B2SensorState B2传感器状态
type B2SensorState struct {
	LidarOnline       bool      `json:"lidar_online"`        // 激光雷达在线状态
	CameraOnline      bool      `json:"camera_online"`       // 相机在线状态
	DepthCameraOnline bool      `json:"depth_camera_online"` // 深度相机在线状态
	ObstacleDetected  bool      `json:"obstacle_detected"`   // 障碍物检测
	Timestamp         time.Time `json:"timestamp"`           // 时间戳
}

// B2JointState B2关节状态
type B2JointState struct {
	Temperatures []float64 `json:"temperatures"` // 关节温度数组 (12个关节)
	Torques      []float64 `json:"torques"`      // 关节扭矩数组 (N.m)
	Angles       []float64 `json:"angles"`       // 关节角度数组 (度)
	Timestamp    time.Time `json:"timestamp"`    // 时间戳
}

// B2SafetyState B2安全状态
type B2SafetyState struct {
	EmergencyStop   bool      `json:"emergency_stop"`   // 急停状态
	CollisionRisk   float64   `json:"collision_risk"`   // 碰撞风险 (0-1)
	StabilityScore  float64   `json:"stability_score"`  // 稳定性评分 (0-1)
	Timestamp       time.Time `json:"timestamp"`        // 时间戳
}

// B2BatteryStatus B2电池状态（工业级58V系统）
type B2BatteryStatus struct {
	Voltage      float64   `json:"voltage"`       // 电压 (V) - 58V标称
	Current      float64   `json:"current"`       // 电流 (A)
	Temperature  float64   `json:"temperature"`   // 温度 (°C)
	Capacity     float64   `json:"capacity"`      // 电量百分比 (%)
	CycleCount   uint32    `json:"cycle_count"`   // 充电周期
	HealthStatus uint8     `json:"health_status"` // 健康状态 (0-100)
	IsCharging   bool      `json:"is_charging"`   // 充电状态
	ErrorCode    uint32    `json:"error_code"`    // 错误代码
	Timestamp    time.Time `json:"timestamp"`     // 时间戳
}

// B2SDK B2机器狗SDK接口
type B2SDK struct {
	initialized bool
	connected   bool
	networkInterface string
	configPath  string
}

// NewB2SDK 创建新的B2SDK实例
func NewB2SDK() *B2SDK {
	return &B2SDK{
		initialized: false,
		connected:   false,
	}
}

// Initialize 初始化B2 SDK
func (sdk *B2SDK) Initialize(configPath, networkInterface string) error {
	if sdk.initialized {
		return nil // 已初始化
	}
	
	sdk.configPath = configPath
	sdk.networkInterface = networkInterface
	
	// TODO: 实际的B2 SDK初始化
	// 这里需要调用宇树B2的实际SDK接口
	// 可能类似于：
	// result := unitree_b2_sdk_init(configPath, networkInterface)
	
	sdk.initialized = true
	return nil
}

// Connect 连接到B2机器人
func (sdk *B2SDK) Connect() error {
	if !sdk.initialized {
		return errors.New("SDK未初始化")
	}
	
	if sdk.connected {
		return nil // 已连接
	}
	
	// TODO: 实际的B2连接逻辑
	// 这里需要调用宇树B2的实际连接接口
	// 可能类似于：
	// result := unitree_b2_sdk_connect()
	
	sdk.connected = true
	return nil
}

// Disconnect 断开B2连接
func (sdk *B2SDK) Disconnect() error {
	if !sdk.connected {
		return nil
	}
	
	// TODO: 实际的B2断开连接逻辑
	// result := unitree_b2_sdk_disconnect()
	
	sdk.connected = false
	return nil
}

// Cleanup 清理B2 SDK资源
func (sdk *B2SDK) Cleanup() {
	if !sdk.initialized {
		return
	}
	
	sdk.Disconnect()
	// TODO: 实际的B2清理逻辑
	// unitree_b2_sdk_cleanup()
	
	sdk.initialized = false
}

// IsConnected 检查B2连接状态
func (sdk *B2SDK) IsConnected() bool {
	return sdk.connected
}

// GetMotionState 获取B2运动状态
func (sdk *B2SDK) GetMotionState() (*B2MotionState, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到B2机器人")
	}
	
	// TODO: 实际的B2运动状态获取
	// 这里需要调用B2 SDK的运动状态接口
	
	// 临时返回模拟数据用于测试
	return &B2MotionState{
		CurrentSpeed: 2.5,
		LoadWeight:   45.0,
		SlopeAngle:   15.0,
		TerrainType:  "rough",
		WorkMode:     "patrol",
		GaitMode:     "trot",
		Timestamp:    time.Now(),
	}, nil
}

// GetSensorState 获取B2传感器状态
func (sdk *B2SDK) GetSensorState() (*B2SensorState, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到B2机器人")
	}
	
	// TODO: 实际的B2传感器状态获取
	
	return &B2SensorState{
		LidarOnline:       true,
		CameraOnline:      true,
		DepthCameraOnline: true,
		ObstacleDetected:  false,
		Timestamp:         time.Now(),
	}, nil
}

// GetJointState 获取B2关节状态
func (sdk *B2SDK) GetJointState() (*B2JointState, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到B2机器人")
	}
	
	// TODO: 实际的B2关节状态获取
	
	// 模拟12个关节的数据
	temperatures := make([]float64, 12)
	torques := make([]float64, 12)
	angles := make([]float64, 12)
	
	for i := 0; i < 12; i++ {
		temperatures[i] = 45.0 + float64(i) // 45-56°C
		torques[i] = 150.0 + float64(i*10)  // 150-260 N.m
		angles[i] = float64(i * 15)         // 0-165度
	}
	
	return &B2JointState{
		Temperatures: temperatures,
		Torques:      torques,
		Angles:       angles,
		Timestamp:    time.Now(),
	}, nil
}

// GetSafetyState 获取B2安全状态
func (sdk *B2SDK) GetSafetyState() (*B2SafetyState, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到B2机器人")
	}
	
	// TODO: 实际的B2安全状态获取
	
	return &B2SafetyState{
		EmergencyStop:  false,
		CollisionRisk:  0.2,
		StabilityScore: 0.9,
		Timestamp:      time.Now(),
	}, nil
}

// GetBatteryStatus 获取B2电池状态
func (sdk *B2SDK) GetBatteryStatus() (*B2BatteryStatus, error) {
	if !sdk.IsConnected() {
		return nil, errors.New("未连接到B2机器人")
	}
	
	// TODO: 实际的B2电池状态获取
	
	return &B2BatteryStatus{
		Voltage:      57.8,  // 58V系统
		Current:      -5.2,  // 放电电流
		Temperature:  38.5,  // 电池温度
		Capacity:     72.0,  // 电量百分比
		CycleCount:   156,   // 充电周期
		HealthStatus: 95,    // 健康状态
		IsCharging:   false, // 非充电状态
		ErrorCode:    0,     // 无错误
		Timestamp:    time.Now(),
	}, nil
}

// ValidateMotionState 验证B2运动状态
func (state *B2MotionState) ValidateMotionState() error {
	if state.CurrentSpeed < 0 || state.CurrentSpeed > 6.5 {
		return fmt.Errorf("速度超出范围: %.2f m/s", state.CurrentSpeed)
	}
	
	if state.LoadWeight < 0 || state.LoadWeight > 120 {
		return fmt.Errorf("负载超出范围: %.2f kg", state.LoadWeight)
	}
	
	if state.SlopeAngle < -45 || state.SlopeAngle > 45 {
		return fmt.Errorf("坡度角度超出范围: %.2f°", state.SlopeAngle)
	}
	
	return nil
}

// ValidateJointState 验证B2关节状态
func (state *B2JointState) ValidateJointState() error {
	if len(state.Temperatures) != 12 {
		return fmt.Errorf("关节温度数据不完整: %d/12", len(state.Temperatures))
	}
	
	if len(state.Torques) != 12 {
		return fmt.Errorf("关节扭矩数据不完整: %d/12", len(state.Torques))
	}
	
	if len(state.Angles) != 12 {
		return fmt.Errorf("关节角度数据不完整: %d/12", len(state.Angles))
	}
	
	// 检查温度范围
	for i, temp := range state.Temperatures {
		if temp < -20 || temp > 85 {
			return fmt.Errorf("关节%d温度异常: %.2f°C", i, temp)
		}
	}
	
	// 检查扭矩范围
	for i, torque := range state.Torques {
		if torque < -360 || torque > 360 {
			return fmt.Errorf("关节%d扭矩异常: %.2f N.m", i, torque)
		}
	}
	
	return nil
}

// GetMaxJointTemperature 获取最高关节温度
func (state *B2JointState) GetMaxJointTemperature() float64 {
	if len(state.Temperatures) == 0 {
		return 0
	}
	
	max := state.Temperatures[0]
	for _, temp := range state.Temperatures {
		if temp > max {
			max = temp
		}
	}
	return max
}

// GetMaxJointTorque 获取最大关节扭矩
func (state *B2JointState) GetMaxJointTorque() float64 {
	if len(state.Torques) == 0 {
		return 0
	}
	
	max := state.Torques[0]
	for _, torque := range state.Torques {
		if abs(torque) > abs(max) {
			max = torque
		}
	}
	return max
}

// abs 计算绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// HasCriticalAlert 检查是否有严重告警
func (state *B2SafetyState) HasCriticalAlert() bool {
	return state.EmergencyStop || 
		   state.CollisionRisk > 0.8 || 
		   state.StabilityScore < 0.3
}

// GetSafetyLevel 获取安全等级
func (state *B2SafetyState) GetSafetyLevel() string {
	if state.EmergencyStop {
		return "紧急停止"
	}
	
	if state.CollisionRisk > 0.8 {
		return "高风险"
	}
	
	if state.CollisionRisk > 0.6 || state.StabilityScore < 0.5 {
		return "中风险"
	}
	
	if state.CollisionRisk > 0.3 || state.StabilityScore < 0.7 {
		return "低风险"
	}
	
	return "安全"
} 
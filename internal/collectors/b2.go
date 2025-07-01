package collectors

import (
	"context"
	"fmt"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
	"ros_exporter/internal/types"
)

// B2Data B2机器狗特有数据结构
type B2Data struct {
	// 运动性能
	Speed         float64 // 当前速度 (m/s)
	MaxSpeed      float64 // 最大速度能力
	LoadWeight    float64 // 当前负载重量 (kg)
	MaxLoadWeight float64 // 最大负载能力

	// 关节状态
	JointTemps   []float64 // 各关节温度
	JointTorques []float64 // 各关节扭矩 (N.m)
	JointAngles  []float64 // 各关节角度

	// 传感器状态
	LidarStatus    bool // 3D激光雷达状态
	CameraStatus   bool // 相机状态
	DepthCamStatus bool // 深度相机状态

	// 环境感知
	ObstacleDetected bool    // 障碍物检测
	SlopeAngle       float64 // 当前坡度角度
	TerrainType      string  // 地形类型

	// 工作模式
	WorkMode string // 工作模式 (patrol, inspection, manual)
	GaitMode string // 步态模式 (walk, trot, run)

	// 安全状态
	EmergencyStop  bool    // 急停状态
	CollisionRisk  float64 // 碰撞风险评分 (0-1)
	StabilityScore float64 // 稳定性评分 (0-1)
}

// B2Collector B2专用收集器
type B2Collector struct {
	config    *config.B2CollectorConfig
	instance  string
	b2SDK     *types.B2SDK
	connected bool
}

// NewB2Collector 创建新的B2收集器
func NewB2Collector(cfg *config.B2CollectorConfig, instance string) *B2Collector {
	return &B2Collector{
		config:   cfg,
		instance: instance,
	}
}

// Collect 收集B2指标
func (c *B2Collector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	// 确保连接
	if !c.connected {
		if err := c.connect(); err != nil {
			return nil, fmt.Errorf("连接B2失败: %w", err)
		}
	}

	// 读取B2数据
	b2Data, err := c.readB2Data()
	if err != nil {
		return nil, fmt.Errorf("读取B2数据失败: %w", err)
	}

	// 转换为指标
	now := time.Now()
	var metrics []client.Metric

	// 基础标签
	baseLabels := map[string]string{
		"instance":   c.instance,
		"robot_type": "b2",
		"robot_id":   c.config.RobotID,
	}

	// 运动性能指标
	metrics = append(metrics,
		client.Metric{
			Name:      "b2_current_speed_mps",
			Value:     b2Data.Speed,
			Labels:    addLabel(baseLabels, "type", "motion"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_max_speed_capability_mps",
			Value:     b2Data.MaxSpeed,
			Labels:    addLabel(baseLabels, "type", "capability"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_load_weight_kg",
			Value:     b2Data.LoadWeight,
			Labels:    addLabel(baseLabels, "type", "load"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_max_load_capability_kg",
			Value:     b2Data.MaxLoadWeight,
			Labels:    addLabel(baseLabels, "type", "capability"),
			Timestamp: now,
		},
	)

	// 关节状态指标
	for i, temp := range b2Data.JointTemps {
		jointLabels := addLabel(baseLabels, "joint_id", fmt.Sprintf("joint_%d", i))
		metrics = append(metrics, client.Metric{
			Name:      "b2_joint_temperature_celsius",
			Value:     temp,
			Labels:    jointLabels,
			Timestamp: now,
		})
	}

	for i, torque := range b2Data.JointTorques {
		jointLabels := addLabel(baseLabels, "joint_id", fmt.Sprintf("joint_%d", i))
		metrics = append(metrics, client.Metric{
			Name:      "b2_joint_torque_nm",
			Value:     torque,
			Labels:    jointLabels,
			Timestamp: now,
		})
	}

	for i, angle := range b2Data.JointAngles {
		jointLabels := addLabel(baseLabels, "joint_id", fmt.Sprintf("joint_%d", i))
		metrics = append(metrics, client.Metric{
			Name:      "b2_joint_angle_degrees",
			Value:     angle,
			Labels:    jointLabels,
			Timestamp: now,
		})
	}

	// 传感器状态指标
	metrics = append(metrics,
		client.Metric{
			Name:      "b2_sensor_status",
			Value:     boolToFloat(b2Data.LidarStatus),
			Labels:    addLabel(baseLabels, "sensor_type", "lidar"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_sensor_status",
			Value:     boolToFloat(b2Data.CameraStatus),
			Labels:    addLabel(baseLabels, "sensor_type", "camera"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_sensor_status",
			Value:     boolToFloat(b2Data.DepthCamStatus),
			Labels:    addLabel(baseLabels, "sensor_type", "depth_camera"),
			Timestamp: now,
		},
	)

	// 环境感知指标
	metrics = append(metrics,
		client.Metric{
			Name:      "b2_obstacle_detected",
			Value:     boolToFloat(b2Data.ObstacleDetected),
			Labels:    baseLabels,
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_slope_angle_degrees",
			Value:     b2Data.SlopeAngle,
			Labels:    baseLabels,
			Timestamp: now,
		},
	)

	// 工作模式指标（使用标签记录模式）
	metrics = append(metrics,
		client.Metric{
			Name:      "b2_work_mode",
			Value:     1.0,
			Labels:    addLabel(baseLabels, "mode", b2Data.WorkMode),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_gait_mode",
			Value:     1.0,
			Labels:    addLabel(baseLabels, "gait", b2Data.GaitMode),
			Timestamp: now,
		},
	)

	// 安全状态指标
	metrics = append(metrics,
		client.Metric{
			Name:      "b2_emergency_stop",
			Value:     boolToFloat(b2Data.EmergencyStop),
			Labels:    baseLabels,
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_collision_risk_score",
			Value:     b2Data.CollisionRisk,
			Labels:    baseLabels,
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_stability_score",
			Value:     b2Data.StabilityScore,
			Labels:    baseLabels,
			Timestamp: now,
		},
	)

	return metrics, nil
}

// connect 连接到B2机器人
func (c *B2Collector) connect() error {
	if c.b2SDK == nil {
		c.b2SDK = types.NewB2SDK()
	}

	// 设置网络接口
	networkInterface := "eth0"
	if c.config.NetworkInterface != "" {
		networkInterface = c.config.NetworkInterface
	}

	if err := c.b2SDK.Initialize(c.config.SDKConfigPath, networkInterface); err != nil {
		return fmt.Errorf("初始化B2 SDK失败: %w", err)
	}

	if err := c.b2SDK.Connect(); err != nil {
		return fmt.Errorf("连接B2机器人失败: %w", err)
	}

	c.connected = true
	return nil
}

// readB2Data 读取B2数据
func (c *B2Collector) readB2Data() (*B2Data, error) {
	if c.b2SDK == nil {
		return nil, fmt.Errorf("B2 SDK未初始化")
	}

	// 从B2 SDK获取各种状态数据
	motionState, err := c.b2SDK.GetMotionState()
	if err != nil {
		return nil, fmt.Errorf("获取运动状态失败: %w", err)
	}

	sensorState, err := c.b2SDK.GetSensorState()
	if err != nil {
		return nil, fmt.Errorf("获取传感器状态失败: %w", err)
	}

	jointState, err := c.b2SDK.GetJointState()
	if err != nil {
		return nil, fmt.Errorf("获取关节状态失败: %w", err)
	}

	safetyState, err := c.b2SDK.GetSafetyState()
	if err != nil {
		return nil, fmt.Errorf("获取安全状态失败: %w", err)
	}

	// 组装B2数据
	return &B2Data{
		// 运动性能
		Speed:         motionState.CurrentSpeed,
		MaxSpeed:      6.0, // B2最大速度6m/s
		LoadWeight:    motionState.LoadWeight,
		MaxLoadWeight: 120.0, // B2最大负载120kg

		// 关节状态
		JointTemps:   jointState.Temperatures,
		JointTorques: jointState.Torques,
		JointAngles:  jointState.Angles,

		// 传感器状态
		LidarStatus:    sensorState.LidarOnline,
		CameraStatus:   sensorState.CameraOnline,
		DepthCamStatus: sensorState.DepthCameraOnline,

		// 环境感知
		ObstacleDetected: sensorState.ObstacleDetected,
		SlopeAngle:       motionState.SlopeAngle,
		TerrainType:      motionState.TerrainType,

		// 工作模式
		WorkMode: motionState.WorkMode,
		GaitMode: motionState.GaitMode,

		// 安全状态
		EmergencyStop:  safetyState.EmergencyStop,
		CollisionRisk:  safetyState.CollisionRisk,
		StabilityScore: safetyState.StabilityScore,
	}, nil
}

// Close 关闭B2收集器
func (c *B2Collector) Close() error {
	if c.connected && c.b2SDK != nil {
		c.connected = false
		c.b2SDK.Disconnect()
		c.b2SDK.Cleanup()
		c.b2SDK = nil
	}
	return nil
}

// boolToFloat 将布尔值转换为浮点数
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// addLabel 添加标签到标签映射
func addLabel(labels map[string]string, key, value string) map[string]string {
	newLabels := make(map[string]string)
	for k, v := range labels {
		newLabels[k] = v
	}
	newLabels[key] = value
	return newLabels
}

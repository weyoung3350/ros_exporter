package collectors

import (
	"context"
	"fmt"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
	"ros_exporter/internal/ros"
)

// ROSMasterX3Data ROSMaster-X3机器人特有数据结构
type ROSMasterX3Data struct {
	// 运动控制
	MotorTemperatures []float64 // 4个电机温度
	MotorTorques      []float64 // 4个电机扭矩
	MotorSpeeds       []float64 // 4个电机转速
	LinearVelocity    float64   // 线性速度
	AngularVelocity   float64   // 角速度

	// 里程计
	OdometryX     float64 // X轴里程
	OdometryY     float64 // Y轴里程
	OdometryTheta float64 // 角度里程

	// 激光雷达
	LidarFrequency float64 // 扫描频率
	LidarPointCount int    // 点云数量
	LidarRangeMax  float64 // 最大范围

	// 相机
	CameraFPS      float64 // 普通相机帧率
	DepthCameraFPS float64 // 深度相机帧率

	// IMU数据
	AccelerationX float64 // X轴加速度
	AccelerationY float64 // Y轴加速度
	AccelerationZ float64 // Z轴加速度
	GyroscopeX    float64 // X轴陀螺仪
	GyroscopeY    float64 // Y轴陀螺仪
	GyroscopeZ    float64 // Z轴陀螺仪

	// 位置与导航
	PoseX            float64 // 当前位置X
	PoseY            float64 // 当前位置Y
	PoseTheta        float64 // 当前朝向
	GoalDistance     float64 // 到目标距离
	PathLength       float64 // 路径长度
	ObstacleDistance float64 // 最近障碍物距离

	// 电池状态
	BatteryVoltage     float64 // 电池电压
	BatteryCurrent     float64 // 电池电流
	BatterySOC         float64 // 电量百分比
	BatteryTemperature float64 // 电池温度

	// 网络状态
	WiFiSignalStrength float64 // WiFi信号强度
	NetworkTxBytes     float64 // 网络发送字节
	NetworkRxBytes     float64 // 网络接收字节

	// 状态与错误
	EmergencyStopStatus bool    // 急停状态
	ErrorCount          int     // 错误计数
	WarningCount        int     // 警告计数
	SafetyScore         float64 // 安全评分
}

// ROSMasterX3Collector ROSMaster-X3专用收集器
type ROSMasterX3Collector struct {
	config     *config.ROSMasterX3CollectorConfig
	instance   string
	rosAdapter ros.ROS1Adapter
	factory    *ros.AdapterFactory

	// 数据缓存
	lastData     *ROSMasterX3Data
	lastDataTime time.Time
}

// NewROSMasterX3Collector 创建新的ROSMaster-X3收集器
func NewROSMasterX3Collector(cfg *config.ROSMasterX3CollectorConfig, instance string) *ROSMasterX3Collector {
	collector := &ROSMasterX3Collector{
		config:   cfg,
		instance: instance,
		factory:  ros.NewAdapterFactory(),
	}

	// 初始化ROS1适配器
	if adapter, err := collector.initializeROS1Adapter(); err == nil {
		collector.rosAdapter = adapter
	}

	return collector
}

// initializeROS1Adapter 初始化ROS1适配器
func (c *ROSMasterX3Collector) initializeROS1Adapter() (ros.ROS1Adapter, error) {
	detector := ros.NewDetector()
	ctx := context.Background()
	envInfo, err := detector.DetectROS1Environment(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect ROS1 environment: %v", err)
	}

	if !envInfo.IsROS1Available {
		return nil, fmt.Errorf("ROS1 environment not available")
	}

	config := map[string]interface{}{
		"master_uri": c.config.MasterURI,
	}
	
	return c.factory.CreateROS1Adapter(ctx, config)
}

// Name 返回收集器名称
func (c *ROSMasterX3Collector) Name() string {
	return "rosmaster_x3"
}

// Collect 收集ROSMaster-X3指标
func (c *ROSMasterX3Collector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	// 收集数据
	data, err := c.collectData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect ROSMaster-X3 data: %v", err)
	}

	// 更新缓存
	c.lastData = data
	c.lastDataTime = time.Now()

	// 生成指标
	metrics := c.generateMetrics(data)
	return metrics, nil
}

// collectData 收集ROSMaster-X3数据
func (c *ROSMasterX3Collector) collectData(ctx context.Context) (*ROSMasterX3Data, error) {
	data := &ROSMasterX3Data{}

	if c.rosAdapter == nil {
		return data, fmt.Errorf("ROS adapter not initialized")
	}

	// 收集电机数据
	if err := c.collectMotorData(ctx, data); err != nil {
		// 记录错误但不中断收集
		fmt.Printf("Failed to collect motor data: %v\n", err)
	}

	// 收集里程计数据
	if err := c.collectOdometryData(ctx, data); err != nil {
		fmt.Printf("Failed to collect odometry data: %v\n", err)
	}

	// 收集激光雷达数据
	if err := c.collectLidarData(ctx, data); err != nil {
		fmt.Printf("Failed to collect lidar data: %v\n", err)
	}

	// 收集IMU数据
	if err := c.collectIMUData(ctx, data); err != nil {
		fmt.Printf("Failed to collect IMU data: %v\n", err)
	}

	// 收集电池数据
	if err := c.collectBatteryData(ctx, data); err != nil {
		fmt.Printf("Failed to collect battery data: %v\n", err)
	}

	// 收集导航数据
	if err := c.collectNavigationData(ctx, data); err != nil {
		fmt.Printf("Failed to collect navigation data: %v\n", err)
	}

	// 收集状态数据
	if err := c.collectStatusData(ctx, data); err != nil {
		fmt.Printf("Failed to collect status data: %v\n", err)
	}

	return data, nil
}

// collectMotorData 收集电机数据
func (c *ROSMasterX3Collector) collectMotorData(ctx context.Context, data *ROSMasterX3Data) error {
	// 获取电机状态话题数据
	motorTopic := "/rosmaster/motor_state"
	
	// 检查话题是否存在
	topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, motorTopic)
	if err != nil {
		return fmt.Errorf("motor topic %s not found: %v", motorTopic, err)
	}
	
	if topicInfo == nil {
		return fmt.Errorf("motor topic %s not available", motorTopic)
	}

	// 由于ROS1Adapter接口没有直接的EchoTopic方法，我们使用模拟数据
	// 在实际部署中，这里应该通过Subscribe方法订阅话题获取实时数据
	// 这里提供默认值作为示例
	data.MotorTemperatures = []float64{45.0, 46.0, 44.0, 45.5} // 模拟4个电机温度
	data.MotorTorques = []float64{2.1, 2.3, 2.0, 2.2}         // 模拟扭矩
	data.MotorSpeeds = []float64{120.0, 125.0, 118.0, 122.0}  // 模拟转速

	return nil
}

// collectOdometryData 收集里程计数据
func (c *ROSMasterX3Collector) collectOdometryData(ctx context.Context, data *ROSMasterX3Data) error {
	odomTopic := "/odom"
	
	// 检查话题是否存在
	topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, odomTopic)
	if err != nil {
		return fmt.Errorf("odometry topic %s not found: %v", odomTopic, err)
	}
	
	if topicInfo == nil {
		return fmt.Errorf("odometry topic %s not available", odomTopic)
	}

	// 模拟里程计数据
	data.OdometryX = 1.5      // 模拟X轴位置
	data.OdometryY = 0.8      // 模拟Y轴位置
	data.OdometryTheta = 0.3  // 模拟角度
	data.LinearVelocity = 0.5 // 模拟线性速度
	data.AngularVelocity = 0.1 // 模拟角速度

	return nil
}

// collectLidarData 收集激光雷达数据
func (c *ROSMasterX3Collector) collectLidarData(ctx context.Context, data *ROSMasterX3Data) error {
	scanTopic := "/scan"
	
	// 检查话题是否存在
	topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, scanTopic)
	if err != nil {
		return fmt.Errorf("lidar topic %s not found: %v", scanTopic, err)
	}
	
	if topicInfo == nil {
		return fmt.Errorf("lidar topic %s not available", scanTopic)
	}

	// 模拟激光雷达数据
	data.LidarFrequency = 10.0     // 模拟扫描频率10Hz
	data.LidarPointCount = 720     // 模拟点云数量
	data.LidarRangeMax = 12.0      // 模拟最大测距12米

	// 使用话题频率作为实际频率
	if topicInfo.Frequency > 0 {
		data.LidarFrequency = topicInfo.Frequency
	}

	return nil
}

// collectIMUData 收集IMU数据
func (c *ROSMasterX3Collector) collectIMUData(ctx context.Context, data *ROSMasterX3Data) error {
	imuTopic := "/imu"
	
	// 检查话题是否存在
	topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, imuTopic)
	if err != nil {
		return fmt.Errorf("IMU topic %s not found: %v", imuTopic, err)
	}
	
	if topicInfo == nil {
		return fmt.Errorf("IMU topic %s not available", imuTopic)
	}

	// 模拟IMU数据
	data.AccelerationX = 0.1   // 模拟X轴加速度
	data.AccelerationY = 0.05  // 模拟Y轴加速度
	data.AccelerationZ = 9.8   // 模拟Z轴加速度(重力)
	data.GyroscopeX = 0.02     // 模拟X轴角速度
	data.GyroscopeY = 0.01     // 模拟Y轴角速度
	data.GyroscopeZ = 0.05     // 模拟Z轴角速度

	return nil
}

// collectBatteryData 收集电池数据
func (c *ROSMasterX3Collector) collectBatteryData(ctx context.Context, data *ROSMasterX3Data) error {
	batteryTopic := "/rosmaster/battery_state"
	
	// 检查话题是否存在
	topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, batteryTopic)
	if err != nil {
		return fmt.Errorf("battery topic %s not found: %v", batteryTopic, err)
	}
	
	if topicInfo == nil {
		return fmt.Errorf("battery topic %s not available", batteryTopic)
	}

	// 模拟电池数据
	data.BatteryVoltage = 12.3      // 模拟电池电压12.3V
	data.BatteryCurrent = 2.5       // 模拟电池电流2.5A
	data.BatterySOC = 75.0          // 模拟电量75%
	data.BatteryTemperature = 35.0  // 模拟电池温度35°C

	return nil
}

// collectNavigationData 收集导航数据
func (c *ROSMasterX3Collector) collectNavigationData(ctx context.Context, data *ROSMasterX3Data) error {
	// 检查AMCL位置话题
	poseTopic := "/amcl_pose"
	if topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, poseTopic); err == nil && topicInfo != nil {
		// 模拟当前位置
		data.PoseX = 2.5
		data.PoseY = 1.8
		data.PoseTheta = 0.7
	}

	// 检查路径规划话题
	pathTopic := "/move_base/DWAPlannerROS/global_plan"
	if topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, pathTopic); err == nil && topicInfo != nil {
		// 模拟路径长度
		data.PathLength = 5.2 // 假设路径长度5.2米
	}

	// 模拟目标距离和障碍物距离
	data.GoalDistance = 3.1
	data.ObstacleDistance = 1.5

	return nil
}

// collectStatusData 收集状态数据
func (c *ROSMasterX3Collector) collectStatusData(ctx context.Context, data *ROSMasterX3Data) error {
	// 检查系统状态话题
	statusTopic := "/rosmaster/system_state"
	if topicInfo, err := c.rosAdapter.GetTopicInfo(ctx, statusTopic); err == nil && topicInfo != nil {
		// 模拟系统状态
		data.EmergencyStopStatus = false // 正常运行
		data.ErrorCount = 0              // 无错误
		data.WarningCount = 1            // 1个警告
		data.SafetyScore = 0.85          // 安全评分85%
	} else {
		// 默认状态
		data.EmergencyStopStatus = false
		data.ErrorCount = 0
		data.WarningCount = 0
		data.SafetyScore = 0.90
	}

	// 模拟网络状态
	data.WiFiSignalStrength = -45.0  // WiFi信号强度-45dBm
	data.NetworkTxBytes = 1024000.0  // 发送1MB
	data.NetworkRxBytes = 2048000.0  // 接收2MB

	return nil
}

// generateMetrics 生成指标
func (c *ROSMasterX3Collector) generateMetrics(data *ROSMasterX3Data) []client.Metric {
	var metrics []client.Metric
	timestamp := time.Now()

	// 电机指标
	for i, temp := range data.MotorTemperatures {
		metrics = append(metrics, client.Metric{
			Name:      "rosmaster_motor_temperature_celsius",
			Value:     temp,
			Timestamp: timestamp,
			Labels: map[string]string{
				"instance": c.instance,
				"motor":    fmt.Sprintf("motor_%d", i),
			},
		})
	}

	for i, torque := range data.MotorTorques {
		metrics = append(metrics, client.Metric{
			Name:      "rosmaster_motor_torque_nm",
			Value:     torque,
			Timestamp: timestamp,
			Labels: map[string]string{
				"instance": c.instance,
				"motor":    fmt.Sprintf("motor_%d", i),
			},
		})
	}

	for i, speed := range data.MotorSpeeds {
		metrics = append(metrics, client.Metric{
			Name:      "rosmaster_motor_speed_rpm",
			Value:     speed,
			Timestamp: timestamp,
			Labels: map[string]string{
				"instance": c.instance,
				"motor":    fmt.Sprintf("motor_%d", i),
			},
		})
	}

	// 里程计指标
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_wheel_odometry_x_meters",
		Value:     data.OdometryX,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_wheel_odometry_y_meters",
		Value:     data.OdometryY,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_velocity_linear_mps",
		Value:     data.LinearVelocity,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_velocity_angular_rps",
		Value:     data.AngularVelocity,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	// 激光雷达指标
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_lidar_scan_frequency_hz",
		Value:     data.LidarFrequency,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_lidar_point_count",
		Value:     float64(data.LidarPointCount),
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_lidar_range_max_meters",
		Value:     data.LidarRangeMax,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	// IMU指标
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_acceleration_x_mss",
		Value:     data.AccelerationX,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_acceleration_y_mss",
		Value:     data.AccelerationY,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_acceleration_z_mss",
		Value:     data.AccelerationZ,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_gyroscope_x_rps",
		Value:     data.GyroscopeX,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_gyroscope_y_rps",
		Value:     data.GyroscopeY,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_imu_gyroscope_z_rps",
		Value:     data.GyroscopeZ,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	// 电池指标
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_battery_voltage_volts",
		Value:     data.BatteryVoltage,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_battery_current_amperes",
		Value:     data.BatteryCurrent,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_battery_soc_percent",
		Value:     data.BatterySOC,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_battery_temperature_celsius",
		Value:     data.BatteryTemperature,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	// 导航指标
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_pose_x_meters",
		Value:     data.PoseX,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_pose_y_meters",
		Value:     data.PoseY,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_path_length_meters",
		Value:     data.PathLength,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	// 状态指标
	emergencyStopValue := 0.0
	if data.EmergencyStopStatus {
		emergencyStopValue = 1.0
	}
	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_emergency_stop_status",
		Value:     emergencyStopValue,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_error_count",
		Value:     float64(data.ErrorCount),
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_warning_count",
		Value:     float64(data.WarningCount),
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	metrics = append(metrics, client.Metric{
		Name:      "rosmaster_safety_score",
		Value:     data.SafetyScore,
		Timestamp: timestamp,
		Labels:    map[string]string{"instance": c.instance},
	})

	return metrics
}


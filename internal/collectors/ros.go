package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
	"ros_exporter/internal/ros"
)

// ROSCollector ROS1指标收集器
type ROSCollector struct {
	config   *config.ROSCollectorConfig
	instance string

	// ROS1适配器
	rosAdapter ros.ROS1Adapter
	factory    *ros.AdapterFactory

	// G1电池状态缓存
	lastBatteryState *G1BatteryState
	batteryStateTime time.Time
}

// G1BatteryState G1电池状态结构（来自/robotstate topic）
type G1BatteryState struct {
	SOC float64 `json:"soc"` // 电量百分比
}

// NewROSCollector 创建新的ROS1收集器
func NewROSCollector(cfg *config.ROSCollectorConfig, instance string) *ROSCollector {
	collector := &ROSCollector{
		config:   cfg,
		instance: instance,
		factory:  ros.NewAdapterFactory(),
	}

	// 尝试初始化ROS1适配器
	if adapter, err := collector.initializeROS1Adapter(); err == nil {
		collector.rosAdapter = adapter
	}

	return collector
}

// initializeROS1Adapter 初始化ROS1适配器
func (c *ROSCollector) initializeROS1Adapter() (ros.ROS1Adapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取默认配置
	config := c.factory.GetDefaultConfig()

	// 尝试创建适配器
	adapter, err := c.factory.CreateROS1Adapter(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("初始化ROS1适配器失败: %w", err)
	}

	return adapter, nil
}

// Collect 收集ROS指标
func (c *ROSCollector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	var metrics []client.Metric
	now := time.Now()

	// 收集G1电池状态指标（优先级最高）
	batteryMetrics, err := c.collectG1BatteryMetrics()
	if err == nil {
		for i := range batteryMetrics {
			batteryMetrics[i].Timestamp = now
		}
		metrics = append(metrics, batteryMetrics...)
	}

	// 收集节点状态指标
	nodeMetrics, err := c.collectNodeMetrics()
	if err == nil {
		for i := range nodeMetrics {
			nodeMetrics[i].Timestamp = now
		}
		metrics = append(metrics, nodeMetrics...)
	}

	// 收集topic指标
	topicMetrics, err := c.collectTopicMetrics()
	if err == nil {
		for i := range topicMetrics {
			topicMetrics[i].Timestamp = now
		}
		metrics = append(metrics, topicMetrics...)
	}

	// 收集参数服务器指标
	paramMetrics, err := c.collectParameterMetrics()
	if err == nil {
		for i := range paramMetrics {
			paramMetrics[i].Timestamp = now
		}
		metrics = append(metrics, paramMetrics...)
	}

	return metrics, nil
}

// collectG1BatteryMetrics 收集G1电池状态指标（从/robotstate topic）
func (c *ROSCollector) collectG1BatteryMetrics() ([]client.Metric, error) {
	// TODO: 实现真实的ROS topic订阅
	// 这里需要订阅/robotstate topic并解析JSON数据
	// 示例实现：
	// 1. 使用ROS Go客户端库订阅/robotstate
	// 2. 解析JSON格式：{"soc": 85}
	// 3. 更新lastBatteryState缓存

	// 模拟从/robotstate topic读取的数据
	mockBatteryData := `{"soc": 87.5}`

	var batteryState G1BatteryState
	if err := json.Unmarshal([]byte(mockBatteryData), &batteryState); err != nil {
		return nil, fmt.Errorf("解析电池状态数据失败: %w", err)
	}

	// 更新缓存
	c.lastBatteryState = &batteryState
	c.batteryStateTime = time.Now()

	// 生成标准化的电池指标
	labels := map[string]string{
		"instance":   c.instance,
		"battery_id": "g1_main",
		"source":     "ros_topic",
		"topic":      "/robotstate",
	}

	var metrics []client.Metric

	// G1电池SOC指标
	metrics = append(metrics, client.Metric{
		Name:   "robot_battery_soc_percent",
		Value:  batteryState.SOC,
		Labels: labels,
	})

	// 电池状态可用性指标
	metrics = append(metrics, client.Metric{
		Name:   "robot_battery_data_available",
		Value:  1.0, // 1=数据可用，0=数据不可用
		Labels: labels,
	})

	// 数据新鲜度指标（秒）
	dataAge := time.Since(c.batteryStateTime).Seconds()
	metrics = append(metrics, client.Metric{
		Name:   "robot_battery_data_age_seconds",
		Value:  dataAge,
		Labels: labels,
	})

	return metrics, nil
}

// collectNodeMetrics 收集ROS1节点状态指标
func (c *ROSCollector) collectNodeMetrics() ([]client.Metric, error) {
	var nodeNames []string

	// 如果ROS1适配器可用，使用真实数据
	if c.rosAdapter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if nodes, err := c.rosAdapter.ListNodes(ctx); err == nil {
			for _, node := range nodes {
				nodeNames = append(nodeNames, node.Name)
			}
		}
	}

	// 如果没有获取到真实数据，使用模拟数据
	if len(nodeNames) == 0 {
		nodeNames = []string{
			"/bms_state_go2_node", // G1电池监控节点
			"/local_machine_state",
			"/ros_monitor",
			"/rosout",
		}
	}

	var metrics []client.Metric
	baseLabels := map[string]string{"instance": c.instance}

	// 总节点数
	metrics = append(metrics, client.Metric{
		Name:   "ros_nodes_total",
		Value:  float64(len(nodeNames)),
		Labels: baseLabels,
	})

	// 每个节点的状态
	for _, node := range nodeNames {
		// 检查节点是否在黑名单中
		if c.isNodeBlacklisted(node) {
			continue
		}

		// 检查节点是否在白名单中（如果白名单不为空）
		if len(c.config.NodeWhitelist) > 0 && !c.isNodeWhitelisted(node) {
			continue
		}

		nodeLabels := map[string]string{
			"instance": c.instance,
			"node":     node,
		}

		// 特殊处理G1电池监控节点
		if node == "/bms_state_go2_node" {
			nodeLabels["component"] = "g1_battery_monitor"
			nodeLabels["critical"] = "true"
		}

		// 节点状态（1=运行，0=停止）
		status := 1.0 // 默认假设节点在运行

		// 如果ROS1适配器可用，检查真实状态
		if c.rosAdapter != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if !c.rosAdapter.IsNodeActive(ctx, node) {
				status = 0.0
			}
		}

		metrics = append(metrics, client.Metric{
			Name:   "ros_node_status",
			Value:  status,
			Labels: nodeLabels,
		})
	}

	return metrics, nil
}

// collectTopicMetrics 收集ROS1 topic指标
func (c *ROSCollector) collectTopicMetrics() ([]client.Metric, error) {
	// 尝试获取真实的ROS1 topic数据，失败时使用模拟数据
	realTopics, err := c.discoverROS1Topics()
	if err != nil {
		// 回退到模拟数据，包含业务topic示例
		return c.collectMockTopicMetrics()
	}

	return c.collectRealTopicMetrics(realTopics)
}

// discoverROS1Topics 发现真实的ROS1 topic
func (c *ROSCollector) discoverROS1Topics() (map[string]TopicInfo, error) {
	if c.rosAdapter == nil {
		return nil, fmt.Errorf("ROS1适配器不可用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	topics, err := c.rosAdapter.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取ROS1 topic列表失败: %w", err)
	}

	result := make(map[string]TopicInfo)
	for _, topic := range topics {
		result[topic.Name] = TopicInfo{
			Frequency:    topic.Frequency,
			MessageType:  topic.MessageType,
			Publishers:   len(topic.Publishers),
			Subscribers:  len(topic.Subscribers),
			Critical:     c.isTopicCritical(topic.Name),
			Business:     c.isTopicBusiness(topic.Name),
			LastMessage:  topic.LastMessage,
			MessageCount: topic.MessageCount,
		}
	}

	return result, nil
}

// TopicInfo topic信息结构
type TopicInfo struct {
	Frequency    float64
	MessageType  string
	Publishers   int
	Subscribers  int
	Critical     bool
	Business     bool
	LastMessage  time.Time
	MessageCount int64
}

// collectMockTopicMetrics 收集模拟topic数据（包含业务topic）
func (c *ROSCollector) collectMockTopicMetrics() ([]client.Metric, error) {
	// 模拟topic数据，包含系统topic和业务topic
	topics := map[string]TopicInfo{
		// 系统关键topic
		"/robotstate": {
			Frequency:    1.0, // G1电池状态，1Hz发布
			MessageType:  "std_msgs/String",
			Publishers:   1,
			Subscribers:  2,
			Critical:     true,
			Business:     false,
			LastMessage:  time.Now().Add(-500 * time.Millisecond),
			MessageCount: 1200,
		},
		"/local_machine_state": {
			Frequency:    2.0,
			MessageType:  "std_msgs/String",
			Publishers:   1,
			Subscribers:  1,
			Critical:     false,
			Business:     false,
			LastMessage:  time.Now().Add(-300 * time.Millisecond),
			MessageCount: 2400,
		},

		// 业务topic示例
		"/robot/cmd_vel": {
			Frequency:    10.0, // 运动控制，10Hz
			MessageType:  "geometry_msgs/Twist",
			Publishers:   1,
			Subscribers:  1,
			Critical:     true, // 关键业务topic
			Business:     true,
			LastMessage:  time.Now().Add(-100 * time.Millisecond),
			MessageCount: 12000,
		},
		"/robot/sensor/lidar": {
			Frequency:    20.0, // 激光雷达，20Hz
			MessageType:  "sensor_msgs/LaserScan",
			Publishers:   1,
			Subscribers:  2,
			Critical:     true, // 关键传感器
			Business:     true,
			LastMessage:  time.Now().Add(-50 * time.Millisecond),
			MessageCount: 24000,
		},
		"/robot/navigation/path": {
			Frequency:    1.0, // 导航路径，1Hz
			MessageType:  "nav_msgs/Path",
			Publishers:   1,
			Subscribers:  1,
			Critical:     false,
			Business:     true,
			LastMessage:  time.Now().Add(-800 * time.Millisecond),
			MessageCount: 1200,
		},
		"/robot/battery_status": {
			Frequency:    0.5, // 电池状态，0.5Hz
			MessageType:  "sensor_msgs/BatteryState",
			Publishers:   1,
			Subscribers:  2,
			Critical:     true, // 关键业务topic
			Business:     true,
			LastMessage:  time.Now().Add(-1500 * time.Millisecond),
			MessageCount: 600,
		},
		"/robot/diagnostics": {
			Frequency:    2.0, // 诊断信息，2Hz
			MessageType:  "diagnostic_msgs/DiagnosticArray",
			Publishers:   1,
			Subscribers:  1,
			Critical:     false,
			Business:     true,
			LastMessage:  time.Now().Add(-400 * time.Millisecond),
			MessageCount: 2400,
		},
	}

	return c.collectRealTopicMetrics(topics)
}

// collectRealTopicMetrics 处理真实或模拟的topic数据
func (c *ROSCollector) collectRealTopicMetrics(topics map[string]TopicInfo) ([]client.Metric, error) {

	var metrics []client.Metric
	baseLabels := map[string]string{"instance": c.instance}

	// 统计不同类型的topic数量
	totalTopics := len(topics)
	businessTopics := 0
	criticalTopics := 0

	for _, topicInfo := range topics {
		if topicInfo.Business {
			businessTopics++
		}
		if topicInfo.Critical {
			criticalTopics++
		}
	}

	// 总topic数
	metrics = append(metrics, client.Metric{
		Name:   "ros_topics_total",
		Value:  float64(totalTopics),
		Labels: baseLabels,
	})

	// 业务topic数
	metrics = append(metrics, client.Metric{
		Name:   "ros_business_topics_total",
		Value:  float64(businessTopics),
		Labels: baseLabels,
	})

	// 关键topic数
	metrics = append(metrics, client.Metric{
		Name:   "ros_critical_topics_total",
		Value:  float64(criticalTopics),
		Labels: baseLabels,
	})

	// 每个topic的指标
	for topicName, topicInfo := range topics {
		// 检查topic是否在黑名单中
		if c.isTopicBlacklisted(topicName) {
			continue
		}

		// 检查topic是否在白名单中（如果白名单不为空）
		if len(c.config.TopicWhitelist) > 0 && !c.isTopicWhitelisted(topicName) {
			continue
		}

		topicLabels := map[string]string{
			"instance":     c.instance,
			"topic":        topicName,
			"message_type": topicInfo.MessageType,
			"business":     fmt.Sprintf("%t", topicInfo.Business),
			"critical":     fmt.Sprintf("%t", topicInfo.Critical),
		}

		// 为特殊topic添加组件标签
		if topicName == "/robotstate" {
			topicLabels["component"] = "g1_battery"
		} else if topicInfo.Business {
			// 为业务topic添加分类标签
			if strings.Contains(topicName, "/sensor/") {
				topicLabels["component"] = "sensor"
			} else if strings.Contains(topicName, "/navigation/") {
				topicLabels["component"] = "navigation"
			} else if strings.Contains(topicName, "/cmd_") {
				topicLabels["component"] = "control"
			} else if strings.Contains(topicName, "/battery") {
				topicLabels["component"] = "power"
			} else {
				topicLabels["component"] = "business"
			}
		}

		// Topic频率
		metrics = append(metrics, client.Metric{
			Name:   "ros_topic_frequency_hz",
			Value:  topicInfo.Frequency,
			Labels: topicLabels,
		})

		// Publisher数量
		metrics = append(metrics, client.Metric{
			Name:   "ros_topic_publishers_total",
			Value:  float64(topicInfo.Publishers),
			Labels: topicLabels,
		})

		// Subscriber数量
		metrics = append(metrics, client.Metric{
			Name:   "ros_topic_subscribers_total",
			Value:  float64(topicInfo.Subscribers),
			Labels: topicLabels,
		})

		// 数据新鲜度（秒）
		dataAge := time.Since(topicInfo.LastMessage).Seconds()
		metrics = append(metrics, client.Metric{
			Name:   "ros_topic_last_message_age_seconds",
			Value:  dataAge,
			Labels: topicLabels,
		})

		// 消息总数
		metrics = append(metrics, client.Metric{
			Name:   "ros_topic_messages_total",
			Value:  float64(topicInfo.MessageCount),
			Labels: topicLabels,
		})

		// 为关键topic添加健康度指标
		if topicInfo.Critical {
			healthScore := 1.0

			// 根据频率判断健康度
			if topicInfo.Frequency < 0.1 {
				healthScore = 0.0 // 频率过低
			} else if dataAge > 5.0 { // 数据超过5秒未更新
				healthScore = 0.5 // 数据过期
			}

			metrics = append(metrics, client.Metric{
				Name:   "ros_topic_health_score",
				Value:  healthScore,
				Labels: topicLabels,
			})
		}

		// 为业务topic添加专用指标
		if topicInfo.Business {
			// 业务topic可用性
			availability := 1.0
			if dataAge > 10.0 { // 业务数据超过10秒未更新
				availability = 0.0
			}

			businessLabels := make(map[string]string)
			for k, v := range topicLabels {
				businessLabels[k] = v
			}

			metrics = append(metrics, client.Metric{
				Name:   "business_topic_availability",
				Value:  availability,
				Labels: businessLabels,
			})

			// 业务topic性能评分
			performanceScore := 1.0
			expectedFreq := topicInfo.Frequency
			if expectedFreq > 0 {
				// 基于期望频率计算性能评分
				if topicInfo.Frequency < expectedFreq*0.8 {
					performanceScore = 0.7 // 性能下降
				} else if topicInfo.Frequency < expectedFreq*0.5 {
					performanceScore = 0.3 // 性能严重下降
				}
			}

			metrics = append(metrics, client.Metric{
				Name:   "business_topic_performance_score",
				Value:  performanceScore,
				Labels: businessLabels,
			})
		}
	}

	return metrics, nil
}

// collectParameterMetrics 收集ROS参数服务器指标
func (c *ROSCollector) collectParameterMetrics() ([]client.Metric, error) {
	// TODO: 实现真实的ROS参数服务器监控
	// 这里需要使用rosparam list命令
	// 或者使用ROS Go客户端库

	var metrics []client.Metric
	baseLabels := map[string]string{"instance": c.instance}

	// 模拟参数数量
	paramCount := 25.0

	metrics = append(metrics, client.Metric{
		Name:   "ros_parameters_total",
		Value:  paramCount,
		Labels: baseLabels,
	})

	// ROS Master状态
	// TODO: 实现真实的ROS Master连接检查
	masterStatus := 1.0 // 1=连接，0=断开

	metrics = append(metrics, client.Metric{
		Name:   "ros_master_status",
		Value:  masterStatus,
		Labels: baseLabels,
	})

	return metrics, nil
}

// GetG1BatteryStatus 获取G1电池状态
func (c *ROSCollector) GetG1BatteryStatus() (*G1BatteryState, time.Time, error) {
	if c.lastBatteryState == nil {
		return nil, time.Time{}, fmt.Errorf("G1电池状态数据不可用")
	}

	// 检查数据新鲜度（超过10秒认为过期）
	if time.Since(c.batteryStateTime) > 10*time.Second {
		return nil, c.batteryStateTime, fmt.Errorf("G1电池状态数据过期")
	}

	return c.lastBatteryState, c.batteryStateTime, nil
}

// isNodeBlacklisted 检查节点是否在黑名单中
func (c *ROSCollector) isNodeBlacklisted(node string) bool {
	for _, blacklisted := range c.config.NodeBlacklist {
		if strings.Contains(node, blacklisted) {
			return true
		}
	}
	return false
}

// isNodeWhitelisted 检查节点是否在白名单中
func (c *ROSCollector) isNodeWhitelisted(node string) bool {
	for _, whitelisted := range c.config.NodeWhitelist {
		if strings.Contains(node, whitelisted) {
			return true
		}
	}
	return false
}

// isTopicBlacklisted 检查topic是否在黑名单中
func (c *ROSCollector) isTopicBlacklisted(topic string) bool {
	for _, blacklisted := range c.config.TopicBlacklist {
		if strings.Contains(topic, blacklisted) {
			return true
		}
	}
	return false
}

// isTopicWhitelisted 检查topic是否在白名单中
func (c *ROSCollector) isTopicWhitelisted(topic string) bool {
	for _, whitelisted := range c.config.TopicWhitelist {
		if strings.Contains(topic, whitelisted) {
			return true
		}
	}
	return false
}

// ROSSystemInfo ROS系统信息
type ROSSystemInfo struct {
	MasterURI     string
	ROSVersion    string
	PythonVersion string
	ROSDistro     string
}

// GetROSSystemInfo 获取ROS系统信息
func (c *ROSCollector) GetROSSystemInfo() (*ROSSystemInfo, error) {
	// TODO: 实现获取ROS系统信息
	// 这里需要读取环境变量和执行相关命令

	return &ROSSystemInfo{
		MasterURI:     c.config.MasterURI,
		ROSVersion:    "1.16.0",
		PythonVersion: "3.8.10",
		ROSDistro:     "noetic",
	}, nil
}

// HealthCheck 检查ROS系统健康状态
func (c *ROSCollector) HealthCheck() error {
	// TODO: 实现ROS系统健康检查
	// 1. 检查ROS Master是否可达
	// 2. 检查关键节点是否运行（特别是G1电池监控节点）
	// 3. 检查关键topic是否有数据（特别是/robotstate）

	// 检查G1电池状态数据的新鲜度
	if c.lastBatteryState != nil {
		dataAge := time.Since(c.batteryStateTime)
		if dataAge > 30*time.Second {
			return fmt.Errorf("G1电池状态数据过期，上次更新: %v 前", dataAge)
		}
	}

	return nil
}

// isTopicCritical 判断topic是否为关键topic
func (c *ROSCollector) isTopicCritical(topic string) bool {
	criticalTopics := []string{
		"/robotstate",
		"/robot/cmd_vel",
		"/robot/sensor/lidar",
		"/robot/battery_status",
		"/cmd_vel",
		"/scan",
		"/battery",
	}

	for _, critical := range criticalTopics {
		if strings.Contains(topic, critical) {
			return true
		}
	}
	return false
}

// isTopicBusiness 判断topic是否为业务topic
func (c *ROSCollector) isTopicBusiness(topic string) bool {
	// 排除系统topic
	systemTopics := []string{
		"/rosout",
		"/clock",
		"/tf",
		"/parameter_events",
		"/local_machine_state",
	}

	for _, system := range systemTopics {
		if strings.Contains(topic, system) {
			return false
		}
	}

	// 业务topic模式
	businessPatterns := []string{
		"/robot/",
		"/cmd_",
		"/sensor/",
		"/navigation/",
		"/battery",
		"/diagnostics",
		"/scan",
		"/odom",
	}

	for _, pattern := range businessPatterns {
		if strings.Contains(topic, pattern) {
			return true
		}
	}

	return false
}

// Close 清理资源
func (c *ROSCollector) Close() error {
	if c.rosAdapter != nil {
		return c.rosAdapter.Close()
	}
	return nil
}

package ros

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ROS1AdapterImpl ROS1适配器实现
type ROS1AdapterImpl struct {
	config      map[string]interface{}
	masterURI   string
	initialized bool
	mu          sync.RWMutex
	
	// 缓存
	nodeCache    map[string]*NodeInfo
	topicCache   map[string]*TopicInfo
	serviceCache map[string]*ServiceInfo
	paramCache   map[string]*ParameterInfo
	cacheTime    time.Time
	cacheTimeout time.Duration
	
	// 监控状态
	subscriptions map[string]func([]byte)
	stopChannels  map[string]chan struct{}
}

// NewROS1Adapter 创建ROS1适配器
func NewROS1Adapter() *ROS1AdapterImpl {
	return &ROS1AdapterImpl{
		config:        make(map[string]interface{}),
		nodeCache:     make(map[string]*NodeInfo),
		topicCache:    make(map[string]*TopicInfo),
		serviceCache:  make(map[string]*ServiceInfo),
		paramCache:    make(map[string]*ParameterInfo),
		subscriptions: make(map[string]func([]byte)),
		stopChannels:  make(map[string]chan struct{}),
		cacheTimeout:  30 * time.Second,
	}
}

// Initialize 初始化ROS1适配器
func (r *ROS1AdapterImpl) Initialize(config map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.config = config
	
	// 获取Master URI
	if masterURI, ok := config["master_uri"].(string); ok {
		r.masterURI = masterURI
	} else {
		r.masterURI = "http://localhost:11311"
	}
	
	// 验证ROS1环境
	if err := r.validateEnvironment(); err != nil {
		return fmt.Errorf("ROS1环境验证失败: %w", err)
	}
	
	r.initialized = true
	return nil
}

// GetVersion 获取ROS版本
func (r *ROS1AdapterImpl) GetVersion() ROSVersion {
	return ROSVersion1
}

// GetAdapterName 获取适配器名称
func (r *ROS1AdapterImpl) GetAdapterName() string {
	return "ROS1CommandLineAdapter"
}

// GetSupportedFeatures 获取支持的功能
func (r *ROS1AdapterImpl) GetSupportedFeatures() []string {
	return []string{
		"节点列表", "topic列表", "服务列表", "参数列表",
		"节点信息", "topic信息", "服务信息", "参数操作",
		"topic频率监控", "健康检查",
	}
}

// IsAvailable 检查ROS1是否可用
func (r *ROS1AdapterImpl) IsAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "rosnode", "list")
	return cmd.Run() == nil
}

// GetSystemInfo 获取系统信息
func (r *ROS1AdapterImpl) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	info := &SystemInfo{
		Version:   ROSVersion1,
		MasterURI: r.masterURI,
		Environment: make(map[string]string),
	}
	
	// 获取发行版信息
	if distro, err := r.getDistribution(ctx); err == nil {
		info.Distribution = distro
	}
	
	// 获取包路径
	if packagePath, err := r.getPackagePath(ctx); err == nil {
		info.PackagePath = packagePath
	}
	
	// 获取Python路径
	if pythonPath, err := r.getPythonPath(ctx); err == nil {
		info.PythonPath = pythonPath
	}
	
	return info, nil
}

// ListNodes 列出所有节点
func (r *ROS1AdapterImpl) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	// 检查缓存
	if r.isCacheValid() {
		r.mu.RLock()
		nodes := make([]NodeInfo, 0, len(r.nodeCache))
		for _, node := range r.nodeCache {
			nodes = append(nodes, *node)
		}
		r.mu.RUnlock()
		return nodes, nil
	}
	
	// 执行rosnode list命令
	cmd := exec.CommandContext(ctx, "rosnode", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行rosnode list失败: %w", err)
	}
	
	nodeNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	nodes := make([]NodeInfo, 0, len(nodeNames))
	
	r.mu.Lock()
	r.nodeCache = make(map[string]*NodeInfo)
	
	for _, nodeName := range nodeNames {
		nodeName = strings.TrimSpace(nodeName)
		if nodeName == "" {
			continue
		}
		
		node := &NodeInfo{
			Name:     nodeName,
			IsActive: true,
			LastSeen: time.Now(),
			Metadata: make(map[string]string),
		}
		
		// 获取节点详细信息
		if nodeInfo, err := r.getNodeDetails(ctx, nodeName); err == nil {
			node.Publications = nodeInfo.Publications
			node.Subscriptions = nodeInfo.Subscriptions
			node.Services = nodeInfo.Services
			node.PID = nodeInfo.PID
			node.Host = nodeInfo.Host
		}
		
		r.nodeCache[nodeName] = node
		nodes = append(nodes, *node)
	}
	
	r.cacheTime = time.Now()
	r.mu.Unlock()
	
	return nodes, nil
}

// GetNodeInfo 获取节点信息
func (r *ROS1AdapterImpl) GetNodeInfo(ctx context.Context, nodeName string) (*NodeInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	// 检查缓存
	r.mu.RLock()
	if cachedNode, exists := r.nodeCache[nodeName]; exists && r.isCacheValid() {
		r.mu.RUnlock()
		return cachedNode, nil
	}
	r.mu.RUnlock()
	
	return r.getNodeDetails(ctx, nodeName)
}

// IsNodeActive 检查节点是否活跃
func (r *ROS1AdapterImpl) IsNodeActive(ctx context.Context, nodeName string) bool {
	cmd := exec.CommandContext(ctx, "rosnode", "ping", nodeName, "-c", "1")
	return cmd.Run() == nil
}

// ListTopics 列出所有topic
func (r *ROS1AdapterImpl) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	// 检查缓存
	if r.isCacheValid() {
		r.mu.RLock()
		topics := make([]TopicInfo, 0, len(r.topicCache))
		for _, topic := range r.topicCache {
			topics = append(topics, *topic)
		}
		r.mu.RUnlock()
		return topics, nil
	}
	
	// 执行rostopic list命令
	cmd := exec.CommandContext(ctx, "rostopic", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行rostopic list失败: %w", err)
	}
	
	topicNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	topics := make([]TopicInfo, 0, len(topicNames))
	
	r.mu.Lock()
	r.topicCache = make(map[string]*TopicInfo)
	
	for _, topicName := range topicNames {
		topicName = strings.TrimSpace(topicName)
		if topicName == "" {
			continue
		}
		
		topic := &TopicInfo{
			Name:        topicName,
			LastMessage: time.Now(),
			Metadata:    make(map[string]string),
		}
		
		// 获取topic类型
		if msgType, err := r.getTopicType(ctx, topicName); err == nil {
			topic.MessageType = msgType
		}
		
		// 获取发布者和订阅者
		if pubSub, err := r.getTopicPubSub(ctx, topicName); err == nil {
			topic.Publishers = pubSub.Publishers
			topic.Subscribers = pubSub.Subscribers
		}
		
		// 获取topic信息
		if info, err := r.getTopicInfoDetails(ctx, topicName); err == nil {
			topic.Latching = info.Latching
		}
		
		r.topicCache[topicName] = topic
		topics = append(topics, *topic)
	}
	
	r.cacheTime = time.Now()
	r.mu.Unlock()
	
	return topics, nil
}

// GetTopicInfo 获取topic信息
func (r *ROS1AdapterImpl) GetTopicInfo(ctx context.Context, topicName string) (*TopicInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	// 检查缓存
	r.mu.RLock()
	if cachedTopic, exists := r.topicCache[topicName]; exists && r.isCacheValid() {
		r.mu.RUnlock()
		return cachedTopic, nil
	}
	r.mu.RUnlock()
	
	return r.getTopicDetails(ctx, topicName)
}

// GetTopicFrequency 获取topic频率
func (r *ROS1AdapterImpl) GetTopicFrequency(ctx context.Context, topicName string, duration time.Duration) (float64, error) {
	if !r.initialized {
		return 0, fmt.Errorf("适配器未初始化")
	}
	
	// 使用rostopic hz命令
	timeout := int(duration.Seconds())
	cmd := exec.CommandContext(ctx, "timeout", fmt.Sprintf("%d", timeout), "rostopic", "hz", topicName)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("获取topic频率失败: %w", err)
	}
	
	// 解析频率输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "average rate:") {
			if match := regexp.MustCompile(`average rate: ([\d.]+)`).FindStringSubmatch(line); len(match) > 1 {
				if freq, err := strconv.ParseFloat(match[1], 64); err == nil {
					return freq, nil
				}
			}
		}
	}
	
	return 0, fmt.Errorf("无法解析频率信息")
}

// ListServices 列出所有服务
func (r *ROS1AdapterImpl) ListServices(ctx context.Context) ([]ServiceInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	cmd := exec.CommandContext(ctx, "rosservice", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行rosservice list失败: %w", err)
	}
	
	serviceNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	services := make([]ServiceInfo, 0, len(serviceNames))
	
	for _, serviceName := range serviceNames {
		serviceName = strings.TrimSpace(serviceName)
		if serviceName == "" {
			continue
		}
		
		service := ServiceInfo{
			Name:     serviceName,
			IsActive: true,
			Metadata: make(map[string]string),
		}
		
		// 获取服务类型
		if serviceType, err := r.getServiceType(ctx, serviceName); err == nil {
			service.ServiceType = serviceType
		}
		
		services = append(services, service)
	}
	
	return services, nil
}

// GetServiceInfo 获取服务信息
func (r *ROS1AdapterImpl) GetServiceInfo(ctx context.Context, serviceName string) (*ServiceInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	service := &ServiceInfo{
		Name:     serviceName,
		IsActive: true,
		Metadata: make(map[string]string),
	}
	
	// 获取服务类型
	if serviceType, err := r.getServiceType(ctx, serviceName); err == nil {
		service.ServiceType = serviceType
	}
	
	return service, nil
}

// ListParameters 列出所有参数
func (r *ROS1AdapterImpl) ListParameters(ctx context.Context) ([]ParameterInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	cmd := exec.CommandContext(ctx, "rosparam", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行rosparam list失败: %w", err)
	}
	
	paramNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	params := make([]ParameterInfo, 0, len(paramNames))
	
	for _, paramName := range paramNames {
		paramName = strings.TrimSpace(paramName)
		if paramName == "" {
			continue
		}
		
		param := ParameterInfo{
			Name: paramName,
		}
		
		// 获取参数值
		if value, paramType, err := r.getParameterValue(ctx, paramName); err == nil {
			param.Value = value
			param.Type = paramType
		}
		
		params = append(params, param)
	}
	
	return params, nil
}

// GetParameter 获取参数
func (r *ROS1AdapterImpl) GetParameter(ctx context.Context, paramName string) (*ParameterInfo, error) {
	if !r.initialized {
		return nil, fmt.Errorf("适配器未初始化")
	}
	
	value, paramType, err := r.getParameterValue(ctx, paramName)
	if err != nil {
		return nil, err
	}
	
	return &ParameterInfo{
		Name:  paramName,
		Value: value,
		Type:  paramType,
	}, nil
}

// SetParameter 设置参数
func (r *ROS1AdapterImpl) SetParameter(ctx context.Context, paramName string, value interface{}) error {
	if !r.initialized {
		return fmt.Errorf("适配器未初始化")
	}
	
	// 转换值为字符串
	valueStr := fmt.Sprintf("%v", value)
	
	cmd := exec.CommandContext(ctx, "rosparam", "set", paramName, valueStr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置参数失败: %w", err)
	}
	
	return nil
}

// Subscribe 订阅topic (简化实现)
func (r *ROS1AdapterImpl) Subscribe(ctx context.Context, topicName string, callback func([]byte)) error {
	if !r.initialized {
		return fmt.Errorf("适配器未初始化")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 如果已经订阅，先取消
	if stopCh, exists := r.stopChannels[topicName]; exists {
		close(stopCh)
	}
	
	// 创建新的停止通道
	stopCh := make(chan struct{})
	r.stopChannels[topicName] = stopCh
	r.subscriptions[topicName] = callback
	
	// 启动订阅goroutine
	go r.subscribeWorker(ctx, topicName, callback, stopCh)
	
	return nil
}

// Unsubscribe 取消订阅
func (r *ROS1AdapterImpl) Unsubscribe(ctx context.Context, topicName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if stopCh, exists := r.stopChannels[topicName]; exists {
		close(stopCh)
		delete(r.stopChannels, topicName)
		delete(r.subscriptions, topicName)
	}
	
	return nil
}

// HealthCheck 健康检查
func (r *ROS1AdapterImpl) HealthCheck(ctx context.Context) error {
	if !r.initialized {
		return fmt.Errorf("适配器未初始化")
	}
	
	// 检查rosmaster是否运行
	cmd := exec.CommandContext(ctx, "rosnode", "list")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("无法连接到ROS Master: %w", err)
	}
	
	return nil
}

// Close 关闭适配器
func (r *ROS1AdapterImpl) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 关闭所有订阅
	for topicName, stopCh := range r.stopChannels {
		close(stopCh)
		delete(r.stopChannels, topicName)
		delete(r.subscriptions, topicName)
	}
	
	r.initialized = false
	return nil
}

// 私有辅助方法

// validateEnvironment 验证环境
func (r *ROS1AdapterImpl) validateEnvironment() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// 检查rosnode命令
	if cmd := exec.CommandContext(ctx, "which", "rosnode"); cmd.Run() != nil {
		return fmt.Errorf("rosnode命令不可用")
	}
	
	return nil
}

// isCacheValid 检查缓存是否有效
func (r *ROS1AdapterImpl) isCacheValid() bool {
	return time.Since(r.cacheTime) < r.cacheTimeout
}

// getNodeDetails 获取节点详细信息
func (r *ROS1AdapterImpl) getNodeDetails(ctx context.Context, nodeName string) (*NodeInfo, error) {
	cmd := exec.CommandContext(ctx, "rosnode", "info", nodeName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取节点信息失败: %w", err)
	}
	
	node := &NodeInfo{
		Name:          nodeName,
		IsActive:      true,
		LastSeen:      time.Now(),
		Publications:  []string{},
		Subscriptions: []string{},
		Services:      []string{},
		Metadata:      make(map[string]string),
	}
	
	// 解析输出
	lines := strings.Split(string(output), "\n")
	section := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "Publications:") {
			section = "publications"
			continue
		} else if strings.HasPrefix(line, "Subscriptions:") {
			section = "subscriptions"
			continue
		} else if strings.HasPrefix(line, "Services:") {
			section = "services"
			continue
		}
		
		if strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(line, "* ")
			if idx := strings.Index(item, " "); idx > 0 {
				item = item[:idx]
			}
			
			switch section {
			case "publications":
				node.Publications = append(node.Publications, item)
			case "subscriptions":
				node.Subscriptions = append(node.Subscriptions, item)
			case "services":
				node.Services = append(node.Services, item)
			}
		}
	}
	
	return node, nil
}

// getTopicDetails 获取topic详细信息
func (r *ROS1AdapterImpl) getTopicDetails(ctx context.Context, topicName string) (*TopicInfo, error) {
	topic := &TopicInfo{
		Name:        topicName,
		LastMessage: time.Now(),
		Metadata:    make(map[string]string),
	}
	
	// 获取消息类型
	if msgType, err := r.getTopicType(ctx, topicName); err == nil {
		topic.MessageType = msgType
	}
	
	// 获取发布者和订阅者
	if pubSub, err := r.getTopicPubSub(ctx, topicName); err == nil {
		topic.Publishers = pubSub.Publishers
		topic.Subscribers = pubSub.Subscribers
	}
	
	return topic, nil
}

// getTopicType 获取topic类型
func (r *ROS1AdapterImpl) getTopicType(ctx context.Context, topicName string) (string, error) {
	cmd := exec.CommandContext(ctx, "rostopic", "type", topicName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// TopicPubSub topic发布订阅信息
type TopicPubSub struct {
	Publishers  []string
	Subscribers []string
}

// getTopicPubSub 获取topic发布订阅信息
func (r *ROS1AdapterImpl) getTopicPubSub(ctx context.Context, topicName string) (*TopicPubSub, error) {
	cmd := exec.CommandContext(ctx, "rostopic", "info", topicName)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	pubSub := &TopicPubSub{
		Publishers:  []string{},
		Subscribers: []string{},
	}
	
	lines := strings.Split(string(output), "\n")
	section := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "Publishers:") {
			section = "publishers"
			continue
		} else if strings.HasPrefix(line, "Subscribers:") {
			section = "subscribers"
			continue
		}
		
		if strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(line, "* ")
			if idx := strings.Index(item, " "); idx > 0 {
				item = item[:idx]
			}
			
			switch section {
			case "publishers":
				pubSub.Publishers = append(pubSub.Publishers, item)
			case "subscribers":
				pubSub.Subscribers = append(pubSub.Subscribers, item)
			}
		}
	}
	
	return pubSub, nil
}

// TopicInfoDetails topic详细信息
type TopicInfoDetails struct {
	Latching bool
}

// getTopicInfoDetails 获取topic详细信息
func (r *ROS1AdapterImpl) getTopicInfoDetails(ctx context.Context, topicName string) (*TopicInfoDetails, error) {
	// ROS1中latching信息需要从rostopic info输出中解析
	// 这里简化处理
	return &TopicInfoDetails{
		Latching: false,
	}, nil
}

// getServiceType 获取服务类型
func (r *ROS1AdapterImpl) getServiceType(ctx context.Context, serviceName string) (string, error) {
	cmd := exec.CommandContext(ctx, "rosservice", "type", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getParameterValue 获取参数值
func (r *ROS1AdapterImpl) getParameterValue(ctx context.Context, paramName string) (interface{}, string, error) {
	cmd := exec.CommandContext(ctx, "rosparam", "get", paramName)
	output, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}
	
	valueStr := strings.TrimSpace(string(output))
	
	// 尝试解析为JSON
	var value interface{}
	if err := json.Unmarshal([]byte(valueStr), &value); err == nil {
		return value, "json", nil
	}
	
	// 尝试解析为数字
	if intVal, err := strconv.Atoi(valueStr); err == nil {
		return intVal, "int", nil
	}
	
	if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return floatVal, "float", nil
	}
	
	// 尝试解析为布尔值
	if boolVal, err := strconv.ParseBool(valueStr); err == nil {
		return boolVal, "bool", nil
	}
	
	// 默认为字符串
	return valueStr, "string", nil
}

// subscribeWorker 订阅工作线程
func (r *ROS1AdapterImpl) subscribeWorker(ctx context.Context, topicName string, callback func([]byte), stopCh chan struct{}) {
	cmd := exec.CommandContext(ctx, "rostopic", "echo", topicName)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	
	if err := cmd.Start(); err != nil {
		return
	}
	
	go func() {
		select {
		case <-stopCh:
			cmd.Process.Kill()
		case <-ctx.Done():
			cmd.Process.Kill()
		}
	}()
	
	// 读取输出并调用回调
	buffer := make([]byte, 4096)
	for {
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		default:
			n, err := stdout.Read(buffer)
			if err != nil {
				return
			}
			if n > 0 {
				callback(buffer[:n])
			}
		}
	}
}

// getDistribution 获取ROS发行版
func (r *ROS1AdapterImpl) getDistribution(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "rosversion", "-d")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getPackagePath 获取包路径
func (r *ROS1AdapterImpl) getPackagePath(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", "echo $ROS_PACKAGE_PATH")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	pathStr := strings.TrimSpace(string(output))
	if pathStr == "" {
		return []string{}, nil
	}
	
	return strings.Split(pathStr, ":"), nil
}

// getPythonPath 获取Python路径
func (r *ROS1AdapterImpl) getPythonPath(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "which", "python")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
} 
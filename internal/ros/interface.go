package ros

import (
	"context"
	"time"
)

// ROSVersion ROS版本枚举 (简化为仅支持ROS1)
type ROSVersion int

const (
	ROSVersionUnknown ROSVersion = iota
	ROSVersion1
)

func (v ROSVersion) String() string {
	switch v {
	case ROSVersion1:
		return "ROS1"
	default:
		return "Unknown"
	}
}

// NodeInfo 节点信息
type NodeInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Publications []string          `json:"publications"`
	Subscriptions []string         `json:"subscriptions"`
	Services     []string          `json:"services"`
	IsActive     bool              `json:"is_active"`
	PID          int               `json:"pid,omitempty"`
	Host         string            `json:"host,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TopicInfo Topic信息 (ROS1专用)
type TopicInfo struct {
	Name          string            `json:"name"`
	MessageType   string            `json:"message_type"`
	Publishers    []string          `json:"publishers"`
	Subscribers   []string          `json:"subscribers"`
	Frequency     float64           `json:"frequency"`
	Bandwidth     float64           `json:"bandwidth"`
	MessageCount  int64             `json:"message_count"`
	LastMessage   time.Time         `json:"last_message"`
	Latching      bool              `json:"latching,omitempty"`    // ROS1 latching属性
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name        string            `json:"name"`
	ServiceType string            `json:"service_type"`
	Providers   []string          `json:"providers"`
	IsActive    bool              `json:"is_active"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ParameterInfo 参数信息
type ParameterInfo struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	ReadOnly    bool        `json:"read_only,omitempty"`
}

// SystemInfo ROS1系统信息
type SystemInfo struct {
	Version       ROSVersion        `json:"version"`
	Distribution  string            `json:"distribution"`  // melodic, noetic等ROS1发行版
	MasterURI     string            `json:"master_uri"`    // ROS Master URI
	PythonPath    string            `json:"python_path"`
	PackagePath   []string          `json:"package_path"`
	Environment   map[string]string `json:"environment"`
	Uptime        time.Duration     `json:"uptime"`
}

// ROSInterface ROS1接口
type ROSInterface interface {
	// 系统信息
	GetVersion() ROSVersion
	GetSystemInfo(ctx context.Context) (*SystemInfo, error)
	IsAvailable(ctx context.Context) bool
	
	// 节点管理
	ListNodes(ctx context.Context) ([]NodeInfo, error)
	GetNodeInfo(ctx context.Context, nodeName string) (*NodeInfo, error)
	IsNodeActive(ctx context.Context, nodeName string) bool
	
	// Topic管理
	ListTopics(ctx context.Context) ([]TopicInfo, error)
	GetTopicInfo(ctx context.Context, topicName string) (*TopicInfo, error)
	GetTopicFrequency(ctx context.Context, topicName string, duration time.Duration) (float64, error)
	
	// 服务管理
	ListServices(ctx context.Context) ([]ServiceInfo, error)
	GetServiceInfo(ctx context.Context, serviceName string) (*ServiceInfo, error)
	
	// 参数管理
	ListParameters(ctx context.Context) ([]ParameterInfo, error)
	GetParameter(ctx context.Context, paramName string) (*ParameterInfo, error)
	SetParameter(ctx context.Context, paramName string, value interface{}) error
	
	// 监控功能
	Subscribe(ctx context.Context, topicName string, callback func([]byte)) error
	Unsubscribe(ctx context.Context, topicName string) error
	
	// 健康检查
	HealthCheck(ctx context.Context) error
	
	// 清理资源
	Close() error
}

// ROS1Adapter ROS1适配器接口
type ROS1Adapter interface {
	ROSInterface
	
	// 适配器特定方法
	Initialize(config map[string]interface{}) error
	GetAdapterName() string
	GetSupportedFeatures() []string
}

// MessageData ROS消息数据
type MessageData struct {
	Topic     string                 `json:"topic"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Sequence  uint64                 `json:"sequence,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	IsHealthy     bool              `json:"is_healthy"`
	Issues        []string          `json:"issues,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	LastCheck     time.Time         `json:"last_check"`
	ResponseTime  time.Duration     `json:"response_time"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ROSError ROS错误类型
type ROSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Context string `json:"context,omitempty"`
}

func (e *ROSError) Error() string {
	if e.Context != "" {
		return e.Code + ": " + e.Message + " (" + e.Context + ")"
	}
	return e.Code + ": " + e.Message
}

// 常见错误代码
const (
	ErrorCodeConnectionFailed  = "CONNECTION_FAILED"
	ErrorCodeNodeNotFound      = "NODE_NOT_FOUND"
	ErrorCodeTopicNotFound     = "TOPIC_NOT_FOUND"
	ErrorCodeServiceNotFound   = "SERVICE_NOT_FOUND"
	ErrorCodeParameterNotFound = "PARAMETER_NOT_FOUND"
	ErrorCodePermissionDenied  = "PERMISSION_DENIED"
	ErrorCodeTimeout           = "TIMEOUT"
	ErrorCodeVersionMismatch   = "VERSION_MISMATCH"
	ErrorCodeUnsupported       = "UNSUPPORTED_OPERATION"
) 
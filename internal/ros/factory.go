package ros

import (
	"context"
	"fmt"
)

// AdapterFactory ROS1适配器工厂
type AdapterFactory struct {
	detector *Detector
}

// NewAdapterFactory 创建适配器工厂
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		detector: NewDetector(),
	}
}

// CreateROS1Adapter 创建ROS1适配器
func (f *AdapterFactory) CreateROS1Adapter(ctx context.Context, config map[string]interface{}) (ROS1Adapter, error) {
	// 检测ROS1环境
	result, err := f.detector.DetectROS1Environment(ctx)
	if err != nil {
		return nil, fmt.Errorf("检测ROS1环境失败: %w", err)
	}

	if !result.IsROS1Available {
		return nil, fmt.Errorf("ROS1环境不可用")
	}

	// 创建适配器
	adapter := NewROS1Adapter()

	// 合并配置
	finalConfig := f.mergeConfigs(config, f.detector.GetRecommendedConfiguration(result))

	// 初始化适配器
	if err := adapter.Initialize(finalConfig); err != nil {
		return nil, fmt.Errorf("初始化ROS1适配器失败: %w", err)
	}

	return adapter, nil
}

// DetectAndCreateAdapter 自动检测并创建适配器
func (f *AdapterFactory) DetectAndCreateAdapter(ctx context.Context, userConfig map[string]interface{}) (ROS1Adapter, error) {
	return f.CreateROS1Adapter(ctx, userConfig)
}

// ValidateROS1Environment 验证ROS1环境
func (f *AdapterFactory) ValidateROS1Environment(ctx context.Context) error {
	return f.detector.ValidateROS1Environment(ctx)
}

// GetEnvironmentInfo 获取环境信息
func (f *AdapterFactory) GetEnvironmentInfo(ctx context.Context) (*DetectResult, error) {
	return f.detector.DetectROS1Environment(ctx)
}

// mergeConfigs 合并配置
func (f *AdapterFactory) mergeConfigs(userConfig, detectedConfig map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// 首先添加检测到的配置
	for k, v := range detectedConfig {
		merged[k] = v
	}

	// 用户配置覆盖检测到的配置
	for k, v := range userConfig {
		merged[k] = v
	}

	return merged
}

// GetDefaultConfig 获取默认ROS1配置
func (f *AdapterFactory) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"ros_version": "1",
		"master_uri":  "http://localhost:11311",
		"timeout":     5,
		"cache_ttl":   30,
	}
} 
package b2

import (
	"fmt"

	"ros_exporter/internal/config"
)

// New 按配置创建 B2DataSource 实例。
//
// 选择规则：
//   - cfg.DataSource == "dds"  → 调 newDDSDataSource（cgo+linux 时连真 DDS，否则 stub 返回错误）
//   - cfg.DataSource == "mock" → 返回 mock 实现（用于 macOS、CI、显式 mock 调试）
//   - cfg.DataSource == ""     → 默认 "mock"，向后兼容旧 config.yaml（无此字段时不会因 DDS 不可用而启动失败）
func New(cfg *config.B2CollectorConfig) (B2DataSource, error) {
	src := cfg.DataSource
	if src == "" {
		src = "mock"
	}
	switch src {
	case "dds":
		return newDDSDataSource(cfg)
	case "mock":
		return newMockDataSource(cfg), nil
	default:
		return nil, fmt.Errorf("不支持的 b2.data_source: %q（支持: dds, mock）", src)
	}
}

package g1

import (
	"fmt"

	"ros_exporter/internal/config"
)

// New 按配置创建 G1DataSource。
//
// cfg.G1DataSource 取值：
//   - "sdk"  → newSDKDataSource（cgo 编译时调真实 SDK；nocgo 编译时 sdk_stub.go 返回错误）
//   - "mock" → newMockDataSource，确定性数据
//   - ""     → 默认按构建模式：cgo → sdk，nocgo → mock
//
// 默认值的设计原因：保持向后兼容。改造前 cgo 构建直接走真 SDK，nocgo 构建走 mock；
// 改造后旧 config.yaml（无此字段）应保持完全相同的行为。
func New(cfg *config.BMSCollectorConfig) (G1DataSource, error) {
	src := cfg.G1DataSource
	if src == "" {
		src = defaultG1DataSource()
	}
	switch src {
	case "sdk":
		return newSDKDataSource(cfg)
	case "mock":
		return newMockDataSource(cfg), nil
	default:
		return nil, fmt.Errorf("不支持的 g1_data_source: %q（支持: sdk, mock）", src)
	}
}

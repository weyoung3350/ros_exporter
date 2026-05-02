//go:build !cgo || !linux || !g1sdk
// +build !cgo !linux !g1sdk

package g1

import (
	"errors"

	"ros_exporter/internal/config"
)

// defaultG1DataSource nocgo 编译时默认 mock（保持改造前行为：nocgo 跑模拟数据）。
func defaultG1DataSource() string { return "mock" }

// newSDKDataSource 在 nocgo 构建中返回明确错误，避免静默退化为 mock。
//
// 区分语义：
//   - mock：用户主动选用 mock 数据源（开发、测试、显式调试）
//   - stub：用户配置了 sdk 但当前构建不支持，应该报错而不是假装在跑
func newSDKDataSource(_ *config.BMSCollectorConfig) (G1DataSource, error) {
	return nil, errors.New(
		"G1 SDK 数据源在当前构建中不可用（需要 cgo + libg1sdk.a）。" +
			"请改 bms.g1_data_source: mock，或重新编译 cgo 版本（要求装好 internal/sdk/unitree 静态库）")
}

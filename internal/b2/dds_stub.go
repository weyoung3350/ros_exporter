//go:build !cgo || !linux
// +build !cgo !linux

// dds_stub.go — 当前构建（macOS / nocgo / 其他非 linux 平台）下的 DataSource 占位。
//
// 设计意图：让用户在不支持 DDS 的环境下也能 go build 成功，
// 但运行时若配置 data_source: dds 会立刻报错（不静默退化为 mock，避免假装在跑）。
//
// 真正的 DDS 实现见 dds_cgo.go（仅 cgo && linux 平台）。

package b2

import (
	"errors"

	"ros_exporter/internal/config"
)

func newDDSDataSource(_ *config.B2CollectorConfig) (B2DataSource, error) {
	return nil, errors.New(
		"B2 DDS 数据源在当前构建中不可用（需要 cgo + linux + 已安装 unitree_sdk2，且 dds_cgo.go 已落地）。" +
			"请改 b2.data_source: mock，或在 Thor 上跑 deploy/install_b2_thor.sh 完成 SDK 安装与编译")
}

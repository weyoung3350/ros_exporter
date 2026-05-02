//go:build cgo && linux && g1sdk
// +build cgo,linux,g1sdk

// 默认编译不启用 G1 cgo——需要显式 `go build -tags g1sdk` 且环境装好 libg1sdk.a。
// 没装 G1 SDK 的机器（如本次 B2 部署的 AGX-Thor）直接走 sdk_stub.go，
// 用户可在 config 中选 g1_data_source: mock 跑模拟数据。

package g1

/*
#cgo CPPFLAGS: -I../sdk/unitree
#cgo LDFLAGS: -L../sdk/unitree -lg1sdk -lstdc++ -lm
#include "g1_sdk.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"ros_exporter/internal/config"
)

// sdkDataSource 通过 CGO 调 internal/sdk/unitree/g1_sdk.cpp 编出的 libg1sdk.a。
//
// 与改造前 types.G1SDK 的 cgo 部分逻辑等价（仅迁移位置 + 实现 G1DataSource interface）。
// 行为零变化的硬约束由 plan v3 第 3.10 节定义：cgo 路径下 robot_battery_* 数值序列必须与
// 改造前完全一致（差值 < 1e-9）。
//
// defaultG1DataSource 在 cgo 编译时返回 "sdk"，匹配改造前默认走真实 SDK 的行为。
type sdkDataSource struct {
	mu          sync.Mutex
	cfg         *config.BMSCollectorConfig
	initialized bool
	connected   bool
}

// defaultG1DataSource cgo 编译时默认 sdk（保持改造前行为）。
func defaultG1DataSource() string { return "sdk" }

func newSDKDataSource(cfg *config.BMSCollectorConfig) (*sdkDataSource, error) {
	return &sdkDataSource{cfg: cfg}, nil
}

func (s *sdkDataSource) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		var cConfigPath *C.char
		if s.cfg != nil && s.cfg.SDKConfigPath != "" {
			cConfigPath = C.CString(s.cfg.SDKConfigPath)
			defer C.free(unsafe.Pointer(cConfigPath))
		}
		if rc := C.g1_sdk_init(cConfigPath); rc != 0 {
			return fmt.Errorf("G1 SDK 初始化失败: %s", s.lastError())
		}
		s.initialized = true
	}

	if s.connected {
		return nil
	}
	if rc := C.g1_sdk_connect(); rc != 0 {
		return fmt.Errorf("G1 SDK 连接失败: %s", s.lastError())
	}
	s.connected = true
	return nil
}

func (s *sdkDataSource) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.connected {
		return nil
	}
	if rc := C.g1_sdk_disconnect(); rc != 0 {
		return fmt.Errorf("G1 SDK 断开失败: %s", s.lastError())
	}
	s.connected = false
	return nil
}

func (s *sdkDataSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected {
		C.g1_sdk_disconnect()
		s.connected = false
	}
	if s.initialized {
		C.g1_sdk_cleanup()
		s.initialized = false
	}
	return nil
}

func (s *sdkDataSource) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.connected {
		return false
	}
	// 也读真实 SDK 的连接状态（防止 SDK 内部断开但本地 flag 没更新）
	return bool(C.g1_sdk_is_connected())
}

func (s *sdkDataSource) GetBatteryStatus() (*BatteryStatus, error) {
	s.mu.Lock()
	if !s.connected {
		s.mu.Unlock()
		return nil, errors.New("未连接到 G1 机器人")
	}

	var cStatus C.G1BatteryStatus
	rc := C.g1_sdk_get_battery_status(&cStatus)
	s.mu.Unlock()

	if rc != 0 {
		return nil, fmt.Errorf("获取 G1 电池状态失败: %s", s.lastError())
	}

	return convertCBattery(&cStatus), nil
}

func (s *sdkDataSource) lastError() string {
	buffer := make([]byte, 256)
	cBuffer := (*C.char)(unsafe.Pointer(&buffer[0]))
	C.g1_sdk_get_last_error(cBuffer, 256)
	return C.GoString(cBuffer)
}

// convertCBattery 把 C struct 转成 Go BatteryStatus。
// 与改造前 types/g1_types.go 第 150-176 行字段映射保持一致。
func convertCBattery(c *C.G1BatteryStatus) *BatteryStatus {
	status := &BatteryStatus{
		Voltage:       float64(c.voltage),
		Current:       float64(c.current),
		Temperature:   float64(c.temperature),
		Capacity:      float64(c.capacity),
		CycleCount:    uint32(c.cycle_count),
		IsCharging:    bool(c.is_charging),
		IsDischarging: bool(c.is_discharging),
		HealthStatus:  uint8(c.health_status),
		ErrorCode:     uint32(c.error_code),
		ErrorMessage:  C.GoString(&c.error_message[0]),
		// SDK timestamp 是 ms，转 Go time
		Timestamp: time.Unix(0, int64(c.timestamp)*int64(time.Millisecond)),
	}

	status.CellVoltages = make([]float64, 40)
	for i := 0; i < 40; i++ {
		status.CellVoltages[i] = float64(c.cell_voltages[i])
	}
	status.Temperatures = make([]float64, 12)
	for i := 0; i < 12; i++ {
		status.Temperatures[i] = float64(c.temperatures[i])
	}
	return status
}

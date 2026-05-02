package g1

import (
	"errors"
	"sync"
	"time"

	"ros_exporter/internal/config"
)

// mockDataSource G1 数据源模拟实现。
//
// 数据严格保持与重构前 internal/types/g1_types_nocgo.go 一致：除 Timestamp 外
// 所有字段值 bit-for-bit 相同（mock 数据是常量公式，不引入随机性）。
// 单测会回归这个一致性约束。
type mockDataSource struct {
	mu          sync.Mutex
	initialized bool
	connected   bool
}

func newMockDataSource(_ *config.BMSCollectorConfig) *mockDataSource {
	return &mockDataSource{}
}

func (m *mockDataSource) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialized = true
	m.connected = true
	return nil
}

func (m *mockDataSource) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

func (m *mockDataSource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.initialized = false
	return nil
}

func (m *mockDataSource) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// GetBatteryStatus 返回与改造前 g1_types_nocgo.go 第 113-138 行完全一致的数据。
// 数值字段（Voltage/Current/Temperature/Capacity 等）确定，仅 Timestamp 用 time.Now() 漂移。
func (m *mockDataSource) GetBatteryStatus() (*BatteryStatus, error) {
	if !m.IsConnected() {
		return nil, errors.New("未连接到 G1 机器人（mock 数据源）")
	}

	status := &BatteryStatus{
		Voltage:       25.2,  // 6S 电池组电压
		Current:       -2.5,  // 放电电流
		Temperature:   35.0,  // 温度
		Capacity:      87.5,  // 电量 87.5%
		CycleCount:    245,   // 循环次数
		IsCharging:    false,
		IsDischarging: true,
		HealthStatus:  92, // 健康度 92%
		ErrorCode:     0,
		ErrorMessage:  "",
		Timestamp:     time.Now(),
	}

	// 40 节单体电压，与改造前公式一致：4.05 + (i%10)*0.01 → 4.05V - 4.14V
	status.CellVoltages = make([]float64, 40)
	for i := 0; i < 40; i++ {
		status.CellVoltages[i] = 4.05 + float64(i%10)*0.01
	}

	// 12 个温度传感器，与改造前公式一致：32.0 + (i%8)*0.5 → 32°C - 35.5°C
	status.Temperatures = make([]float64, 12)
	for i := 0; i < 12; i++ {
		status.Temperatures[i] = 32.0 + float64(i%8)*0.5
	}

	return status, nil
}

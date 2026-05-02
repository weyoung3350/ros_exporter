package b2

import (
	"sync"
	"time"

	"ros_exporter/internal/config"
)

// mockDataSource 是 B2DataSource 的内置模拟实现。
//
// 用途：
//   - macOS / Windows 开发机本地跑通整个采集 pipeline
//   - CI 单元测试（B2 真实 DDS 不可达）
//   - config.yaml 中显式 data_source: mock 用于排查代码层问题（与 DDS 数据源解耦）
//
// 数据特性：
//   - 数值字段确定性（不随机），便于回归测试做 bit-for-bit 比较
//   - 仅 Received / LastSeen 等时间戳字段使用 time.Now()，会随时间漂移
//   - 模拟"健康"状态：DDS 已连接、最近收包、无错误
type mockDataSource struct {
	cfg *config.B2CollectorConfig

	mu        sync.Mutex
	connected bool
	startTime time.Time // Connect 时刻，用于推算累计时间
}

func newMockDataSource(cfg *config.B2CollectorConfig) *mockDataSource {
	return &mockDataSource{cfg: cfg}
}

func (m *mockDataSource) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	m.startTime = time.Now()
	return nil
}

func (m *mockDataSource) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	return nil
}

func (m *mockDataSource) Close() error {
	return m.Disconnect()
}

func (m *mockDataSource) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockDataSource) GetSnapshot() (*Snapshot, error) {
	m.mu.Lock()
	connected := m.connected
	m.mu.Unlock()
	if !connected {
		return nil, errMockNotConnected
	}

	now := time.Now()
	return &Snapshot{
		LowState:   mockLowState(now),
		SportState: mockSportState(now),
	}, nil
}

func (m *mockDataSource) Health() Health {
	m.mu.Lock()
	connected := m.connected
	m.mu.Unlock()
	now := time.Now()
	return Health{
		DDSConnected:       connected,
		LowStateLastSeen:   now,
		SportStateLastSeen: now,
	}
}

// mockLowState 返回确定性数值的 LowState（除 Received 外）。
//
// 设计：每个字段用一个固定公式生成，便于测试断言精确值。
// IMU 取一个静止站立姿态：四元数 (1,0,0,0)、重力加速度 9.8 z 方向。
// 关节温度 35-46 °C 阶梯、扭矩 0、关节角按腿和关节类型给典型站姿值。
// 电池模拟一个 75% SOC、放电中、无故障的健康状态。
func mockLowState(now time.Time) *LowState {
	temps := make([]float64, JointCount)
	torques := make([]float64, JointCount)
	angles := make([]float64, JointCount)
	velocities := make([]float64, JointCount)
	modes := make([]uint8, JointCount)
	lost := make([]uint32, JointCount)
	statuses := make([]uint32, JointCount)

	for i := 0; i < JointCount; i++ {
		// 温度 35-46 °C 递增（每个关节 +1）
		temps[i] = 35.0 + float64(i)
		// 扭矩 0（站立无负载）
		torques[i] = 0.0
		// 站立姿态典型角度（按 hip_roll=0, hip_pitch=0.6, knee=-1.4 给）
		switch i % 3 {
		case 0:
			angles[i] = 0.0
		case 1:
			angles[i] = 0.6
		case 2:
			angles[i] = -1.4
		}
		velocities[i] = 0.0
		modes[i] = 1   // 1 = enabled / position control（典型）
		lost[i] = 0    // 通信无丢包
		statuses[i] = 0 // 无错误
	}

	// 电池 cell voltage 固定 15 节（已用 unitree_sdk2 BmsState_.hpp 核对）
	var cells [15]uint16
	for i := 0; i < 15; i++ {
		// 4000-4028 mV 范围（健康电池单体压差应 <50mV）
		cells[i] = 4000 + uint16(i*2)
	}

	return &LowState{
		IMU: IMU{
			Quaternion: [4]float64{1.0, 0.0, 0.0, 0.0},  // 静止站立
			Gyroscope:  [3]float64{0.0, 0.0, 0.0},
			Accel:      [3]float64{0.0, 0.0, 9.81},      // 重力
		},
		Joints: Joints{
			Temperatures: temps,
			Torques:      torques,
			Angles:       angles,
			Velocities:   velocities,
			Modes:        modes,
			LostCounts:   lost,
			StatusCodes:  statuses,
			FootForce:    [4]int16{120, 130, 110, 125}, // 站立时四足均匀承重
			FootForceEst: [4]int16{118, 132, 108, 127},
		},
		Battery: Battery{
			VersionHigh:  1,
			VersionLow:   0,
			SOC:          75,
			Current:      -2500, // 放电 2.5A
			Cycle:        42,
			BQNTC:        [2]uint8{34, 35},
			MCUNTC:       [2]uint8{32, 33},
			CellVoltages: cells,
			Status:       0,
		},
		Safety: Safety{
			EmergencyStop: false,
			BMSError:      false,
		},
		Received: now,
	}
}

func mockSportState(now time.Time) *SportState {
	return &SportState{
		VelocityBody: [3]float64{0.0, 0.0, 0.0}, // 静止
		ModeRaw:      5,                          // 5 = stand（典型）
		GaitTypeRaw:  0,                          // 0 = idle
		Received:     now,
	}
}

// errMockNotConnected 在未连接时调用 GetSnapshot 时返回。
// 不导出，避免误用。
var errMockNotConnected = newMockError("B2 mock 数据源未连接，请先调用 Connect()")

type mockError string

func (e mockError) Error() string { return string(e) }
func newMockError(s string) error { return mockError(s) }

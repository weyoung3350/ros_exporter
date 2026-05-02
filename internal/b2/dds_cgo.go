//go:build cgo && linux
// +build cgo,linux

// dds_cgo.go — B2DataSource 的 DDS 实现，通过 internal/sdk/unitree/libb2sdk.a 调 unitree_sdk2。
//
// 编译前置（在 Thor 或同等 ARM64 Linux）：
//   1. unitree_sdk2 已 sudo make install 到 /usr/local
//   2. /usr/local/lib 在 ld 搜索路径（参见 docs/B2_INTEGRATION.md）
//   3. 在 internal/sdk/unitree/ 跑 `make -f Makefile.b2` 生成 libb2sdk.a
//
// 然后 `go build` 即可。

package b2

/*
#cgo CPPFLAGS: -I../sdk/unitree
#cgo LDFLAGS: -L${SRCDIR}/../sdk/unitree -lb2sdk -L/usr/local/lib -lunitree_sdk2 -lddscxx -lddsc -lpthread -ldl -lm -lstdc++

#include <stdlib.h>
#include "b2_sdk.h"
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

// ddsDataSource 通过 unitree_sdk2 + cyclonedds 直连 B2 主控。
//
// 内部把 IO 全部委托给 internal/sdk/unitree/libb2sdk.a 的 C wrapper：
//   - wrapper 在后台线程异步收 DDS 消息，缓存最新值
//   - GetSnapshot 只是从 wrapper 读缓存（无 IO 阻塞）
//   - DDS 自愈（last_seen 检测、超龄重建）也在 wrapper 内
//
// Go 侧主要做：
//   - cgo 调用入口
//   - C POD struct → Go struct 转换
//   - 单实例守护（unitree_sdk2 ChannelFactory 是进程级单例，并发 Connect 会出问题）
type ddsDataSource struct {
	cfg       *config.B2CollectorConfig
	mu        sync.Mutex
	connected bool
}

// 全局守护，防止同进程多次 Connect 重复初始化 ChannelFactory。
// codex I-2 也提到：单进程不应跑多个 B2 DataSource 实例。
var (
	ddsGlobalMu sync.Mutex
	ddsActive   bool // 当前是否已有实例 Connect
)

func newDDSDataSource(cfg *config.B2CollectorConfig) (B2DataSource, error) {
	return &ddsDataSource{cfg: cfg}, nil
}

func (d *ddsDataSource) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.connected {
		return nil
	}

	ddsGlobalMu.Lock()
	if ddsActive {
		ddsGlobalMu.Unlock()
		return errors.New("进程内已有 B2 DDS 数据源在运行（unitree_sdk2 ChannelFactory 是单例，不支持多实例）")
	}
	ddsActive = true
	ddsGlobalMu.Unlock()

	cIface := C.CString(d.cfg.NetworkInterface)
	cLow := C.CString(defaultIfEmpty(d.cfg.LowStateTopic, "rt/lowstate"))
	cSport := C.CString(defaultIfEmpty(d.cfg.SportStateTopic, "rt/sportmodestate"))
	defer C.free(unsafe.Pointer(cIface))
	defer C.free(unsafe.Pointer(cLow))
	defer C.free(unsafe.Pointer(cSport))

	staleMs := durationOrDefault(d.cfg.DDSStaleThreshold, 5*time.Second)
	if rc := C.b2_dds_init(cIface, cLow, cSport, C.int(staleMs)); rc != 0 {
		ddsGlobalMu.Lock()
		ddsActive = false
		ddsGlobalMu.Unlock()
		return fmt.Errorf("b2_dds_init 失败: %s", C.GoString(C.b2_dds_last_error()))
	}

	// 等首包：让 Connect 在网络/拓扑不通时显式失败，而不是后续 GetSnapshot 时返回空数据
	connectTimeoutMs := durationOrDefault(d.cfg.DDSConnectTimeout, 5*time.Second)
	if rc := C.b2_dds_wait_first_packet(C.int(connectTimeoutMs)); rc != 0 {
		// 首包没到通常是 DDS 多播被防火墙阻塞、网卡未配 multicast、或 B2 主控离线
		// 不立即清理（让用户能拿到 health 看具体什么时候开始有数据），但视作连接失败
		C.b2_dds_close()
		ddsGlobalMu.Lock()
		ddsActive = false
		ddsGlobalMu.Unlock()
		return fmt.Errorf("等待 lowstate 首包超时: %s", C.GoString(C.b2_dds_last_error()))
	}

	d.connected = true
	return nil
}

func (d *ddsDataSource) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.connected {
		return nil
	}
	C.b2_dds_close()
	d.connected = false
	ddsGlobalMu.Lock()
	ddsActive = false
	ddsGlobalMu.Unlock()
	return nil
}

func (d *ddsDataSource) Close() error {
	return d.Disconnect()
}

func (d *ddsDataSource) IsConnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connected
}

func (d *ddsDataSource) GetSnapshot() (*Snapshot, error) {
	if !d.IsConnected() {
		return nil, errors.New("B2 DDS 数据源未连接")
	}

	var c C.B2RawSnapshot
	if rc := C.b2_dds_get_snapshot(&c); rc != 0 {
		return nil, fmt.Errorf("b2_dds_get_snapshot 失败 (rc=%d)", int(rc))
	}

	snap := &Snapshot{}
	if c.has_low_state != 0 {
		snap.LowState = convertLowState(&c.low_state)
	}
	if c.has_sport_state != 0 {
		snap.SportState = convertSportState(&c.sport_state)
	}
	return snap, nil
}

func (d *ddsDataSource) Health() Health {
	var c C.B2RawHealth
	if rc := C.b2_dds_get_health(&c); rc != 0 {
		return Health{LastError: fmt.Sprintf("b2_dds_get_health rc=%d", int(rc))}
	}
	now := time.Now()
	h := Health{
		DDSConnected:   c.dds_connected != 0,
		ReconnectCount: uint64(c.reconnect_count),
		ErrorCount:     uint64(c.error_count),
		LastError:      C.GoString(&c.last_error[0]),
	}
	// age_ns 是单调时钟下"距上次收到的纳秒数"，转回 Wall clock 的 LastSeen 用 now - age 近似
	if int64(c.low_state_age_ns) >= 0 && int64(c.low_state_age_ns) < int64(time.Hour) {
		h.LowStateLastSeen = now.Add(-time.Duration(c.low_state_age_ns))
	}
	if int64(c.sport_state_age_ns) >= 0 && int64(c.sport_state_age_ns) < int64(time.Hour) {
		h.SportStateLastSeen = now.Add(-time.Duration(c.sport_state_age_ns))
	}
	return h
}

// ----- C → Go 字段转换 -----

func convertLowState(c *C.B2RawLowState) *LowState {
	ls := &LowState{
		IMU: IMU{
			Quaternion: [4]float64{
				float64(c.imu.quaternion[0]),
				float64(c.imu.quaternion[1]),
				float64(c.imu.quaternion[2]),
				float64(c.imu.quaternion[3]),
			},
			Gyroscope: [3]float64{
				float64(c.imu.gyroscope[0]),
				float64(c.imu.gyroscope[1]),
				float64(c.imu.gyroscope[2]),
			},
			Accel: [3]float64{
				float64(c.imu.accelerometer[0]),
				float64(c.imu.accelerometer[1]),
				float64(c.imu.accelerometer[2]),
			},
		},
		Joints: Joints{
			Temperatures: make([]float64, JointCount),
			Torques:      make([]float64, JointCount),
			Angles:       make([]float64, JointCount),
			Velocities:   make([]float64, JointCount),
			Modes:        make([]uint8, JointCount),
			LostCounts:   make([]uint32, JointCount),
			StatusCodes:  make([]uint32, JointCount),
		},
	}
	for i := 0; i < JointCount; i++ {
		ls.Joints.Temperatures[i] = float64(c.motors[i].temperature)
		ls.Joints.Torques[i] = float64(c.motors[i].tau_est)
		ls.Joints.Angles[i] = float64(c.motors[i].q)
		ls.Joints.Velocities[i] = float64(c.motors[i].dq)
		ls.Joints.Modes[i] = uint8(c.motors[i].mode)
		ls.Joints.LostCounts[i] = uint32(c.motors[i].lost)
		ls.Joints.StatusCodes[i] = uint32(c.motors[i].reserve)
	}
	for i := 0; i < FootCount; i++ {
		ls.Joints.FootForce[i] = int16(c.foot_force[i])
		ls.Joints.FootForceEst[i] = int16(c.foot_force_est[i])
	}
	ls.Battery = Battery{
		VersionHigh: uint8(c.bms.version_high),
		VersionLow:  uint8(c.bms.version_low),
		Status:      uint8(c.bms.status),
		SOC:         uint8(c.bms.soc),
		Current:     int32(c.bms.current),
		Cycle:       uint16(c.bms.cycle),
	}
	for i := 0; i < 2; i++ {
		ls.Battery.BQNTC[i] = uint8(c.bms.bq_ntc[i])
		ls.Battery.MCUNTC[i] = uint8(c.bms.mcu_ntc[i])
	}
	for i := 0; i < 15; i++ {
		ls.Battery.CellVoltages[i] = uint16(c.bms.cell_vol[i])
	}
	// Safety：SDK 暂未直接暴露 emergency_stop 字段，此处保留默认 false。
	// 若未来 wrapper 解析 wireless_remote 等字段判定急停，再补。
	ls.Received = time.Now() // wrapper 收的本机单调时刻不便转 wall clock，简化用 GetSnapshot 时刻
	return ls
}

func convertSportState(c *C.B2RawSportState) *SportState {
	return &SportState{
		VelocityBody: [3]float64{
			float64(c.velocity[0]),
			float64(c.velocity[1]),
			float64(c.velocity[2]),
		},
		ModeRaw:     uint8(c.mode),
		GaitTypeRaw: uint8(c.gait_type),
		Received:    time.Now(),
	}
}

// ----- helper -----

func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func durationOrDefault(d, def time.Duration) int {
	if d <= 0 {
		d = def
	}
	return int(d / time.Millisecond)
}

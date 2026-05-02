// Package b2 提供宇树 B2 工业级四足机器人的数据源抽象。
//
// 数据源有多种实现：
//   - dds_cgo.go：通过 unitree_sdk2 + DDS 直接和 B2 主控通信（CGO + Linux）
//   - mock.go：确定性模拟数据，用于 macOS 开发、CI 单元测试、配置 data_source: mock 时
//   - dds_stub.go：当前构建不支持 DDS（!cgo 或 !linux）时的占位，返回明确错误
//
// 设计要点（详见 docs/B2_INTEGRATION.md）：
//
//  1. 单方法 GetSnapshot：B2 的 lowstate 和 sportmodestate 是两个独立 DDS topic，
//     wrapper 内异步缓存最新值。如果暴露多个 GetXxx，调用方一轮采集会混用
//     不同时刻的缓存，导致指标时序错乱。GetSnapshot 一次性返回所有字段
//     + 每段时间戳 + 字段可用性（topic 未收到首包时对应字段为 nil）。
//
//  2. Health 暴露 DDS 自检：DDS 中途断连缓存不会"过期"，必须由 wrapper 守护线程
//     检测 last_seen 超龄 → 销毁重建 subscriber。Health 把这个状态对外暴露，
//     方便 collector 输出 b2_data_source_connected 等自检指标。
//
//  3. 字段只暴露 SDK 真实有的：BmsState IDL 里没有 health/is_charging/error_code，
//     原 stub 假装能读到这些字段是错的。本接口的 B2Battery 严格对齐 IDL。
package b2

import "time"

// B2DataSource 屏蔽 DDS / Mock / 未来 ROS2 等多种 B2 数据来源。
type B2DataSource interface {
	// Connect 建立到 B2 主控的连接。对 DDS 实现，这意味着初始化 ChannelFactory、
	// 订阅 topic、并等待至少一条 lowstate 首包（超时由 cfg.DDSConnectTimeout 控制）。
	Connect() error

	// Disconnect 断开连接但保留资源（可再次 Connect）。
	Disconnect() error

	// Close 释放所有资源（DDS 实现内会清理 ChannelFactory、停止守护线程）。
	// 调用 Close 后此 DataSource 实例不应再被使用。
	Close() error

	// IsConnected 返回当前连接状态（不发起新 IO，仅读内部 flag）。
	IsConnected() bool

	// GetSnapshot 返回一份 B2 状态快照。
	//
	// 返回的 Snapshot 中，LowState 或 SportState 字段可能为 nil，表示该 topic
	// 还没收到首包、或最近收到的包已超龄（被 wrapper 守护线程标记为 stale）。
	// 调用方对 nil 字段不应输出对应指标，避免发送过期或 NaN 数据。
	//
	// 此方法不应阻塞等待新消息——它只读 wrapper 内部的最新缓存。
	GetSnapshot() (*Snapshot, error)

	// Health 返回 DDS 自检状态（连接、收包延迟、重连/错误计数）。
	// collector 用这个生成 b2_data_source_connected、b2_dds_topic_last_seen_seconds 等指标。
	// Mock 实现可以返回总是健康的状态。
	Health() Health
}

// Snapshot 是一次采集得到的 B2 全部状态。
//
// 两个子状态来自不同 DDS topic、不同时刻：
//   - LowState 来自 rt/lowstate（默认；topic 名可配置），含 IMU、关节、电池、安全
//   - SportState 来自 rt/sportmodestate（默认），含机身速度、运动模式、步态
//
// 任一子状态为 nil 表示该 topic 还没收到首包或已 stale。
type Snapshot struct {
	LowState   *LowState
	SportState *SportState
}

// LowState 来自 B2 的 rt/lowstate topic（unitree_sdk2 的 LowState_ IDL）。
type LowState struct {
	IMU      IMU
	Joints   Joints
	Battery  Battery
	Safety   Safety
	Received time.Time // wrapper 收到此 LowState 消息的时刻（不是 SDK 自身的时间戳）
}

// SportState 来自 B2 的 rt/sportmodestate topic。
//
// 注：B2 的 sportmodestate topic 名在 unitree_sdk2 的 B2 示例中没有直接订阅过，
// 现场可能是 lf/sportmodestate 等变体。topic 名通过 config.B2CollectorConfig.SportStateTopic
// 配置，现场用 `ros2 topic list` 验证。
type SportState struct {
	// VelocityBody 机身坐标系下的线速度，单位 m/s。
	// 索引：0=x_forward, 1=y_left, 2=z_up
	VelocityBody [3]float64

	// ModeRaw 运动模式原码（具体语义查 SDK IDL；常见 0=idle, 5=stand, 6=walk）
	ModeRaw uint8

	// GaitTypeRaw 步态原码（0=idle, 1=trot, 2=run 等；具体查 IDL）
	GaitTypeRaw uint8

	Received time.Time
}

// IMU 来自 LowState.imu_state。
type IMU struct {
	Quaternion [4]float64 // w, x, y, z
	Gyroscope  [3]float64 // 角速度，rad/s，body frame
	Accel      [3]float64 // 线加速度，m/s², body frame
}

// Joints 12 个关节的状态。
//
// Unitree B2 是 12 关节四足，但 SDK 的 LowState.motor_state 数组通常长度 20（备用槽位）。
// 我们只取前 12 个。索引到关节的映射（按 unitree_sdk2 的 b2 例程约定，**首次部署需现场验证**）：
//
//	 0  FR_hip   1  FR_thigh   2  FR_calf
//	 3  FL_hip   4  FL_thigh   5  FL_calf
//	 6  RR_hip   7  RR_thigh   8  RR_calf
//	 9  RL_hip  10  RL_thigh  11  RL_calf
//
// 每个数组长度 12。
type Joints struct {
	Temperatures []float64 // °C
	Torques      []float64 // 估计扭矩，N·m（来自 motor_state.tau_est）
	Angles       []float64 // 关节位置，rad（来自 motor_state.q）
	Velocities   []float64 // 关节角速度，rad/s（来自 motor_state.dq）
	Modes        []uint8   // motor_state.mode 原码
	LostCounts   []uint32  // motor_state.lost 通信丢失计数
	StatusCodes  []uint32  // motor_state.reserve / status 原始字段（不解释，留给 dashboard）

	// FootForce / FootForceEst 来自 LowState.foot_force[4] / foot_force_est[4]
	// 索引顺序：0=FL, 1=FR, 2=RL, 3=RR（Unitree 约定，与 motor 索引顺序不同）
	FootForce    [4]int16
	FootForceEst [4]int16
}

// Battery 来自 LowState.bms_state（BmsState_ IDL，从 unitree_sdk2 v0.10.2 IDL 实测核对）。
//
// 严格对齐 SDK 真实字段（codex C-2 验证依据：unitree_sdk2/include/unitree/idl/go2/BmsState_.hpp）：
//   - SDK 不报 health_status / is_charging / error_code / total_voltage
//   - cell_vol 数组固定长度 15
//   - bq_ntc / mcu_ntc 是无符号 uint8（不是 int8）
//   - current 是 int32 mA（不是 int16）
//
// 总电压、单体最大/最小/压差由 collector 从 CellVoltages 派生计算。
type Battery struct {
	VersionHigh  uint8       // 固件版本高字节
	VersionLow   uint8       // 固件版本低字节
	SOC          uint8       // 电量百分比 0-100
	Current      int32       // 充放电电流，mA（带符号：放电为负）
	Cycle        uint16      // 充电循环次数
	BQNTC        [2]uint8    // BQ 芯片 NTC 温度，°C（无符号；负温度场景几乎不会触发）
	MCUNTC       [2]uint8    // BMS MCU 板 NTC 温度，°C
	CellVoltages [15]uint16  // 15 节电池单体电压，mV
	Status       uint8       // BMS 状态原码（IDL 中是 uint8 不是 uint32）
}

// Safety 来自 LowState 的若干安全相关 bit / 字段。
type Safety struct {
	EmergencyStop bool // 来自 SDK 的紧急停止位
	BMSError      bool // 任何 Battery.Status 非零都视为 error（保守判定）
}

// Health DDS 自检状态。
//
// Mock 实现可返回固定的"健康"值（DDSConnected=true，LastSeen=time.Now()）。
// DDS 实现中由守护线程更新这些字段，wrapper 检测 last_seen 超龄会重建 subscriber 并 ReconnectCount++。
type Health struct {
	DDSConnected       bool
	LowStateLastSeen   time.Time
	SportStateLastSeen time.Time
	ReconnectCount     uint64
	ErrorCount         uint64
	LastError          string
}

// jointLegMap 关节标签语义化：把 motor 索引（0-11）映射到 leg + joint。
//
// 顺序参照 unitree_sdk2/example/b2/b2_stand_example.cpp 的 _targetPos 数组（已现场核对）：
// 用 _targetPos_4 = {-0.5,1.36,-2.65,  0.5,1.36,-2.65,  -0.5,1.36,-2.65,  0.5,1.36,-2.65}
// 验证 hip 横滚符号：FR(idx 0)=-0.5, FL(idx 3)=+0.5——FR 朝右 FL 朝左，确认顺序 FR/FL/RR/RL。
//
// 关节命名沿用 SDK 原生术语（hip/thigh/calf），方便 dashboard 直接对照官方文档：
//   - hip   = 髋关节横滚（abduction，左右摆）
//   - thigh = 大腿（hip_pitch，前后摆）
//   - calf  = 小腿（knee）
var jointLegMap = [12][2]string{
	{"FR", "hip"}, {"FR", "thigh"}, {"FR", "calf"},
	{"FL", "hip"}, {"FL", "thigh"}, {"FL", "calf"},
	{"RR", "hip"}, {"RR", "thigh"}, {"RR", "calf"},
	{"RL", "hip"}, {"RL", "thigh"}, {"RL", "calf"},
}

// JointLabels 返回第 i 个关节（0..11）的 leg 和 joint 标签。索引越界返回空字符串。
func JointLabels(i int) (leg, joint string) {
	if i < 0 || i >= len(jointLegMap) {
		return "", ""
	}
	return jointLegMap[i][0], jointLegMap[i][1]
}

// FootForceLeg 返回 foot_force 数组第 i 个元素（0..3）对应的腿标签。
// Unitree 约定：0=FL, 1=FR, 2=RL, 3=RR。
func FootForceLeg(i int) string {
	switch i {
	case 0:
		return "FL"
	case 1:
		return "FR"
	case 2:
		return "RL"
	case 3:
		return "RR"
	default:
		return ""
	}
}

// JointCount B2 的关节数量。
const JointCount = 12

// FootCount B2 的足端数量。
const FootCount = 4

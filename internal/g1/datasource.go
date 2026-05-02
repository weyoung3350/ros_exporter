// Package g1 提供宇树 G1 人形机器人的数据源抽象。
//
// 与 internal/b2 包对称设计：interface + 多实现 + build tag 双隔离。
// 但 G1 在本仓库目前只用于电池监控（BMS collector），数据接口最简：仅 GetBatteryStatus。
//
// 实现：
//   - sdk_cgo.go：通过 internal/sdk/unitree/g1_sdk.cpp 编出的 libg1sdk.a 调真实 SDK（cgo 编译）
//   - mock.go：确定性模拟数据，与重构前 internal/types/g1_types_nocgo.go 输出数值完全一致
//   - sdk_stub.go：!cgo 构建时返回明确错误（与 mock 区分：mock 是主动选，stub 是降级失败）
//
// 重构兼容性约束：bms.go 中 G1 分支转用此包后，输出的 robot_battery_* 指标必须与
// 改造前完全一致（数值字段；timestamp 字段允许漂移，详见 plan v3 第 3.10 节）。
package g1

import "time"

// G1DataSource 屏蔽 SDK / Mock 等多种 G1 数据来源。
type G1DataSource interface {
	Connect() error
	Disconnect() error
	Close() error
	IsConnected() bool

	GetBatteryStatus() (*BatteryStatus, error)
}

// BatteryStatus G1 电池状态。
//
// 字段对齐 internal/sdk/unitree/g1_sdk.h 的 G1BatteryStatus C struct，保证 cgo 转换无字段丢失。
// 与重构前 types.G1BatteryStatus 字段完全一致（结构层不变，仅迁移位置），
// 确保 BMS collector 输出的 robot_battery_* 指标值一致。
type BatteryStatus struct {
	// 基础电池信息
	Voltage     float64 `json:"voltage"`     // 总电压 V
	Current     float64 `json:"current"`     // 电流 A
	Temperature float64 `json:"temperature"` // 平均温度 °C
	Capacity    float64 `json:"capacity"`    // 剩余容量 %
	CycleCount  uint32  `json:"cycle_count"` // 循环次数

	// 单体电压（40 节电池）
	CellVoltages []float64 `json:"cell_voltages"`

	// 温度传感器（12 个）
	Temperatures []float64 `json:"temperatures"`

	// 状态标志
	IsCharging    bool  `json:"is_charging"`
	IsDischarging bool  `json:"is_discharging"`
	HealthStatus  uint8 `json:"health_status"` // 0-100

	// 错误状态
	ErrorCode    uint32 `json:"error_code"`
	ErrorMessage string `json:"error_message"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

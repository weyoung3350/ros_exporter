# ros_exporter 适配宇树 B2（AGX-Thor 现场）实施方案

> **状态**：v3，合并 codex 评审反馈。3 Critical（编译闭环 / 电池字段 / DDS 自愈）+ 6 Warning（topic 配置 / GetSnapshot / 漏指标 / G1 约束 / 命名规范 / systemd）已纳入。评审报告归档：`/Users/dna/ros_exporter/docs/CODEX_REVIEW_REPORT.md`
> **创建日期**：2026-05-02
> **目标载体**：Jetson AGX Thor (ARM64 Linux) + 宇树 B2 工业级四足（编号 1401，BWT-B2-S1-00011）
> **关联文档**：`/Users/dna/Documents/Develop/claude_prj/AGX-Thor/AGX-Thor上狗集成方案.md`

---

## 1. Context（为什么做这件事）

### 1.1 背景

`ros_exporter` 是公司现有的统一 ROS/机器人指标 exporter，主动 push 到 VictoriaMetrics。仓库已经有 G1（真实 CGO 集成）、Go2、ROSMaster-X3 的多机器人骨架，**B2 也已有完整骨架但全部是 stub**——`internal/types/b2_types.go` 中所有 SDK 方法都是 `// TODO` + 硬编码假数据，运行起来 B2 指标全是恒定模拟值。

现场需求来自 AGX-Thor 上狗集成项目（参考文档第 1-7 行）：要把 ros_exporter 真实部署到 Thor + B2 1401 这套实物上，让 B2 关节温度、电池、急停、IMU 等关键指标真正可用，并保留对 G1/Go2/ROSMaster-X3 的兼容。

### 1.2 现状关键事实

通过仓库探索 + AGX-Thor 文档对照，确认：

1. `internal/collectors/b2.go`（343 行）：业务层完整，但持有具体 struct `*types.B2SDK`，无法替换实现
2. `internal/types/b2_types.go`（356 行）：`B2SDK.Initialize/Connect/GetMotionState/GetSensorState/GetJointState/GetSafetyState/GetBatteryStatus` 均为 stub
3. `internal/collectors/bms.go` 第 337-400 行：`connectB2/disconnectB2/readB2BMSData` 走的也是同一个 stub
4. `internal/config/config.go` 第 130-148 行：`B2CollectorConfig` 已定义，但缺少数据源类型字段
5. **现有指标设计有两类语义错误**（必须在重构中修正）：
   - `b2_sensor_status{lidar/camera/depth_camera}`：宇树 DDS 的 `LowState` **不报告**这几路独立设备的在线状态（雷达走以太网、相机走 USB，由 Thor 直接采集）
   - `terrain_type / collision_risk / stability_score / load_weight`：宇树 SDK **不直接给**，需要算法推算，stub 假装能读到
6. **平台差异**：现场是 ARM64 Jetson Thor，主网卡 `enP2p1s0`（不是 `eth0`），ROS2 Humble + Unitree DDS。当前 ros_exporter 的 `internal/ros/` 只有 `adapter_ros1.go`（ROS2 缺口本次不补，B2 走 DDS 不依赖 ROS2）

### 1.3 期望结果

- B2 关键指标（关节 12 路温度/扭矩/位置、电池、急停、错误码、IMU 姿态）通过 `unitree_sdk2` DDS 实时采集
- 数据源抽象层让未来加 ROS2 / 第三方机器人变成"新增实现"而非"改核心代码"
- macOS 开发机和无 SDK 的 CI 环境仍能 `go build` 成功（自动降级 mock）
- 一台 Thor 上 `bash deploy/install_b2_thor.sh` 完成 SDK 安装 + 编译 + systemd 启动
- G1/Go2/ROSMaster-X3 现有路径**零行为变化**

---

## 2. 已确认决策（来自 4 轮 brainstorm）

| # | 决策点 | 选择 | 理由 |
|---|-------|------|------|
| 1 | B2 数据源 | **DDS（CGO + unitree_sdk2）** | 直连 B2 主控最完整，对齐现有 G1 CGO 模式 |
| 2 | 抽象层 | **interface + build tag 双隔离** | interface 保扩展性，build tag 解决 CGO 在 macOS/CI 编译问题 |
| 3 | 第一版指标范围 | **核心子集**：IMU、关节 12 路（温度/扭矩/位置）、电池、急停/错误码 | 砍掉 SDK 不直接给的派生字段；关节标签语义化（leg=FL/FR/RL/RR, joint=hip_roll/hip_pitch/knee） |
| 4 | 部署形态 | **Thor 本机编译 + systemd** | 最简、CGO+DDS+ARM64 容器化有踩坑概率，与 OTA Agent 同构 |
| 5 | 关节角度单位 | **弧度 (rad)** | 与 unitree_sdk2 原生一致，避免反复转换；指标名 `b2_joint_angle_radians` |
| 6 | 配置告警阈值字段 | **删除** | `MaxJointTemp/MaxLoadWeight/MaxSpeed/CollisionRiskThreshold` 移交 VM/Grafana alert rules，避免双重定义 |
| 7 | G1 同步重构 | **做** | G1 现有 CGO 是真集成不是 stub，重构需保证零行为变化；架构对称 B2 让代码风格统一 |
| 8 | 接口形态 | **`GetSnapshot()` 单方法** | codex W-2：5 个 `GetXxx` 会让一轮采集混用不同时刻缓存；snapshot 含两段时间戳 + 字段可用性 bitmap |
| 9 | 电池字段 | **基于 `BmsState_` IDL 真实字段** | codex C-2：原 stub 假设的 health/is_charging/error 在 SDK 中**不存在**；总电压由 `sum(cell_vol)` 算 |
| 10 | DDS 自愈 | **per-topic last_seen + 超龄重建** | codex C-3：仅首包超时 + IsConnected 不够，旧缓存会一直推送；新增 `b2_dds_*` 自检指标 |
| 11 | topic 名 | **配置化** | codex W-1：`rt/sportmodestate` 在 B2 官方示例中没直接订过，可能是 `lf/sportmodestate` 等变体；现场用 `ros2 topic list` 确认后可覆盖默认值 |
| 12 | 指标命名 | **完整 Prometheus 单位后缀** | codex W-5：用 `meters_per_second` / `radians_per_second` / `newton_meters`，不缩写。一刀切替换旧名（B2 dashboard 之前是 stub 假数据，无外部依赖） |

附带：SDK 用 `unitree_sdk2`（非 `unitree_legged_sdk` 旧版）。

---

## 3. 设计

### 3.1 包结构（新增 `internal/b2/` 和 `internal/g1/`）

```
internal/
├── b2/                        ← 新增包：B2 数据源抽象 + 多实现
│   ├── datasource.go          B2DataSource interface + B2Snapshot 共享类型
│   ├── factory.go             New(cfg) (B2DataSource, error)，按 cfg.DataSource 选实现
│   ├── mock.go                MockDataSource（CI、macOS、显式 mock）
│   ├── dds_cgo.go             //go:build cgo && linux  ← Go 侧 cgo 入口
│   ├── dds_bridge.h           //go:build cgo && linux  ← C 接口声明（Go cgo 看到的）
│   ├── dds_bridge.cc          //go:build cgo && linux  ← C++ 实现，调 unitree_sdk2（与 .go 同目录才会被 cgo 自动编译；codex C-1）
│   └── dds_stub.go            //go:build !cgo || !linux  ← 返回明确错误
├── g1/                        ← 新增包：G1 同步重构（对称 B2 结构）
│   ├── datasource.go          G1DataSource interface（仅 GetBatteryStatus 一个数据方法）
│   ├── factory.go             New(cfg) (G1DataSource, error)，按 cfg.DataSource 选实现
│   ├── mock.go                MockDataSource（CI、macOS、显式 mock；从 g1_types_nocgo.go 迁移）
│   ├── sdk_cgo.go             //go:build cgo  ← 从 g1_types.go 迁移（C 接口已存在，无需重写 wrapper）
│   └── sdk_stub.go            //go:build !cgo  ← 返回明确错误（与 mock 区分：mock 主动选，stub 是降级）
├── collectors/
│   ├── b2.go                  改造：持有 b2.B2DataSource 而非 *types.B2SDK
│   └── bms.go                 改造：B2 分支改用 b2.B2DataSource；G1 分支改用 g1.G1DataSource
├── types/
│   ├── b2_types.go            精简：保留纯数据 struct（B2MotionState/B2JointState/B2BatteryStatus/B2SafetyState/B2IMUState 新增），删除 *B2SDK 类型和方法
│   └── g1_types.go            精简：保留 G1BatteryStatus 数据 struct + BatteryMetrics 计算工具，删除 *G1SDK 类型和 cgo 调用代码（迁到 internal/g1/）；删除 g1_types_nocgo.go（迁到 internal/g1/mock.go）
└── config/
    └── config.go              B2CollectorConfig 加 DataSource 字段、删告警阈值；BMSCollectorConfig 加 G1DataSource 字段（"sdk" | "mock"）

## 注：不再有 third_party/unitree_sdk2_b2.h
##     C 接口直接放在 internal/b2/dds_bridge.h，C++ 实现在 internal/b2/dds_bridge.cc
##     原因：Go cgo 只编译与 .go 文件同目录的 C/C++ 源（codex C-1）

deploy/
├── systemd/
│   └── ros_exporter.service   systemd unit 模板
└── install_b2_thor.sh         一键安装脚本（SDK 检测 → 编译 → 装服务）

config_b2.yaml                 B2 现场默认配置（新增）
docs/B2_INTEGRATION.md         B2 接入手册（新增）
```

### 3.2 接口契约（`internal/b2/datasource.go`）

**关键设计**（codex W-2）：用 `GetSnapshot()` 单方法替代 5 个 `GetXxx`。原因：

- DDS wrapper 内部缓存 `lowstate` 和 `sportmodestate` 两个独立 topic 的最新值
- 5 个 `GetXxx` 让 collector 一轮采集中混用**不同时刻**的缓存（IMU 来自 t=1.0s 的 lowstate，速度来自 t=2.5s 的 sportstate）
- snapshot 一次性返回所有字段 + **每字段对应的时间戳** + **字段可用性 flag**（topic 还没收到首包时不发对应指标）

```go
package b2

import "time"

// B2DataSource 屏蔽 DDS / Mock / 未来 ROS2 等多种数据来源
type B2DataSource interface {
    Connect() error
    Disconnect() error
    Close() error
    IsConnected() bool

    // GetSnapshot 一次性返回所有字段，含每段的时间戳和可用性
    GetSnapshot() (*B2Snapshot, error)

    // Health 暴露 DDS 自检状态（codex C-3）
    Health() B2Health
}

// B2Snapshot 一次采集的全部 B2 状态
type B2Snapshot struct {
    LowState   *B2LowState   // nil 表示该 topic 还没收到首包或已超龄
    SportState *B2SportState // 同上
}

// B2LowState 来自 rt/lowstate（含 IMU、关节、电池）
type B2LowState struct {
    IMU      B2IMU
    Joints   B2Joints       // 20 个 motor，但实际只用前 12 个（B2 是 12 关节四足）
    Battery  B2Battery
    Safety   B2Safety
    Received time.Time      // wrapper 收到此 LowState 的时刻
}

// B2SportState 来自 rt/sportmodestate（含速度、步态等）
type B2SportState struct {
    VelocityBody [3]float64  // m/s, body frame: x_forward, y_left, z_up
    Mode         uint8       // SDK mode 原码（0=idle, 5=stand, 6=walk 等，具体查 IDL）
    GaitType     uint8       // SDK gait 原码
    Received     time.Time
}

type B2IMU struct {
    Quaternion [4]float64    // w, x, y, z
    Gyroscope  [3]float64    // rad/s
    Accel      [3]float64    // m/s^2
}

// B2Joints 关节状态。按 Unitree 官方索引：
//   0  FR_hip   1  FR_thigh   2  FR_calf
//   3  FL_hip   4  FL_thigh   5  FL_calf
//   6  RR_hip   7  RR_thigh   8  RR_calf
//   9  RL_hip  10  RL_thigh  11  RL_calf
// 注：实际索引顺序需现场用 unitree_sdk2 example/b2 的 b2_stand_example.cpp 确认（codex W-1 同款怀疑）
type B2Joints struct {
    Temperatures   []float64  // 长度 12，°C
    Torques        []float64  // 长度 12，N·m（来自 SDK MotorState.tau_est）
    Angles         []float64  // 长度 12，rad
    Velocities     []float64  // 长度 12，rad/s（dq）
    ModeRaw        []uint8    // 长度 12，电机 mode 原码（codex W-3）
    LostCount      []uint32   // 长度 12，通信丢失计数（codex W-3）
    StatusRaw      []uint32   // 长度 12，原始 status 字段（不解释，留给 dashboard 做条件判断）
    FootForce      [4]int16   // 足端力（FL/FR/RL/RR；codex W-3 关键工业指标）
    FootForceEst   [4]int16   // 估计足端力（codex W-3）
}

// B2Battery 来自 BmsState_，**只暴露 SDK 真实有的字段**（codex C-2）
type B2Battery struct {
    SOC          uint8      // 电量百分比 0-100
    Current      int16      // 充放电电流，mA（带符号）
    Cycle        uint16     // 循环次数
    NTC          [2]int8    // BMS 板上 NTC 温度（具体几路查 IDL；常见 2 路）
    BQNTC        [2]int8    // BQ 芯片 NTC（同上）
    CellVoltages []uint16   // 每节电池电压，mV（具体节数 SDK 侧给的是 15 节，但需现场确认 B2 的实际节数）
    Status       uint32     // BMS 原码（不解释，留给 dashboard）
    // 总电压由 sum(CellVoltages) 计算，不作为字段（codex C-2）
}

type B2Safety struct {
    EmergencyStop bool      // 来自 LowState bit 字段
    BMSError      bool      // 任何 status 非零都视为 error（保守判定）
}

// B2Health DDS 自检状态（codex C-3）
type B2Health struct {
    DDSConnected      bool
    LowStateLastSeen  time.Time          // 最近一次收到 lowstate 的时间
    SportStateLastSeen time.Time         // 同上 sportmodestate
    ReconnectCount    uint64             // 累计重建 subscriber 次数
    ErrorCount        uint64             // 累计 DDS 错误次数
    LastError         string             // 最近一次错误描述
}
```

### 3.3 工厂选择（`internal/b2/factory.go`）

```go
// New 按配置选实现；DataSource 为空时默认 mock（向后兼容旧 config.yaml）
func New(cfg *config.B2CollectorConfig) (B2DataSource, error) {
    src := cfg.DataSource
    if src == "" {
        src = "mock"
    }
    switch src {
    case "dds":
        return newDDSDataSource(cfg)  // CGO 环境用 dds_cgo.go，否则用 dds_stub.go
    case "mock":
        return newMockDataSource(cfg), nil
    default:
        return nil, fmt.Errorf("未知的 b2.data_source: %s（支持: dds, mock）", src)
    }
}
```

### 3.4 CGO 实现要点（`internal/b2/dds_cgo.go` + `dds_bridge.cc`）

```go
//go:build cgo && linux
// +build cgo,linux

package b2

/*
#cgo CXXFLAGS: -std=c++17 -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lunitree_sdk2 -ldl -lpthread
#include "dds_bridge.h"
*/
import "C"
```

要点：

1. **bridge 文件放包内**（codex C-1）：`dds_bridge.h` + `dds_bridge.cc` 与 `dds_cgo.go` 同在 `internal/b2/`。Go cgo 自动编译同目录 `.cc` 文件——这是 **Go cgo 的硬约束**，放在 `third_party/` 不会被编译。所有 `.cc/.h` 头部加 `#ifdef BUILD_CGO_LINUX` 等预处理，或文件名加 `_linux.cc` 后缀让 Go 工具链按平台选编（更干净）

2. **C++ → C 边界**：`dds_bridge.cc` 内部用 C++ 调 `unitree_sdk2`，对外只暴露 C ABI 函数（如 `b2_dds_init / b2_dds_get_snapshot / b2_dds_close`）。Go 不直接 cgo C++ 类

3. **DDS 初始化**：`unitree::robot::ChannelFactory::Instance()->Init(0, networkInterface)` + 订阅 `rt/lowstate` 和 `rt/sportmodestate`（topic 名通过 C 接口参数传入，**不在 .cc 里硬编码**——支持 codex W-1 的配置化）

4. **缓存与时间戳**（codex C-3 自愈核心）：
   - bridge 内为每个 topic 维护：`{ mutex, last_msg, last_seen_steady_clock, message_count, error_count }`
   - DDS 回调线程更新缓存 + 时间戳；Go 侧 `GetSnapshot()` 调 C 接口读取，加锁拷贝出去
   - **后台守护线程**（bridge 内启）每 1s 检查所有 topic 的 `now() - last_seen`：超 5s 视为断流 → 销毁并重建对应 subscriber → `reconnect_count++`
   - 重建期间 `Health()` 返回 `DDSConnected=false`，`b2_data_source_connected=0`

5. **首包等待**：`Connect()` 返回成功的判定 = 在 5s 超时内**至少**收到一条 `rt/lowstate`（核心 topic）。`rt/sportmodestate` 缺失不阻塞 connect，但对应 `B2Snapshot.SportState` 为 nil

6. **网卡名通过 `cfg.NetworkInterface` 传入**，AGX-Thor 现场默认 `enP2p1s0`

#### `dds_bridge.h` 接口骨架

```c
#ifdef __cplusplus
extern "C" {
#endif

// 返回值：0=成功，非 0=错误码
int  b2_dds_init(const char* iface, const char* low_state_topic, const char* sport_state_topic);
int  b2_dds_wait_first_packet(int timeout_ms);  // 等首包
int  b2_dds_get_snapshot(B2RawSnapshot* out);   // out 是 POD struct，无指针
int  b2_dds_get_health(B2RawHealth* out);
void b2_dds_close(void);
const char* b2_dds_last_error(void);

#ifdef __cplusplus
}
#endif
```

#### 编译/链接闭环（codex C-1）

| 输入 | 处理 |
|------|------|
| `internal/b2/dds_bridge.cc` | Go cgo 在 `cgo && linux` build tag 下自动 `g++ -c` 编译此文件，产物 `.o` 自动链接进最终 binary |
| `internal/b2/dds_cgo.go` 的 `#cgo LDFLAGS` | 显式链接 `-lunitree_sdk2 -ldl -lpthread`（unitree_sdk2 装在 `/usr/local/lib`） |
| `internal/b2/dds_bridge.h` | bridge 自己用，对外提供 C 接口 |
| `internal/b2/dds_stub.go` 内的 `//go:build !cgo \|\| !linux` | 排除 .cc 文件不参与 stub 构建（通过文件名后缀 `_linux.cc` 自动按平台选；GOOS 不是 linux 时 Go 工具链跳过） |

### 3.5 stub 实现（`internal/b2/dds_stub.go`）

```go
//go:build !cgo || !linux
// +build !cgo !linux

package b2

import "errors"

func newDDSDataSource(_ *config.B2CollectorConfig) (B2DataSource, error) {
    return nil, errors.New(
        "B2 DDS 数据源在当前构建中不可用（需要 cgo + linux）。" +
        "macOS 开发机请改 b2.data_source: mock；Thor 部署请用 build.sh build:b2-thor 编译")
}
```

效果：macOS 开发机 `go build` 成功（用 stub），但若配置选 `dds` 则启动时给明确错误，**不会假装在跑**。

### 3.6 配置变更（`internal/config/config.go`）

```go
type B2CollectorConfig struct {
    Enabled          bool          `yaml:"enabled"`
    DataSource       string        `yaml:"data_source"`        // "dds" | "mock"，空=mock
    RobotID          string        `yaml:"robot_id"`
    NetworkInterface string        `yaml:"network_interface"`  // 现场默认 enP2p1s0
    UpdateInterval   time.Duration `yaml:"update_interval"`

    // topic 配置（codex W-1：现场用 ros2 topic list 验证后可覆盖默认值）
    LowStateTopic   string `yaml:"low_state_topic"`     // 默认 "rt/lowstate"
    SportStateTopic string `yaml:"sport_state_topic"`   // 默认 "rt/sportmodestate"

    // DDS 自愈参数（codex C-3）
    DDSStaleThreshold  time.Duration `yaml:"dds_stale_threshold"`  // 默认 5s，超此时间视为断流
    DDSConnectTimeout  time.Duration `yaml:"dds_connect_timeout"`  // 默认 5s，首包等待超时
    DDSReconnectMinGap time.Duration `yaml:"dds_reconnect_min_gap"`// 默认 2s，重建 subscriber 最小间隔（避免抖动）

    MonitorMotion  bool `yaml:"monitor_motion"`
    MonitorIMU     bool `yaml:"monitor_imu"`
    MonitorJoints  bool `yaml:"monitor_joints"`
    MonitorBattery bool `yaml:"monitor_battery"`  // 是否在 b2 collector 中暴露电池详细指标（cell voltage min/max/diff 等）
    MonitorSafety  bool `yaml:"monitor_safety"`

    // 删除：SDKConfigPath（不再需要外部 SDK 配置文件，wrapper 内部初始化）
    // 删除：MonitorSensors（载荷传感器不属于 B2 SDK 范畴）
    // 删除：MaxJointTemp / MaxLoadWeight / MaxSpeed / CollisionRiskThreshold（移交 VM/Grafana alert rules）
}
```

`DefaultConfig()` 中 B2 默认值：`Enabled=false, DataSource="mock", NetworkInterface="enP2p1s0", LowStateTopic="rt/lowstate", SportStateTopic="rt/sportmodestate", DDSStaleThreshold=5s, DDSConnectTimeout=5s, DDSReconnectMinGap=2s`。

### 3.7 Collector 改造（`internal/collectors/b2.go`）

主要变化：

- `B2Collector` 持有 `b2.B2DataSource` 替代 `*types.B2SDK`
- `Collect()` 调一次 `GetSnapshot()`，按 snapshot 中字段可用性 + 配置开关选择性采集
- 字段缺失（snapshot.LowState / SportState 为 nil）时**不发对应指标**，避免发空值或 NaN
- **删除指标**：`b2_sensor_status{*}`、`b2_obstacle_detected`、`b2_slope_angle_degrees`、`b2_collision_risk_score`、`b2_stability_score`、`b2_load_weight_kg`、`b2_max_*_capability_*`、`b2_work_mode`/`b2_gait_mode`

**最终指标列表**（命名遵循 Prometheus 单位后缀完整规范，codex W-5）：

| 指标名 | 标签 | 来源 SDK 字段 | 说明 |
|--------|-----|--------------|------|
| **关节** | | | |
| `b2_joint_temperature_celsius` | `leg, joint, joint_id` | LowState.motor_state[i].temperature | 关节温度 |
| `b2_joint_torque_newton_meters` | `leg, joint, joint_id` | LowState.motor_state[i].tau_est | 估计扭矩 |
| `b2_joint_position_radians` | `leg, joint, joint_id` | LowState.motor_state[i].q | 关节角度（rad，SDK 原生单位）|
| `b2_joint_velocity_radians_per_second` | `leg, joint, joint_id` | LowState.motor_state[i].dq | 关节角速度 |
| `b2_joint_mode` | `leg, joint, joint_id` | LowState.motor_state[i].mode | 电机 mode 原码（codex W-3）|
| `b2_joint_lost_total` | `leg, joint, joint_id` | LowState.motor_state[i].lost | 通信丢失累计（codex W-3）|
| `b2_joint_status` | `leg, joint, joint_id` | LowState.motor_state[i].reserve/status | 原始状态码 |
| **足端力**（codex W-3） | | | |
| `b2_foot_force` | `leg=FL/FR/RL/RR` | LowState.foot_force[i] | 足端力（无量纲传感器原码） |
| `b2_foot_force_estimate` | `leg=FL/FR/RL/RR` | LowState.foot_force_est[i] | 估计足端力 |
| **IMU** | | | |
| `b2_imu_quaternion` | `axis=w/x/y/z` | LowState.imu_state.quaternion | 姿态四元数 |
| `b2_imu_angular_velocity_radians_per_second` | `axis=x/y/z` | LowState.imu_state.gyroscope | 角速度 |
| `b2_imu_linear_acceleration_meters_per_second_squared` | `axis=x/y/z` | LowState.imu_state.accelerometer | 加速度 |
| **运动**（来自 sportmodestate） | | | |
| `b2_body_velocity_meters_per_second` | `axis=x/y/z` | SportState.velocity[i] | 机身速度（body frame） |
| `b2_sport_mode` | — | SportState.mode | 运动模式原码 |
| `b2_sport_gait` | — | SportState.gait_type | 步态原码 |
| **电池**（codex C-2，**只暴露 SDK 真实字段**） | | | |
| `b2_battery_soc_percent` | — | BmsState.soc | 电量百分比 |
| `b2_battery_current_milliamperes` | — | BmsState.current | 充放电电流（带符号）|
| `b2_battery_cycle_count` | — | BmsState.cycle | 充电循环 |
| `b2_battery_ntc_temperature_celsius` | `sensor=ntc0/ntc1/bq0/bq1` | BmsState.ntc / bq_ntc | BMS 板上温度（具体几路现场确认） |
| `b2_battery_cell_voltage_millivolts` | `cell_id=0..N` | BmsState.cell_vol[i] | 每节电压；N 由 SDK 实际给的数组长度决定 |
| `b2_battery_cell_voltage_min_millivolts` | — | min(cell_vol) | 衍生：最低单体（电池均衡度） |
| `b2_battery_cell_voltage_max_millivolts` | — | max(cell_vol) | 衍生：最高单体 |
| `b2_battery_cell_voltage_diff_millivolts` | — | max-min | 衍生：单体压差（>50mV 通常是均衡警告） |
| `b2_battery_total_voltage_millivolts` | — | sum(cell_vol) | 衍生：总电压（SDK 不直接给，由 cell_vol 求和；codex C-2）|
| `b2_battery_status` | — | BmsState.status | 原码（不解释）|
| **安全** | | | |
| `b2_emergency_stop` | — | LowState 紧急停止 bit | 0/1 |
| **DDS 自检**（codex C-3） | | | |
| `b2_data_source_connected` | `data_source=dds/mock` | Health.DDSConnected | 0/1 |
| `b2_dds_topic_last_seen_seconds` | `topic=lowstate/sportmodestate` | now - LastSeen | 距上次收包秒数 |
| `b2_dds_reconnect_total` | — | Health.ReconnectCount | 累计重连次数 |
| `b2_dds_error_total` | — | Health.ErrorCount | 累计 DDS 错误 |

**关节标签语义**：
- `leg` 取值：`FL/FR/RL/RR`
- `joint` 取值：`hip_roll/hip_pitch/knee`（按 SDK 关节顺序映射，**首次部署需现场用 b2_stand_example.cpp 确认顺序**——codex 评审依据）
- `joint_id` 取值：`0..11`（保留原始索引，便于现场快速定位）

**电池数据职责划分**：
- `bms.go` 收 `robot_battery_*` 系列（跨机器人统一指标，B2/G1/Go2 共用），从 snapshot.LowState.Battery 取
- `b2.go` 收 `b2_battery_*` 系列（B2 特有的细节，如 cell voltage 数组、NTC、status 原码）。**两者数据源同一个 snapshot**，不会双重读取 SDK

### 3.8 BMS Collector 改造（`internal/collectors/bms.go`）

- `UnitreeSDKInterface` 持有 `b2.B2DataSource` 和 `g1.G1DataSource`（按 robot_type 分派）
- `connectB2/disconnectB2/readB2BMSData` 改为薄转发到 `b2.B2DataSource.GetSnapshot().LowState.Battery`
- `connectG1/disconnectG1/readG1BMSData` 改为薄转发到 `g1.G1DataSource.GetBatteryStatus()`
- **Go2 / Mock / Serial / CAN 分支完全不动**

`BMSData` 字段映射（B2 路径）：

| BMSData 字段 | 来源（B2） |
|------------|-----------|
| Voltage    | `sum(BmsState.cell_vol) / 1000.0`（mV → V）|
| Current    | `BmsState.current / 1000.0`（mA → A，带符号）|
| SOC        | `BmsState.soc`（直接 0-100）|
| Temperature| `mean(BmsState.ntc + bq_ntc)`（多路温度求平均）|
| Power      | `Voltage * Current` |
| Cycles     | `BmsState.cycle` |
| Health     | **不填**（SDK 不报；保留 0 值，并文档化此约束——codex C-2）|

### 3.9 构建与部署

#### `build.sh` 增加 target

```bash
build:b2-thor    # 在 Thor 本机或同架构机器上：CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -tags 'cgo' -o ros_exporter
build:dev        # 现有逻辑，不变（mock 数据源，CGO_ENABLED 默认）
```

构建前检查 `/usr/local/include/unitree/` 是否存在 unitree_sdk2 头文件，缺失则给出"先跑 install_b2_thor.sh 装 SDK"的提示。

#### `deploy/install_b2_thor.sh`（在 Thor 上执行）

1. 检测平台：必须 `uname -m == aarch64` && Ubuntu/Debian
2. 检查 unitree_sdk2 是否已装；未装则 `git clone https://github.com/unitreerobotics/unitree_sdk2 && cd unitree_sdk2 && mkdir build && cd build && cmake .. && sudo make install`
3. `bash build.sh build:b2-thor` 出二进制
4. 二进制装到 `/usr/local/bin/ros_exporter`
5. 配置文件装到 `/etc/ros_exporter/config.yaml`（从 `config_b2.yaml` 模板复制，若已存在则不覆盖）
6. systemd unit 装到 `/etc/systemd/system/ros_exporter.service`
7. `systemctl daemon-reload && systemctl enable --now ros_exporter`
8. 输出验证命令：`systemctl status ros_exporter && journalctl -u ros_exporter -f`

#### `deploy/systemd/ros_exporter.service`（codex W-6 修订）

要点：
- `User=ros_exporter`（**不再用 root**）：创建专用 system user
- `AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN`：DDS 多播只需要 raw socket，不需要 root
- `ProtectSystem=strict`、`PrivateTmp=true`、`NoNewPrivileges=true`：标准 systemd hardening
- `ReadOnlyPaths=/proc /sys`：让 system collector 仍能读
- `Restart=on-failure, RestartSec=5s`
- `Environment=CYCLONEDDS_URI=file:///etc/ros_exporter/cyclonedds.xml`（CycloneDDS 配置可现场注入，模板参考 `/Users/dna/Documents/Develop/claude_prj/AGX-Thor/new-thor-artifacts/templates/cyclonedds.xml`）
- `ExecStart=/usr/local/bin/ros_exporter -config /etc/ros_exporter/config.yaml`

**关于 DDS 多播的真实先决条件**（codex W-6）：`User=root` 不是充分条件。真正需要的是：
1. 网卡 `enP2p1s0` **multicast 已启用**（`ip link show enP2p1s0` 看 `MULTICAST` flag）
2. 路由表里有 `224.0.0.0/4` 出口指向该网卡
3. 防火墙不阻塞 UDP 多播端口（CycloneDDS 默认 7400+）
4. CycloneDDS XML 配置中 `<NetworkInterfaceAddress>` 指向该网卡

`install_b2_thor.sh` 在装 service 前**先做 4 项检查**，全部通过才注册 service；否则给具体修复命令（如 `sudo ip link set enP2p1s0 multicast on`）。

#### `config_b2.yaml`（现场模板）

关键默认：
```yaml
collectors:
  system:
    network:
      interfaces: ["enP2p1s0"]
  bms:
    interface_type: "unitree_sdk"
    robot_type: "b2"
    network_interface: "enP2p1s0"
  b2:
    enabled: true
    data_source: "dds"
    robot_id: "b2-1401"
    network_interface: "enP2p1s0"
    monitor_motion: true
    monitor_imu: true
    monitor_joints: true
    monitor_battery: true
    monitor_safety: true
```

### 3.10 G1 同步重构（对称 B2，零行为变化）

#### 现状

- `g1_sdk.h` **已经是 C 接口**（非 C++），无需新写 wrapper 层
- `internal/types/g1_types.go`（cgo 版）+ `g1_types_nocgo.go`（nocgo mock 版）双文件 build tag
- 仅暴露 1 个数据方法 `GetBatteryStatus`（G1 在本仓库只做电池监控，不像 B2 还要关节/IMU）

#### 重构动作

| 原文件 | 目标 |
|-------|------|
| `internal/types/g1_types.go` (cgo)   | 数据 struct + `BatteryMetrics.ToMetrics()` 工具函数留在原位置 |
| `internal/types/g1_types.go` (cgo)   | `G1SDK` 类型 + cgo 调用代码 → 迁到 `internal/g1/sdk_cgo.go` |
| `internal/types/g1_types_nocgo.go`  | `G1SDK` 模拟实现 → 迁到 `internal/g1/mock.go`（重命名为 `MockDataSource`） |
| `internal/collectors/bms.go`         | `connectG1/disconnectG1/readG1BMSData` 改用 `g1.G1DataSource`（薄转发）|

`g1_sdk.h` 文件**不动**，CGO flags（`-L. -lg1sdk -lstdc++ -lm`）也不动。

#### 接口契约（`internal/g1/datasource.go`）

```go
type G1DataSource interface {
    Connect() error
    Disconnect() error
    Close() error
    IsConnected() bool
    GetBatteryStatus() (*types.G1BatteryStatus, error)
}
```

只有一个数据方法，对称简单。`types.G1BatteryStatus` 仍是数据契约（不带方法），`BatteryMetrics.ToMetrics()` 计算工具留在 types 包。

#### 配置变更

`BMSCollectorConfig` 加：
```go
G1DataSource string `yaml:"g1_data_source"` // "sdk" | "mock"，空=按构建模式自动选（cgo→sdk, !cgo→mock）
```

向后兼容：旧 config.yaml 不写此字段时，行为与改造前完全一致（cgo 编译跑真 SDK，nocgo 编译跑 mock）。

#### 行为零变化验证（codex W-4 调整）

回归测试要点（详见第 4.3 节）：
- 同一台装 G1 SDK 的机器，改造前后跑 30 分钟：`robot_battery_*` 数值字段（voltage/current/soc/temperature/power/cycles/health）的序列**完全一致**（差值 < 1e-9）
- **不强求 timestamp bit-for-bit 一致**——`g1_types_nocgo.go` 现有 mock 用 `time.Now()`，每次跑都不同。改造后 mock 也用 `time.Now()`，timestamp 字段必然漂移
- mock 数据的**数值部分**（除 timestamp 外的所有字段）必须与改造前完全一致（用同一组确定性公式生成）

### 3.11 兼容性矩阵

| 场景 | 编译命令 | 默认数据源 | 行为 |
|------|---------|-----------|------|
| macOS 开发机 | `go build` | mock | 全套指标用模拟数据，可本地调 dashboard |
| Linux x86 + 无 SDK（CI） | `go build` | mock | 同上，用于跑单测 |
| Linux ARM64 Thor + 已装 unitree_sdk2 | `bash build.sh build:b2-thor` | dds | 真实数据 |
| 误把 macOS 二进制 + dds config 跑起来 | — | dds | 启动报错"DDS 数据源在当前构建中不可用"，**不会静默跑 mock 假装在工作** |

---

## 4. 测试与验证

### 4.1 单元测试（CI 可跑）

- `internal/b2/mock_test.go`：mock 实现满足 interface 契约；`GetSnapshot()` 返回值字段范围合理
- `internal/b2/factory_test.go`：`New(DataSource="dds")` 在 stub 构建中返回明确错误，错误信息含"cgo + linux"提示
- `internal/b2/mock_health_test.go`：mock 模拟 DDS 断流/重连场景，验证 `Health()` 返回值（codex C-3）
- `internal/g1/mock_test.go`：mock 输出的**数值字段**与改造前 `g1_types_nocgo.go` 完全一致（除 timestamp，codex W-4）
- `internal/collectors/b2_test.go`：
  - 用 mock 验证 `Collect()` 输出的指标数量、标签、值
  - 验证关节 leg/joint 标签映射（joint_id=3 → leg=FL,joint=hip_roll；按 SDK 顺序）
  - 验证 cell voltage 衍生指标（min/max/diff/total）计算正确
  - 验证 snapshot.SportState 为 nil 时**不输出** `b2_body_velocity_*` 指标
- `internal/collectors/bms_test.go`：B2/G1 分支分别用 mock 验证 `robot_battery_*` 指标输出
- `internal/config/config_test.go`：旧 config.yaml（无 `data_source` / `g1_data_source` / `low_state_topic` 等新字段）能正常加载并使用默认值

### 4.2 现场联调（Thor 上）

```bash
# 1. 装 SDK + 编译 + 起服务
bash deploy/install_b2_thor.sh

# 2. 验证服务
systemctl status ros_exporter
journalctl -u ros_exporter -f --since "1 min ago"

# 3. 查 HTTP metrics 端点（exporter 默认 127.0.0.1:9100）
curl -s http://127.0.0.1:9100/metrics | grep ^b2_

# 4. 验证关键指标存在（命名规范见 v3）
#    b2_joint_temperature_celsius{leg="FL",joint="hip_roll",joint_id="3"} 42.3
#    b2_joint_position_radians{leg="FL",joint="knee",joint_id="5"} -1.42
#    b2_joint_torque_newton_meters{leg="FR",joint="hip_pitch",joint_id="1"} 12.5
#    b2_imu_quaternion{axis="w"} 0.999
#    b2_imu_angular_velocity_radians_per_second{axis="z"} 0.001
#    b2_battery_soc_percent 73
#    b2_battery_cell_voltage_diff_millivolts 12   # 健康电池应 < 50mV
#    b2_emergency_stop 0
#    b2_data_source_connected{data_source="dds"} 1
#    b2_dds_topic_last_seen_seconds{topic="lowstate"} 0.05  # 健康应 < 1s

# 5. 验证 VictoriaMetrics 已收到
curl -G "$VM_URL/api/v1/query" --data-urlencode 'query=b2_joint_temperature_celsius'

# 6. 现场验证三件事（codex 评审依据）：
#    a) 触发急停 → b2_emergency_stop 变 1
#    b) 抬起 FL 腿 → b2_joint_torque_newton_meters{leg="FL"} 标签下的值变化（用于确认 leg 标签映射正确）
#    c) 拔掉网线 5s+ → b2_data_source_connected 变 0、b2_dds_reconnect_total 增加（验证自愈）
```

### 4.3 回归测试（G1 重构必须零行为变化）

- **G1 真机回归**：在装 G1 SDK 的旧机器上 checkout 改造前 commit → 跑 5 分钟收集 `robot_battery_*` 序列 → checkout 改造后 → 跑 5 分钟 → 用 promtool / VM query 对比两次序列，差值必须 < 1e-9
- **G1 mock 回归**：改造前 `go build -tags '!cgo'` 跑 1 分钟，记录 mock 数据；改造后同样跑，输出应**完全一致**（mock 数据是确定性的，不应漂移）
- ROSMaster-X3 测试机（如果有）跑一次 `test_rosmaster_x3.sh`，确认未受波及

---

## 5. 关键文件路径速查

| 路径 | 类型 | 说明 |
|------|------|------|
| `/Users/dna/ros_exporter/internal/collectors/b2.go` | 改造 | 持有 interface |
| `/Users/dna/ros_exporter/internal/collectors/bms.go` | 改造 | B2 分支转发到新接口 |
| `/Users/dna/ros_exporter/internal/types/b2_types.go` | 精简 | 删除 SDK 类型 |
| `/Users/dna/ros_exporter/internal/config/config.go` | 改造 | 加 DataSource 字段，删阈值字段 |
| `/Users/dna/ros_exporter/internal/b2/` | 新增包 | datasource/factory/mock/dds_cgo/dds_stub |
| `/Users/dna/ros_exporter/internal/g1/` | 新增包 | datasource/factory/mock/sdk_cgo/sdk_stub（从 types/g1_types* 迁移） |
| `/Users/dna/ros_exporter/internal/types/g1_types.go` | 精简 | 只保留 G1BatteryStatus 数据 struct + BatteryMetrics 工具 |
| `/Users/dna/ros_exporter/internal/types/g1_types_nocgo.go` | 删除 | mock 逻辑迁到 internal/g1/mock.go |
| `/Users/dna/ros_exporter/g1_sdk.h` | **不动** | C wrapper 已是 C 接口，原样复用 |
| `/Users/dna/ros_exporter/third_party/unitree_sdk2_b2.h` | 新增 | B2 的 C wrapper 头（unitree_sdk2 是 C++） |
| `/Users/dna/ros_exporter/deploy/systemd/ros_exporter.service` | 新增 | systemd unit |
| `/Users/dna/ros_exporter/deploy/install_b2_thor.sh` | 新增 | 一键安装 |
| `/Users/dna/ros_exporter/config_b2.yaml` | 新增 | 现场配置模板 |
| `/Users/dna/ros_exporter/build.sh` | 改造 | 加 build:b2-thor target |
| `/Users/dna/ros_exporter/docs/B2_INTEGRATION.md` | 新增 | 接入手册 |

参考来源：
- B2 关节顺序约定：unitree_sdk2 官方 README + `internal/types/b2_types.go` 第 263 行注释（"12个关节"）
- 网卡名 `enP2p1s0`：`/Users/dna/Documents/Develop/claude_prj/AGX-Thor/AGX-Thor上狗集成方案.md` 附录 A
- CycloneDDS 模板：`/Users/dna/Documents/Develop/claude_prj/AGX-Thor/new-thor-artifacts/templates/cyclonedds.xml`
- G1 CGO 集成参照：`/Users/dna/ros_exporter/g1_sdk.h` + `/Users/dna/ros_exporter/internal/types/g1_types.go`

---

## 6. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| unitree_sdk2 在 ARM64 Jetson 上 `cmake` 失败（依赖、glibc 版本） | 现场卡住 | install 脚本提前检测；预备文档化常见错误（缺 `libboost-all-dev` 等） |
| DDS 多播被 Thor 防火墙/网卡配置阻塞（codex W-6） | 启动后读不到数据 | install 脚本预检 4 项前置（multicast flag / 路由 / 防火墙 / CycloneDDS XML），不通过给具体修复命令；运行时 `b2_dds_topic_last_seen_seconds` 暴露实际收包延迟 |
| DDS 中途断连 + 缓存推旧值（codex C-3） | 指标看起来"正常"但其实是历史快照 | wrapper 守护线程 per-topic last_seen 检测，超 5s 销毁重建 subscriber，`b2_dds_reconnect_total` 暴露事件 |
| `rt/sportmodestate` topic 名在 B2 实际固件中可能不同（codex W-1） | 速度/步态指标永远缺失 | topic 名配置化，install 脚本提示现场跑 `ros2 topic list` 验证 |
| B2 关节顺序假设错误（codex W-3 同款怀疑） | leg/joint 标签贴错 | 首次部署在已知姿态（蹲下/站立）抬一条腿验证哪个 motor_state 索引对应的扭矩/温度变化；记入 `docs/B2_INTEGRATION.md` |
| BmsState IDL 字段（cell_vol 节数、ntc 路数）在不同固件版本不一致 | 数组越界或维度错乱 | 解析时按 SDK 实际数组长度处理（不假设 15 节或 2 路），不做硬编码 |
| B2 1401 固件版本与 unitree_sdk2 master 不兼容 | 字段缺失或语义错乱 | install 脚本固定 SDK 到已验证 commit；现场首次部署用 SN/版本 + commit hash 记入 `docs/对话记录.md` |
| 现有 B2 grafana dashboard 引用旧指标名 | 面板空白 | B2 之前是 stub 假数据，**没有真实在用的 dashboard**；用户已确认 W-5 选 A 一刀切。新指标命名规范文档化 |
| CGO 在 macOS 上误开导致编译失败 | 开发机阻塞 | build tag 严格 `cgo && linux`；macOS 即使 CGO_ENABLED=1 也会走 stub |
| `dds_bridge.cc` 放错位置不被 cgo 编译（codex C-1） | 编译表面成功但运行时找不到符号 | 强约束：`.cc/.h` 必须与 `dds_cgo.go` 同在 `internal/b2/`；写入 `docs/B2_INTEGRATION.md` 维护规约 |

---

## 7. 已决策项归档（用户 2026-05-02 拍板）

### 7.1 brainstorm 阶段决策

| 原待确认 | 决策 |
|---------|------|
| 关节单位 | ✅ 改用 rad |
| 删告警阈值字段 | ✅ 删除，移交 VM/Grafana |
| 同步重构 G1 | ✅ 本次顺手做，对称 B2 架构 |
| 旧指标命名兼容（W-5）| ✅ A 一刀切（B2 dashboard 之前是 stub 假数据，无外部依赖）|
| codex 反馈合并方式 | ✅ 一次性合并到 plan v3 |

### 7.2 codex 评审反馈处置（v3 合并完成）

| 反馈 | 处置 | 落地位置 |
|------|------|---------|
| C-1 编译闭环 | ✅ 必改 | 第 3.1 节包结构 + 第 3.4 节 |
| C-2 电池字段 | ✅ 必改 | 第 3.2 节 `B2Battery` + 第 3.7 节指标表 |
| C-3 DDS 自愈 | ✅ 必改 | 第 3.4 节守护线程 + 第 3.6 节配置 + 第 3.7 节自检指标 |
| W-1 topic 配置化 | ✅ 改 | 第 3.6 节 |
| W-2 GetSnapshot | ✅ 改 | 第 3.2 节接口 |
| W-3 漏指标 | ✅ 改 | 第 3.7 节补 foot_force / mode / lost / cell voltage min/max/diff |
| W-4 G1 mock 约束 | ✅ 改 | 第 3.10 节零行为变化验证 |
| W-5 命名规范 | ✅ 改（A 一刀切）| 第 3.7 节全表用 `_meters_per_second / _radians / _newton_meters` |
| W-6 systemd | ✅ 改 | 第 3.9 节用 `AmbientCapabilities` + 4 项 DDS 多播预检 |
| I-1 ROS2 备选 | ❌ 不改 | 已选 DDS，备选记入 docs |
| I-2 多 B2 部署 | ✅ 加入 docs | 第 8.1 节多机部署说明 |

### 7.3 剩余未决

- **Go2 stub**：本次**不做**（超出范围，G1+B2 已经够大）

---

## 8. 不在本次范围

- ROS2 适配器（B2 走 DDS 不需要，但未来如果接 ROS2-only 机器人需补 `internal/ros/adapter_ros2.go`）
- 载荷传感器（雷达/相机/深度相机）独立 collector：从 B2 collector 移出后，本次**不补**新的实现，留待独立任务
- Grafana dashboard 更新：删除指标对应的面板由用户/运维同步处理
- OTA 中枢的部署方案下发集成
- 容器化镜像（建议 A 跑稳后做 B 方案）

### 8.1 多台 B2 部署模式（codex I-2）

如果未来要监控多台 B2，**强烈建议每台 B2 一个独立 ros_exporter 实例**，不要单进程跑多机器人：

- Unitree `ChannelFactory::Instance()` 是**进程级单例**，无法多绑
- 同一进程订阅多台 B2 的 `rt/lowstate` 会导致消息互相覆盖（topic 名一样，无法按来源区分）
- 多台 B2 用 Prometheus `instance` label 区分（每台 Thor 上的 ros_exporter 用各自 hostname 作 instance）

`docs/B2_INTEGRATION.md` 中明确写出这个约束。

### 8.2 全栈本机化容器（用户 2026-05-02 提的未来阶段）

把**采集 + 存储 + 展示**三件套都跑在 Thor 本机，不依赖外部 VictoriaMetrics：

- **采集**：`ros_exporter`（本方案产出物）
- **存储**：VictoriaMetrics 单机版（轻量 ARM64 镜像）
- **展示**：Grafana

部署形态建议：
- `docker-compose.yml` 在 Thor 上 `up -d`，三个服务共一个 `network_mode: host`（DDS 多播 + 跨服务低延迟）
- VM 数据盘挂 `/var/lib/vm`（Thor SSD），保留滚动 N 天
- Grafana dashboard 提前备好（B2 关节温度、电池均衡、IMU 姿态、DDS 自检）
- ros_exporter 容器需要：`network_mode: host`、挂载 `/proc /sys`（系统指标）、挂 `/usr/local/lib/libunitree_sdk2.so`（DDS 库）或镜像里直接打进 SDK

**先后顺序**：本次先把 ros_exporter 在 Thor 本机系统级 systemd 跑通，验证 DDS 数据可读、推到外部 VM。容器化阶段再做（约束更复杂：DDS 容器化有踩坑概率，且 VM/Grafana 镜像选 ARM64 版本要逐个验证）。

容器化方案具体设计写到独立 plan，不在本次。

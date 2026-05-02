# Codex 独立技术评审报告

> **评审任务 ID**：task-moo0jrmk-epoa3h
> **评审日期**：2026-05-02
> **评审模型**：Codex (medium effort)
> **被评审方案**：`/Users/dna/.claude/plans/codex-fluffy-pillow.md` v2

---

## Critical（不改方案会出错）

### C-1. C wrapper 编译/链接闭环不成立
**问题**：`third_party/*.cpp` 放在 `internal/b2/` 包外面，**Go 的 cgo 不会自动编进**。Go 约定 `.c/.cpp/.cc` 必须和 `.go` 在同一包目录才会被 cgo 编译。

**改法**：
- **方案 A**：把 bridge `.cc/.cpp` 直接放到 `internal/b2/` 包内（受 `//go:build cgo && linux` 控制）
- **方案 B**：先把 wrapper 编成静态库（`libunitree_b2_bridge.a`），装到 `/usr/local/lib`，再用 `#cgo LDFLAGS: -lunitree_b2_bridge` 显式链接

### C-2. B2 电池字段假设错了
**问题**：plan 中 `B2BatteryStatus` 字段（`HealthStatus / IsCharging / ErrorCode / 总电压`）来自原 stub 的猜测。实际 `LowState.BmsState` IDL **只有** `soc / current / cycle / ntc / cell_vol`，**没有** health、is_charging、error_code、总电压字段。

**改法**：
- 总电压由 `sum(cell_vol)` 计算，不是 SDK 直接给的字段
- 删除 `HealthStatus / IsCharging / ErrorCode`
- 暴露 `status` 原始码（具体语义查 IDL），拿不到的字段不生成指标
- 新增 `cell_voltages []float64` 字段（每节单体电压）

### C-3. DDS 断流自愈不闭环
**问题**：plan 只提到首包超时 + `IsConnected`，但 wrapper 内缓存可能**长期推旧值**——DDS 中断后数据不会"过期"，会一直返回上次缓存。

**改法**：
- wrapper 内每个 topic 记录 `last_seen` 时间戳
- 超龄（如 > 5s）置 `disconnected`，触发**重建 subscriber**
- 暴露指标：`b2_dds_topic_last_seen_seconds{topic}`、`b2_dds_reconnect_total`、`b2_dds_error_total`

---

## Warning（建议改但不阻塞）

### W-1. topic 名要做成配置
`rt/lowstate` 大概率正确；`rt/sportmodestate` 在 B2 官方示例中**没直接订阅过**，ROS2 文档存在 `lf/sportmodestate` / `sportmodestate` 差异。

**改法**：`B2CollectorConfig` 加 `low_state_topic / sport_state_topic` 字段（默认 `rt/lowstate` / `rt/sportmodestate`），现场用 `ros2 topic list` 或 DDS 抓包确认后可覆盖。

### W-2. interface 改 GetSnapshot
**问题**：5 个 `GetXxx` 让同一轮采集混用不同时间点的缓存数据（IMU 来自 t=1s 的 lowstate，速度来自 t=2s 的 sportstate）。

**改法**：interface 改单方法 `GetSnapshot() (*B2Snapshot, error)`，含两个时间戳 + 字段可用性 bitmap。

### W-3. 工业 B2 关键指标遗漏
- `foot_force` / `foot_force_est`（足端力，工业巡检关键）
- 电机 `mode` / `lost`（电机模式 / 通信丢失计数）
- BMS cell voltage min/max/diff（电池均衡度）
- `joint_error_code` 字段在 SDK 中实际叫什么？应改成真实字段名（lost、status）

**改法**：plan 第 3.7 节指标列表补齐这几路；joint 指标用真实字段名。

### W-4. G1 "bit-for-bit" 回归不可达
**问题**：现有 `g1_types_nocgo.go` mock 用 `time.Now()` 做 timestamp，每次跑都不同。

**改法**：把回归约束放宽为"**除 Timestamp 外字段值完全一致**"；或注入 clock interface 让测试用确定性时钟。

### W-5. 指标命名规范 + dashboard 兼容
Prometheus 推荐完整单位后缀：`meters_per_second / radians_per_second / newton_meters`，不要简写 `mps / rad_per_sec / nm`。

**改法**：
- 指标重命名（`b2_joint_velocity_radians_per_second`、`b2_joint_torque_newton_meters` 等）
- 关节单位从度改 rad 会让**旧 dashboard 空面板**——需要迁移说明文档；或考虑**双发一期**（旧名保留 + 新名并发，过渡期后下线）

### W-6. systemd 配置说明不严谨
**问题**：plan 写"`User=root` 因为 DDS 多播需要"——错。`User=root` 不是 DDS 多播的**充分条件**。

**改法**：
- 真正关键的是网卡 multicast 开启、路由、防火墙、CycloneDDS interface 配置
- `User=root` 用法改为"读 `/proc /sys` 系统指标需要"，DDS 部分单独讲网络配置
- systemd unit 加 `AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW` 比直接 root 更优

---

## Info（学习价值）

### I-1. ROS2 直连备选
现场已有 `robot_control` 容器跑 unitree_ros2 包装层时，订 ROS2 topic 可能更省事。但 Go 侧 ROS2 依赖（rclgo / rosgo）实际比 DDS+C wrapper **更重**——综合不一定更优。

### I-2. 多台 B2 部署模式
建议**每台 B2 一个独立 exporter 实例**，不要单进程跑多机器人。原因：Unitree `ChannelFactory` 是**单例**，且相同 topic 在单进程下无法区分来源。多机用 Prometheus instance label 区分。

---

## 评审依据

- Unitree SDK2 README
- `example/b2/b2_stand_example.cpp`
- `LowState_` / `BmsState_` IDL 定义
- Unitree ROS2 README

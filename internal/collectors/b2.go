package collectors

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"ros_exporter/internal/b2"
	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
)

// B2Collector 收集宇树 B2 四足机器人专有指标。
//
// 指标命名遵循 Prometheus 单位后缀完整规范（v3 codex W-5）：
// 速度用 _meters_per_second，扭矩用 _newton_meters，角度用 _radians 等，不缩写。
//
// 数据来源是 b2.B2DataSource，可以是 DDS 真实连接、mock 模拟，或将来其他实现。
// collector 不直接调 SDK，所有 IO 都通过 DataSource 抽象。
//
// 电池数据职责划分：
//   - bms.go 输出跨机器人统一的 robot_battery_* 指标（voltage / soc / temperature 等）
//   - 本 collector 输出 B2 特有的细节指标 b2_battery_*（cell voltage 数组、衍生 min/max/diff、NTC 等）
//
// 两者数据源同一份 snapshot，不会重复读取 SDK。
type B2Collector struct {
	config     *config.B2CollectorConfig
	instance   string
	dataSource b2.B2DataSource
	connected  bool
}

// NewB2Collector 创建 B2 collector。
//
// 内部按 cfg.DataSource 创建对应的 b2.B2DataSource 实现。
// 若 cfg.DataSource 不识别（如配置错），延迟到首次 Collect 才报错——
// 这样无效配置不会阻止整个 exporter 启动。
func NewB2Collector(cfg *config.B2CollectorConfig, instance string) *B2Collector {
	return &B2Collector{
		config:   cfg,
		instance: instance,
	}
}

// Collect 收集 B2 指标。
//
// 流程：
//  1. 若未启用直接返回
//  2. 若未连接，创建 DataSource 并连接（懒初始化，避免启动期就阻塞）
//  3. 调 GetSnapshot 取一份快照（含 LowState 和 SportState 两段独立 topic 的最新缓存）
//  4. 按 monitor_* 开关 + snapshot 字段可用性输出指标。LowState/SportState 为 nil 时跳过对应指标
//  5. 始终输出 b2_data_source_connected 和 b2_dds_* 自检指标
func (c *B2Collector) Collect(ctx context.Context) ([]client.Metric, error) {
	if !c.config.Enabled {
		return nil, nil
	}

	if c.dataSource == nil {
		ds, err := b2.New(c.config)
		if err != nil {
			return nil, fmt.Errorf("创建 B2 数据源失败: %w", err)
		}
		c.dataSource = ds
	}

	if !c.connected {
		if err := c.dataSource.Connect(); err != nil {
			return nil, fmt.Errorf("连接 B2 数据源失败: %w", err)
		}
		c.connected = true
	}

	snap, err := c.dataSource.GetSnapshot()
	if err != nil {
		return nil, fmt.Errorf("读取 B2 snapshot 失败: %w", err)
	}

	now := time.Now()
	baseLabels := map[string]string{
		"instance":    c.instance,
		"robot_type":  "b2",
		"robot_id":    c.config.RobotID,
		"data_source": c.dataSourceName(),
	}

	var metrics []client.Metric

	// LowState 来源指标：IMU、关节、电池、安全
	if snap != nil && snap.LowState != nil {
		ls := snap.LowState
		if c.config.MonitorIMU {
			metrics = append(metrics, c.imuMetrics(baseLabels, &ls.IMU, now)...)
		}
		if c.config.MonitorJoints {
			metrics = append(metrics, c.jointMetrics(baseLabels, &ls.Joints, now)...)
			metrics = append(metrics, c.footForceMetrics(baseLabels, &ls.Joints, now)...)
		}
		if c.config.MonitorBattery {
			metrics = append(metrics, c.batteryMetrics(baseLabels, &ls.Battery, now)...)
		}
		if c.config.MonitorSafety {
			metrics = append(metrics, c.safetyMetrics(baseLabels, &ls.Safety, now)...)
		}
	}

	// SportState 来源指标：机身速度、运动模式、步态
	if snap != nil && snap.SportState != nil && c.config.MonitorMotion {
		metrics = append(metrics, c.motionMetrics(baseLabels, snap.SportState, now)...)
	}

	// 自检指标：始终输出（不受 monitor 开关控制，运维需要观测连接状态）
	metrics = append(metrics, c.healthMetrics(baseLabels, now)...)

	return metrics, nil
}

func (c *B2Collector) imuMetrics(base map[string]string, imu *b2.IMU, now time.Time) []client.Metric {
	axisQuat := []string{"w", "x", "y", "z"}
	axisXYZ := []string{"x", "y", "z"}

	out := make([]client.Metric, 0, 4+3+3)
	for i, a := range axisQuat {
		out = append(out, client.Metric{
			Name:      "b2_imu_quaternion",
			Value:     imu.Quaternion[i],
			Labels:    addLabel(base, "axis", a),
			Timestamp: now,
		})
	}
	for i, a := range axisXYZ {
		out = append(out, client.Metric{
			Name:      "b2_imu_angular_velocity_radians_per_second",
			Value:     imu.Gyroscope[i],
			Labels:    addLabel(base, "axis", a),
			Timestamp: now,
		})
		out = append(out, client.Metric{
			Name:      "b2_imu_linear_acceleration_meters_per_second_squared",
			Value:     imu.Accel[i],
			Labels:    addLabel(base, "axis", a),
			Timestamp: now,
		})
	}
	return out
}

func (c *B2Collector) jointMetrics(base map[string]string, j *b2.Joints, now time.Time) []client.Metric {
	out := make([]client.Metric, 0, b2.JointCount*7)
	for i := 0; i < b2.JointCount; i++ {
		leg, joint := b2.JointLabels(i)
		labels := jointLabels(base, leg, joint, i)

		if i < len(j.Temperatures) {
			out = append(out, client.Metric{Name: "b2_joint_temperature_celsius", Value: j.Temperatures[i], Labels: labels, Timestamp: now})
		}
		if i < len(j.Torques) {
			out = append(out, client.Metric{Name: "b2_joint_torque_newton_meters", Value: j.Torques[i], Labels: labels, Timestamp: now})
		}
		if i < len(j.Angles) {
			out = append(out, client.Metric{Name: "b2_joint_position_radians", Value: j.Angles[i], Labels: labels, Timestamp: now})
		}
		if i < len(j.Velocities) {
			out = append(out, client.Metric{Name: "b2_joint_velocity_radians_per_second", Value: j.Velocities[i], Labels: labels, Timestamp: now})
		}
		if i < len(j.Modes) {
			out = append(out, client.Metric{Name: "b2_joint_mode", Value: float64(j.Modes[i]), Labels: labels, Timestamp: now})
		}
		if i < len(j.LostCounts) {
			out = append(out, client.Metric{Name: "b2_joint_lost_total", Value: float64(j.LostCounts[i]), Labels: labels, Timestamp: now})
		}
		if i < len(j.StatusCodes) {
			out = append(out, client.Metric{Name: "b2_joint_status", Value: float64(j.StatusCodes[i]), Labels: labels, Timestamp: now})
		}
	}
	return out
}

func (c *B2Collector) footForceMetrics(base map[string]string, j *b2.Joints, now time.Time) []client.Metric {
	out := make([]client.Metric, 0, b2.FootCount*2)
	for i := 0; i < b2.FootCount; i++ {
		leg := b2.FootForceLeg(i)
		labels := addLabel(base, "leg", leg)
		out = append(out, client.Metric{Name: "b2_foot_force", Value: float64(j.FootForce[i]), Labels: labels, Timestamp: now})
		out = append(out, client.Metric{Name: "b2_foot_force_estimate", Value: float64(j.FootForceEst[i]), Labels: labels, Timestamp: now})
	}
	return out
}

// batteryMetrics 输出 B2 特有的细节电池指标（与跨机器人统一的 robot_battery_* 指标互补）。
//
// 字段对齐 unitree_go::msg::dds_::BmsState_ IDL（codex C-2 现场验证依据）。
func (c *B2Collector) batteryMetrics(base map[string]string, bat *b2.Battery, now time.Time) []client.Metric {
	out := []client.Metric{
		{Name: "b2_battery_soc_percent", Value: float64(bat.SOC), Labels: base, Timestamp: now},
		{Name: "b2_battery_current_milliamperes", Value: float64(bat.Current), Labels: base, Timestamp: now},
		{Name: "b2_battery_cycle_count", Value: float64(bat.Cycle), Labels: base, Timestamp: now},
		{Name: "b2_battery_status", Value: float64(bat.Status), Labels: base, Timestamp: now},
		// 固件版本作为 info 指标（值固定 1，版本通过标签暴露）便于 join 查询时识别版本不一致
		{Name: "b2_battery_firmware_info", Value: 1,
			Labels:    mergeLabels(base, map[string]string{"version": fmt.Sprintf("%d.%d", bat.VersionHigh, bat.VersionLow)}),
			Timestamp: now},
	}

	// NTC 温度：BQ 芯片 + MCU 板各 2 路（命名沿用 SDK 原字段名）
	for i, t := range bat.BQNTC {
		out = append(out, client.Metric{
			Name: "b2_battery_ntc_temperature_celsius", Value: float64(t),
			Labels: addLabel(base, "sensor", fmt.Sprintf("bq%d", i)), Timestamp: now,
		})
	}
	for i, t := range bat.MCUNTC {
		out = append(out, client.Metric{
			Name: "b2_battery_ntc_temperature_celsius", Value: float64(t),
			Labels: addLabel(base, "sensor", fmt.Sprintf("mcu%d", i)), Timestamp: now,
		})
	}

	// 单体电压数组 + 衍生 min/max/diff/total
	//
	// 现场观察（B2 1401 实测）：cell_vol[14]=0 是 IDL 预留槽位，B2 实际有效节数 ≤14。
	// 衍生计算必须跳过 0 值，否则 min=0 / diff=max 是错误信号（看起来像电池故障）。
	// 所有原始 cell_voltage 仍然按数组索引输出，让 dashboard 能直接看到哪些槽位是预留。
	if len(bat.CellVoltages) > 0 {
		var sum uint32
		var min, max uint32
		minSet := false
		for i, v := range bat.CellVoltages {
			out = append(out, client.Metric{
				Name: "b2_battery_cell_voltage_millivolts", Value: float64(v),
				Labels: addLabel(base, "cell_id", strconv.Itoa(i)), Timestamp: now,
			})
			vu := uint32(v)
			sum += vu // total 包括预留槽位的 0 值——总电压用 sum(实际有数据的) 也对
			if vu == 0 {
				continue
			}
			if !minSet || vu < min {
				min = vu
				minSet = true
			}
			if vu > max {
				max = vu
			}
		}
		if minSet {
			out = append(out,
				client.Metric{Name: "b2_battery_cell_voltage_min_millivolts", Value: float64(min), Labels: base, Timestamp: now},
				client.Metric{Name: "b2_battery_cell_voltage_max_millivolts", Value: float64(max), Labels: base, Timestamp: now},
				client.Metric{Name: "b2_battery_cell_voltage_diff_millivolts", Value: float64(max - min), Labels: base, Timestamp: now},
			)
		}
		out = append(out,
			client.Metric{Name: "b2_battery_total_voltage_millivolts", Value: float64(sum), Labels: base, Timestamp: now},
		)
	}
	return out
}

func (c *B2Collector) safetyMetrics(base map[string]string, s *b2.Safety, now time.Time) []client.Metric {
	return []client.Metric{
		{Name: "b2_emergency_stop", Value: boolToFloat(s.EmergencyStop), Labels: base, Timestamp: now},
	}
}

func (c *B2Collector) motionMetrics(base map[string]string, sp *b2.SportState, now time.Time) []client.Metric {
	axes := []string{"x", "y", "z"}
	out := make([]client.Metric, 0, 5)
	for i, a := range axes {
		out = append(out, client.Metric{
			Name: "b2_body_velocity_meters_per_second", Value: sp.VelocityBody[i],
			Labels: addLabel(base, "axis", a), Timestamp: now,
		})
	}
	out = append(out,
		client.Metric{Name: "b2_sport_mode", Value: float64(sp.ModeRaw), Labels: base, Timestamp: now},
		client.Metric{Name: "b2_sport_gait", Value: float64(sp.GaitTypeRaw), Labels: base, Timestamp: now},
	)
	return out
}

// healthMetrics 暴露 DataSource 自检状态。无论 LowState/SportState 是否可用都输出，
// 让运维能直接看到 DDS 是否健康。
func (c *B2Collector) healthMetrics(base map[string]string, now time.Time) []client.Metric {
	h := c.dataSource.Health()

	out := []client.Metric{
		{Name: "b2_data_source_connected", Value: boolToFloat(h.DDSConnected), Labels: base, Timestamp: now},
		{Name: "b2_dds_reconnect_total", Value: float64(h.ReconnectCount), Labels: base, Timestamp: now},
		{Name: "b2_dds_error_total", Value: float64(h.ErrorCount), Labels: base, Timestamp: now},
	}

	// last_seen 距今秒数；若 LastSeen 是零值（尚未收到任何包），直接发负数（-1）让 dashboard 区分"从未收到"
	out = append(out,
		client.Metric{
			Name:      "b2_dds_topic_last_seen_seconds",
			Value:     ageSecondsOrSentinel(h.LowStateLastSeen, now),
			Labels:    addLabel(base, "topic", "lowstate"),
			Timestamp: now,
		},
		client.Metric{
			Name:      "b2_dds_topic_last_seen_seconds",
			Value:     ageSecondsOrSentinel(h.SportStateLastSeen, now),
			Labels:    addLabel(base, "topic", "sportmodestate"),
			Timestamp: now,
		},
	)
	return out
}

// Close 关闭 collector，释放数据源资源。
func (c *B2Collector) Close() error {
	if c.dataSource == nil {
		return nil
	}
	defer func() {
		c.connected = false
		c.dataSource = nil
	}()
	return c.dataSource.Close()
}

// dataSourceName 返回当前数据源类型名（用作 metric label）。
func (c *B2Collector) dataSourceName() string {
	if c.config.DataSource == "" {
		return "mock"
	}
	return c.config.DataSource
}

// jointLabels 构造 leg/joint/joint_id 三个标签合并到 base，统一关节指标的标签 schema。
func jointLabels(base map[string]string, leg, joint string, jointID int) map[string]string {
	out := make(map[string]string, len(base)+3)
	for k, v := range base {
		out[k] = v
	}
	out["leg"] = leg
	out["joint"] = joint
	out["joint_id"] = strconv.Itoa(jointID)
	return out
}

// ageSecondsOrSentinel 返回 t 距 now 的秒数；若 t 为零值返回 -1（"从未收到"哨兵）。
func ageSecondsOrSentinel(t, now time.Time) float64 {
	if t.IsZero() {
		return -1
	}
	return now.Sub(t).Seconds()
}

// boolToFloat 将布尔值转换为 0/1。
// 注：b2.go 也是 bms.go 等其他 collector 共用的 helper，定义在这里供整个包使用。
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// addLabel 在 labels 上叠加一对 key/value，返回新 map（不修改原 map）。
func addLabel(labels map[string]string, key, value string) map[string]string {
	out := make(map[string]string, len(labels)+1)
	for k, v := range labels {
		out[k] = v
	}
	out[key] = value
	return out
}

// mergeLabels 把 extra 合并到 base，返回新 map。用于一次叠加多个标签。
func mergeLabels(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

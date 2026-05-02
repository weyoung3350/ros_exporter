package collectors

import (
	"context"
	"strings"
	"testing"

	"ros_exporter/internal/client"
	"ros_exporter/internal/config"
)

// TestB2CollectorDisabled 启用为 false 时不输出任何指标。
func TestB2CollectorDisabled(t *testing.T) {
	c := NewB2Collector(&config.B2CollectorConfig{Enabled: false}, "test-host")
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("disabled 时不应报错: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("disabled 时不应输出指标，实际输出 %d 个", len(metrics))
	}
}

// TestB2CollectorMockEnabled 验证 mock 数据源 + 全部 monitor 开关时输出指标的命名规范、
// 标签 schema、衍生指标（cell voltage min/max/diff/total）计算正确。
//
// 这是 codex W-5（Prometheus 命名后缀）+ C-2（电池字段重设计）的回归测试。
func TestB2CollectorMockEnabled(t *testing.T) {
	cfg := &config.B2CollectorConfig{
		Enabled:        true,
		DataSource:     "mock",
		RobotID:        "b2-test",
		MonitorMotion:  true,
		MonitorIMU:     true,
		MonitorJoints:  true,
		MonitorBattery: true,
		MonitorSafety:  true,
	}
	c := NewB2Collector(cfg, "test-host")
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect 应成功: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("应输出指标")
	}

	// 收集所有指标名（不含标签）
	names := map[string]int{}
	for _, m := range metrics {
		names[m.Name]++
	}

	// 必须存在的指标（命名规范遵循 Prometheus 单位后缀完整规范）
	mustHave := []string{
		"b2_joint_temperature_celsius",
		"b2_joint_torque_newton_meters",
		"b2_joint_position_radians",
		"b2_joint_velocity_radians_per_second",
		"b2_joint_mode",
		"b2_joint_lost_total",
		"b2_imu_quaternion",
		"b2_imu_angular_velocity_radians_per_second",
		"b2_imu_linear_acceleration_meters_per_second_squared",
		"b2_battery_soc_percent",
		"b2_battery_current_milliamperes",
		"b2_battery_cell_voltage_millivolts",
		"b2_battery_cell_voltage_min_millivolts",
		"b2_battery_cell_voltage_max_millivolts",
		"b2_battery_cell_voltage_diff_millivolts",
		"b2_battery_total_voltage_millivolts",
		"b2_emergency_stop",
		"b2_body_velocity_meters_per_second",
		"b2_foot_force",
		"b2_foot_force_estimate",
		"b2_data_source_connected",
		"b2_dds_topic_last_seen_seconds",
		"b2_dds_reconnect_total",
	}
	for _, name := range mustHave {
		if names[name] == 0 {
			t.Errorf("缺失关键指标: %s", name)
		}
	}

	// 不应再有 v2 之前的旧名（codex W-5 一刀切）
	mustNotHave := []string{
		"b2_joint_torque_nm",
		"b2_joint_angle_radians",      // 改名为 b2_joint_position_radians
		"b2_joint_velocity_rad_per_sec",
		"b2_imu_gyroscope_rad_per_sec",
		"b2_imu_acceleration_mps2",
		"b2_current_speed_mps",
		"b2_sensor_status",            // 载荷传感器误置（codex W-3）
		"b2_collision_risk_score",     // SDK 不报
		"b2_stability_score",          // 同上
	}
	for _, name := range mustNotHave {
		if names[name] > 0 {
			t.Errorf("不应再输出旧名指标: %s", name)
		}
	}

	// 关节指标每路 12 个标签组合
	if names["b2_joint_temperature_celsius"] != 12 {
		t.Errorf("b2_joint_temperature_celsius 应输出 12 路（每个关节），实际 %d", names["b2_joint_temperature_celsius"])
	}
	// 足端力每路 4 个
	if names["b2_foot_force"] != 4 {
		t.Errorf("b2_foot_force 应输出 4 路（每只脚），实际 %d", names["b2_foot_force"])
	}
}

// TestB2CollectorJointLabels 验证关节指标的 leg/joint/joint_id 标签映射符合 SDK 顺序。
func TestB2CollectorJointLabels(t *testing.T) {
	cfg := &config.B2CollectorConfig{
		Enabled:       true,
		DataSource:    "mock",
		MonitorJoints: true,
	}
	c := NewB2Collector(cfg, "h")
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 找一个关节指标，按 joint_id 验证 leg/joint
	// 命名沿用 SDK 原生术语（hip/thigh/calf）。详见 internal/b2/datasource.go jointLegMap 注释。
	expectations := map[string]struct{ leg, joint string }{
		"0":  {"FR", "hip"},
		"3":  {"FL", "hip"},
		"5":  {"FL", "calf"},
		"11": {"RL", "calf"},
	}

	for _, m := range metrics {
		if m.Name != "b2_joint_temperature_celsius" {
			continue
		}
		jid := m.Labels["joint_id"]
		expect, ok := expectations[jid]
		if !ok {
			continue
		}
		if m.Labels["leg"] != expect.leg || m.Labels["joint"] != expect.joint {
			t.Errorf("joint_id=%s: 期望 leg=%s,joint=%s；实际 leg=%s,joint=%s",
				jid, expect.leg, expect.joint, m.Labels["leg"], m.Labels["joint"])
		}
	}
}

// TestB2CollectorCellVoltageDerivedMetrics 验证 cell voltage min/max/diff/total 衍生指标计算。
//
// mock 数据公式：cells[i] = 4000 + 2*i (i=0..14)，所以 min=4000, max=4028, diff=28, sum=60210
func TestB2CollectorCellVoltageDerivedMetrics(t *testing.T) {
	cfg := &config.B2CollectorConfig{
		Enabled:        true,
		DataSource:     "mock",
		MonitorBattery: true,
	}
	c := NewB2Collector(cfg, "h")
	metrics, _ := c.Collect(context.Background())

	getValue := func(name string) (float64, bool) {
		for _, m := range metrics {
			if m.Name == name {
				return m.Value, true
			}
		}
		return 0, false
	}

	min, ok := getValue("b2_battery_cell_voltage_min_millivolts")
	if !ok || min != 4000 {
		t.Errorf("min: ok=%v val=%v 期望 4000", ok, min)
	}
	max, ok := getValue("b2_battery_cell_voltage_max_millivolts")
	if !ok || max != 4028 {
		t.Errorf("max: ok=%v val=%v 期望 4028", ok, max)
	}
	diff, ok := getValue("b2_battery_cell_voltage_diff_millivolts")
	if !ok || diff != 28 {
		t.Errorf("diff: ok=%v val=%v 期望 28", ok, diff)
	}
	total, ok := getValue("b2_battery_total_voltage_millivolts")
	if !ok || total != 60210 {
		t.Errorf("total: ok=%v val=%v 期望 60210", ok, total)
	}
}

// TestB2CollectorDDSConfigError 配置非法的 data_source 时 Collect 应返回错误。
func TestB2CollectorDDSConfigError(t *testing.T) {
	cfg := &config.B2CollectorConfig{Enabled: true, DataSource: "ros2"}
	c := NewB2Collector(cfg, "h")
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("非法 data_source 应返回错误")
	}
}

// TestB2CollectorMonitorFlags 验证 monitor_* 开关能选择性输出指标。
// 只开 IMU，应有 IMU 指标，不应有 joint / battery 指标。
func TestB2CollectorMonitorFlags(t *testing.T) {
	cfg := &config.B2CollectorConfig{
		Enabled:    true,
		DataSource: "mock",
		MonitorIMU: true,
	}
	c := NewB2Collector(cfg, "h")
	metrics, _ := c.Collect(context.Background())

	hasPrefix := func(prefix string) bool {
		for _, m := range metrics {
			if strings.HasPrefix(m.Name, prefix) {
				return true
			}
		}
		return false
	}

	if !hasPrefix("b2_imu_") {
		t.Error("MonitorIMU=true 应输出 b2_imu_* 指标")
	}
	if hasPrefix("b2_joint_") {
		t.Error("MonitorJoints=false 不应输出 b2_joint_* 指标")
	}
	if hasPrefix("b2_battery_") {
		t.Error("MonitorBattery=false 不应输出 b2_battery_* 指标")
	}

	// 自检指标始终输出
	if !hasPrefix("b2_data_source_connected") {
		t.Error("自检指标应始终输出")
	}
}

// 编译时确认 B2Collector.Collect 返回 client.Metric 类型（防回归）
var _ = func() []client.Metric {
	c := NewB2Collector(&config.B2CollectorConfig{}, "")
	m, _ := c.Collect(context.Background())
	return m
}

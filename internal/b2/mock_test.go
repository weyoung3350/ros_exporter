package b2

import (
	"testing"

	"ros_exporter/internal/config"
)

// TestMockDataSourceLifecycle 验证 mock 数据源生命周期：
// 创建 → IsConnected=false → Connect → IsConnected=true → GetSnapshot 成功 → Close → IsConnected=false。
func TestMockDataSourceLifecycle(t *testing.T) {
	ds := newMockDataSource(&config.B2CollectorConfig{})

	if ds.IsConnected() {
		t.Fatal("未 Connect 前应 IsConnected=false")
	}

	if _, err := ds.GetSnapshot(); err == nil {
		t.Fatal("未 Connect 时 GetSnapshot 应报错")
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("Connect 应成功: %v", err)
	}
	if !ds.IsConnected() {
		t.Fatal("Connect 后应 IsConnected=true")
	}

	snap, err := ds.GetSnapshot()
	if err != nil {
		t.Fatalf("GetSnapshot 应成功: %v", err)
	}
	if snap == nil || snap.LowState == nil || snap.SportState == nil {
		t.Fatal("snapshot 的 LowState 和 SportState 都应非空")
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("Close 应成功: %v", err)
	}
	if ds.IsConnected() {
		t.Fatal("Close 后应 IsConnected=false")
	}
}

// TestMockSnapshotShape 验证 mock 数据各字段的形状（数组长度、范围）。
// 不验证具体数值（数值会随 mock 公式调整，但形状是 contract）。
func TestMockSnapshotShape(t *testing.T) {
	ds := newMockDataSource(&config.B2CollectorConfig{})
	if err := ds.Connect(); err != nil {
		t.Fatal(err)
	}
	snap, _ := ds.GetSnapshot()

	ls := snap.LowState

	// 关节数组长度都应是 12
	for name, arr := range map[string]int{
		"Temperatures": len(ls.Joints.Temperatures),
		"Torques":      len(ls.Joints.Torques),
		"Angles":       len(ls.Joints.Angles),
		"Velocities":   len(ls.Joints.Velocities),
		"Modes":        len(ls.Joints.Modes),
		"LostCounts":   len(ls.Joints.LostCounts),
		"StatusCodes":  len(ls.Joints.StatusCodes),
	} {
		if arr != JointCount {
			t.Errorf("%s 长度应为 %d，实际 %d", name, JointCount, arr)
		}
	}

	// 电池单体电压数组应非空，且单体压差合理（healthy battery <50mV）
	if len(ls.Battery.CellVoltages) == 0 {
		t.Fatal("CellVoltages 不应为空")
	}
	var min, max uint16 = ls.Battery.CellVoltages[0], ls.Battery.CellVoltages[0]
	for _, v := range ls.Battery.CellVoltages {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if max-min > 50 {
		t.Errorf("mock 健康电池单体压差应 < 50mV，实际 %d", max-min)
	}

	// 四元数 w 分量应非零（避免奇异姿态）
	if ls.IMU.Quaternion[0] == 0 {
		t.Error("Quaternion[w] 不应为 0")
	}

	// SOC 应在 0-100 范围
	if ls.Battery.SOC > 100 {
		t.Errorf("SOC 应 <= 100，实际 %d", ls.Battery.SOC)
	}
}

// TestJointLabelMapping 验证关节索引到 leg/joint 标签的映射。
//
// 这是 B2 现场联调最关键的 contract：抬起 FL 腿时，索引 3-5 的关节应观察到角度变化。
// 顺序按 Unitree 官方约定（FR, FL, RR, RL）。
func TestJointLabelMapping(t *testing.T) {
	// 命名沿用 SDK 原生术语：hip=横滚, thigh=大腿前后摆, calf=小腿（knee）
	cases := []struct {
		idx           int
		expectLeg     string
		expectJoint   string
	}{
		{0, "FR", "hip"},
		{1, "FR", "thigh"},
		{2, "FR", "calf"},
		{3, "FL", "hip"},
		{4, "FL", "thigh"},
		{5, "FL", "calf"},
		{6, "RR", "hip"},
		{9, "RL", "hip"},
		{11, "RL", "calf"},
	}
	for _, c := range cases {
		leg, joint := JointLabels(c.idx)
		if leg != c.expectLeg || joint != c.expectJoint {
			t.Errorf("idx=%d: 期望 leg=%s,joint=%s；实际 leg=%s,joint=%s",
				c.idx, c.expectLeg, c.expectJoint, leg, joint)
		}
	}

	// 越界返回空字符串
	if leg, joint := JointLabels(-1); leg != "" || joint != "" {
		t.Errorf("idx=-1 应返回空字符串，实际 leg=%q joint=%q", leg, joint)
	}
	if leg, joint := JointLabels(12); leg != "" || joint != "" {
		t.Errorf("idx=12 应返回空字符串（超过 11），实际 leg=%q joint=%q", leg, joint)
	}
}

// TestFootForceLegMapping 验证足端力数组索引到腿标签的映射（FL/FR/RL/RR）。
func TestFootForceLegMapping(t *testing.T) {
	cases := []struct {
		idx int
		leg string
	}{
		{0, "FL"}, {1, "FR"}, {2, "RL"}, {3, "RR"}, {4, ""}, {-1, ""},
	}
	for _, c := range cases {
		got := FootForceLeg(c.idx)
		if got != c.leg {
			t.Errorf("FootForceLeg(%d) 期望 %q，实际 %q", c.idx, c.leg, got)
		}
	}
}

// TestMockHealthIsHealthy 验证 mock 数据源汇报"健康"状态——便于 collector 单测断言。
func TestMockHealthIsHealthy(t *testing.T) {
	ds := newMockDataSource(&config.B2CollectorConfig{})
	_ = ds.Connect()
	h := ds.Health()
	if !h.DDSConnected {
		t.Fatal("mock Health 应 DDSConnected=true")
	}
	if h.LowStateLastSeen.IsZero() {
		t.Fatal("LowStateLastSeen 应非零")
	}
	if h.ReconnectCount != 0 || h.ErrorCount != 0 {
		t.Errorf("mock 应无错误/重连，实际 reconnect=%d error=%d", h.ReconnectCount, h.ErrorCount)
	}
}

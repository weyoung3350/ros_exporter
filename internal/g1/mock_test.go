package g1

import (
	"testing"

	"ros_exporter/internal/config"
)

// TestMockBatteryShapeAndValues 锁定 mock 数据的数值字段——
// 与改造前 internal/types/g1_types_nocgo.go 公式必须一致（plan v3 第 3.10 节零行为变化约束）。
//
// 时间戳允许漂移（codex W-4），不在断言范围。
func TestMockBatteryShapeAndValues(t *testing.T) {
	ds := newMockDataSource(&config.BMSCollectorConfig{})
	if err := ds.Connect(); err != nil {
		t.Fatal(err)
	}
	status, err := ds.GetBatteryStatus()
	if err != nil {
		t.Fatal(err)
	}

	// 数值字段（除 timestamp 外）必须与改造前一致
	wants := map[string]float64{
		"Voltage":     25.2,
		"Current":     -2.5,
		"Temperature": 35.0,
		"Capacity":    87.5,
	}
	gots := map[string]float64{
		"Voltage":     status.Voltage,
		"Current":     status.Current,
		"Temperature": status.Temperature,
		"Capacity":    status.Capacity,
	}
	for k, want := range wants {
		if gots[k] != want {
			t.Errorf("%s: 期望 %v，实际 %v（破坏了零行为变化约束）", k, want, gots[k])
		}
	}

	if status.CycleCount != 245 {
		t.Errorf("CycleCount 期望 245，实际 %d", status.CycleCount)
	}
	if status.HealthStatus != 92 {
		t.Errorf("HealthStatus 期望 92，实际 %d", status.HealthStatus)
	}
	if status.IsCharging != false || status.IsDischarging != true {
		t.Errorf("IsCharging/IsDischarging 期望 false/true，实际 %v/%v", status.IsCharging, status.IsDischarging)
	}
	if status.ErrorCode != 0 || status.ErrorMessage != "" {
		t.Errorf("无错误状态期望 0/\"\"，实际 %d/%q", status.ErrorCode, status.ErrorMessage)
	}

	// 单体电压数组：长度 40，公式 4.05 + (i%10)*0.01
	if len(status.CellVoltages) != 40 {
		t.Fatalf("CellVoltages 长度期望 40，实际 %d", len(status.CellVoltages))
	}
	for i := 0; i < 40; i++ {
		want := 4.05 + float64(i%10)*0.01
		if status.CellVoltages[i] != want {
			t.Errorf("CellVoltages[%d] 期望 %v，实际 %v", i, want, status.CellVoltages[i])
		}
	}

	// 温度数组：长度 12，公式 32.0 + (i%8)*0.5
	if len(status.Temperatures) != 12 {
		t.Fatalf("Temperatures 长度期望 12，实际 %d", len(status.Temperatures))
	}
	for i := 0; i < 12; i++ {
		want := 32.0 + float64(i%8)*0.5
		if status.Temperatures[i] != want {
			t.Errorf("Temperatures[%d] 期望 %v，实际 %v", i, want, status.Temperatures[i])
		}
	}
}

// TestMockNotConnected GetBatteryStatus 在未 Connect 时应报错。
func TestMockNotConnected(t *testing.T) {
	ds := newMockDataSource(&config.BMSCollectorConfig{})
	if _, err := ds.GetBatteryStatus(); err == nil {
		t.Fatal("未连接时 GetBatteryStatus 应报错")
	}
}

package collectors

import (
	"context"
	"testing"

	"ros_exporter/internal/config"
)

// TestBMSCollectorDisabled enabled=false 时不输出指标。
func TestBMSCollectorDisabled(t *testing.T) {
	c := NewBMSCollector(&config.BMSCollectorConfig{Enabled: false}, "h")
	metrics, _ := c.Collect(context.Background())
	if len(metrics) > 0 {
		t.Error("disabled 时不应输出")
	}
}

// TestBMSCollectorMockInterface InterfaceType 未指定时走 mock，输出 7 个 robot_battery_* 指标。
func TestBMSCollectorMockInterface(t *testing.T) {
	cfg := &config.BMSCollectorConfig{Enabled: true, InterfaceType: "mock"}
	c := NewBMSCollector(cfg, "h")
	metrics, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 应有 7 个 robot_battery_* 指标
	expected := []string{
		"robot_battery_voltage_volts",
		"robot_battery_current_amperes",
		"robot_battery_soc_percent",
		"robot_battery_temperature_celsius",
		"robot_battery_power_watts",
		"robot_battery_cycles_total",
		"robot_battery_health_percent",
	}
	got := map[string]bool{}
	for _, m := range metrics {
		got[m.Name] = true
	}
	for _, name := range expected {
		if !got[name] {
			t.Errorf("缺失 %s", name)
		}
	}

	// 标签 schema：instance + battery_id + interface
	if len(metrics) > 0 {
		labels := metrics[0].Labels
		for _, k := range []string{"instance", "battery_id", "interface"} {
			if _, ok := labels[k]; !ok {
				t.Errorf("标签缺失: %s", k)
			}
		}
	}
}

// TestB2BMSDataMapping 验证 B2 BmsState 字段到 BMSData 的映射规则（codex C-2）：
//   - Voltage 来自 sum(cell_voltages)/1000
//   - Current 来自 BmsState.current/1000（带符号）
//   - Health 保留 0（SDK 不报，不假装）
//   - Temperature 来自 NTC + BQNTC 求平均
func TestB2BMSDataMapping(t *testing.T) {
	// 这里直接用 averageTemperature 内部函数测平均逻辑
	// 完整的 B2 数据流（DDS → BmsState → BMSData）需要 mock 注入，留给现场联调阶段
	// 单测只验证最关键的转换数学正确

	// 通过 b2 mock 数据源验证整链路（BMS B2 分支默认 DataSource=dds，无法在 macOS 单测）
	// 替代：直接构造 b2.Battery 测 averageTemperature
	tests := []struct {
		name string
		bq   [2]uint8
		mcu  [2]uint8
		want float64
	}{
		{"全零", [2]uint8{0, 0}, [2]uint8{0, 0}, 0},
		{"两路 BQ", [2]uint8{30, 32}, [2]uint8{0, 0}, 31},
		{"四路全有效", [2]uint8{30, 32}, [2]uint8{34, 36}, 33},
		{"混合零和有效", [2]uint8{0, 32}, [2]uint8{34, 0}, 33}, // (32+34)/2
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bat := newB2BatteryForTest(tt.bq, tt.mcu)
			got := averageTemperature(bat)
			if got != tt.want {
				t.Errorf("averageTemperature(BQ=%v, MCU=%v) = %v, 期望 %v", tt.bq, tt.mcu, got, tt.want)
			}
		})
	}
}

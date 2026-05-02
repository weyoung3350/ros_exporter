package collectors

import (
	"ros_exporter/internal/b2"
)

// newB2BatteryForTest 构造一个 b2.Battery 用于单测。
//
// 字段类型对齐 BmsState_ IDL：bq_ntc / mcu_ntc 是 uint8。
func newB2BatteryForTest(bq, mcu [2]uint8) *b2.Battery {
	return &b2.Battery{
		BQNTC:  bq,
		MCUNTC: mcu,
	}
}

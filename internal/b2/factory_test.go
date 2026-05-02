package b2

import (
	"strings"
	"testing"

	"ros_exporter/internal/config"
)

// TestFactoryDefault 空字符串应默认走 mock（向后兼容旧 config.yaml）。
func TestFactoryDefault(t *testing.T) {
	ds, err := New(&config.B2CollectorConfig{DataSource: ""})
	if err != nil {
		t.Fatalf("空 DataSource 应默认 mock 不报错: %v", err)
	}
	if _, ok := ds.(*mockDataSource); !ok {
		t.Errorf("默认应是 mockDataSource，实际类型 %T", ds)
	}
}

// TestFactoryMock 显式 mock 创建成功。
func TestFactoryMock(t *testing.T) {
	ds, err := New(&config.B2CollectorConfig{DataSource: "mock"})
	if err != nil {
		t.Fatalf("mock 应创建成功: %v", err)
	}
	if ds == nil {
		t.Fatal("mock DataSource 不应为 nil")
	}
}

// TestFactoryDDSStubError 在当前骨架阶段（dds_cgo.go 未落地），所有平台 dds 都返回错误。
// 错误信息应包含"cgo"提示，让用户知道怎么修。
func TestFactoryDDSStubError(t *testing.T) {
	_, err := New(&config.B2CollectorConfig{DataSource: "dds"})
	if err == nil {
		t.Fatal("当前骨架阶段 dds 应返回错误（dds_cgo.go 未落地）")
	}
	if !strings.Contains(err.Error(), "cgo") {
		t.Errorf("错误信息应提示 cgo 要求，实际: %s", err.Error())
	}
}

// TestFactoryUnknown 未知数据源类型返回明确错误，不静默退化。
func TestFactoryUnknown(t *testing.T) {
	_, err := New(&config.B2CollectorConfig{DataSource: "ros2"})
	if err == nil {
		t.Fatal("未知 data_source 应返回错误")
	}
	if !strings.Contains(err.Error(), "ros2") {
		t.Errorf("错误信息应提及无效值，实际: %s", err.Error())
	}
}

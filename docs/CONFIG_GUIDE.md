# ros_exporter 配置指南

## 📋 概述

ros_exporter 现在使用统一的 `config.yaml` 配置文件，支持多种部署环境。不再需要维护多个配置文件，所有环境的配置都可以在一个文件中管理。

## 🚀 快速开始

### 1. 基础配置

默认的 `config.yaml` 已经包含了适合大多数环境的配置。你只需要根据实际部署环境调整以下关键配置：

```yaml
# 环境标识
victoria_metrics:
  extra_labels:
    environment: "production"  # 修改为你的环境类型

# 网络接口 
collectors:
  system:
    network:
      interfaces: []  # 空表示监控所有接口，或指定如 ["eth0", "wlan0"]
```

### 2. 环境特定配置

根据你的部署环境选择相应的配置：

#### 🖥️ 开发环境
```yaml
exporter:
  push_interval: 30s
  instance: "dev-laptop"

victoria_metrics:
  endpoint: "http://localhost:8428/api/v1/import/prometheus" # 本地开发示例
  extra_labels:
    environment: "development"

collectors:
  system:
    temperature:
      enabled: false  # 笔记本/虚拟机通常禁用
  bms:
    enabled: false    # 开发环境不需要电池监控
  ros:
    enabled: false    # 根据需要启用
```

#### 🧪 ROS2测试环境  
```yaml
exporter:
  push_interval: 15s
  instance: "ros2-test-vm"

victoria_metrics:
  extra_labels:
    environment: "testing"
    host: "<your_host>" # 请填写你的主机名或服务器IP

collectors:
  system:
    temperature:
      enabled: false  # 虚拟机环境
    network:
      interfaces: ["ens160"]  # 虚拟机网卡
  bms:
    enabled: false    # 测试环境不需要
  ros:
    enabled: true     # ROS环境必须启用
```

#### 🤖 机器人生产环境
```yaml
exporter:
  push_interval: 10s  # 高频率监控
  instance: "auto"    # 自动使用主机名

victoria_metrics:
  extra_labels:
    environment: "robot-production"
    robot_type: "unitree_go2"
    location: "field"

collectors:
  system:
    temperature:
      enabled: true   # 物理硬件温度监控重要
    network:
      interfaces: ["eth0", "wlan0"]  # 机器人网卡
  bms:
    enabled: true     # 电池监控核心功能
    interface_type: "unitree_sdk"
  ros:
    enabled: true     # 机器人控制系统
```

## ⚙️ 详细配置说明

### Exporter配置 (exporter)

| 参数 | 说明 | 推荐值 |
|------|------|--------|
| `push_interval` | 数据推送间隔 | 开发:30s, 测试:15s, 生产:10s |
| `instance` | 实例标识 | "auto"(自动主机名) 或自定义名称 |
| `log_level` | 日志级别 | "info" |

### VictoriaMetrics配置

| 参数 | 说明 | 示例 |
|------|------|------|
| `endpoint` | 数据推送端点 | 生产: `<your_endpoint>` | # 请填写你的 VictoriaMetrics/Prometheus Pushgateway 地址
| | | 开发: `http://localhost:8428/api/v1/import/prometheus` | # 本地开发示例
| `extra_labels.environment` | 环境标识 | "development", "testing", "robot-production" |

### 系统监控配置 (collectors.system)

| 功能 | 参数 | 物理机器人 | 虚拟机/容器 | 开发环境 |
|------|------|-----------|------------|----------|
| 温度监控 | `temperature.enabled` | ✅ true | ❌ false | ❌ false |
| 网络监控 | `network.interfaces` | ["eth0", "wlan0"] | ["ens160"] | [] (全部) |
| 带宽监控 | `network.bandwidth_enabled` | ✅ true | ✅ true | ❌ false |

### BMS电池监控配置 (collectors.bms)

| 参数 | 说明 | 推荐值 |
|------|------|--------|
| `enabled` | 是否启用 | 仅物理机器人启用 |
| `interface_type` | 接口类型 | "unitree_sdk" (推荐) |
| `robot_type` | 机器人类型 | "auto" (自动检测) |
| `update_interval` | 更新间隔 | 5s |

### ROS监控配置 (collectors.ros)

| 参数 | 说明 | 推荐值 |
|------|------|--------|
| `enabled` | 是否启用 | ROS环境必须启用 |
| `master_uri` | ROS Master地址 | "http://localhost:11311" |
| `scrape_interval` | 抓取间隔 | 5s |

## 🔧 常见配置场景

### 场景1: 多机器人集群监控

每个机器人使用不同的实例标识：

```yaml
exporter:
  instance: "robot-001"  # robot-002, robot-003...

victoria_metrics:
  extra_labels:
    robot_id: "001"
    location: "warehouse"
```

### 场景2: 本地开发调试

禁用不必要的监控，使用本地VictoriaMetrics：

```yaml
exporter:
  push_interval: 30s

victoria_metrics:
  endpoint: "http://localhost:8428/api/v1/import/prometheus"

collectors:
  bms:
    enabled: false
  ros:
    enabled: false
```
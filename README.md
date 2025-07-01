# ros_exporter

一个统一的ROS指标导出器，基于Go语言开发，支持系统指标、BMS电池管理和ROS监控，主动推送数据到VictoriaMetrics。

## 🎯 核心特性

- **🖥️ 系统监控**: CPU、内存、磁盘、网络等基础系统指标（基于Node Exporter逻辑）
- **🌡️ 温度监控**: CPU温度监控，支持sensors命令和thermal_zone文件
- **📊 带宽监控**: 实时网络带宽计算，支持指定网卡接口监控
- **🔋 BMS监控**: 支持宇树SDK、串口、CAN总线等多种接口的电池管理系统监控
- **🤖 ROS监控**: ROS节点状态、topic频率、参数服务器等指标
- **📊 主动推送**: 直接推送到VictoriaMetrics，解决动态IP和网络不稳定问题
- **⚙️ 高度可配置**: YAML配置文件，支持白名单/黑名单过滤
- **🔄 容错设计**: 自动重试、优雅退出、错误恢复

## 📁 项目结构

```
ros_exporter/
├── main.go                    # 主程序入口
├── go.mod                     # Go模块定义
├── config.yaml               # 统一配置文件
├── build.sh                  # 构建脚本
├── internal/                 # 核心代码
│   ├── config/               # 配置管理
│   ├── exporter/            # 核心导出器
│   ├── client/               # VictoriaMetrics客户端
│   └── collectors/           # 指标收集器
├── scripts/                  # 实用脚本
│   ├── clean-tmp.sh          # 临时文件清理
│   └── quick-clean.sh        # 快速清理
├── tmp/                      # 临时文件目录
│   ├── build/                # 构建临时文件
│   ├── cache/                # 缓存文件
│   ├── logs/                 # 日志文件
│   └── test/                 # 测试文件
└── docs/                     # 文档目录
    ├── README.md             # 项目文档
    └── CONFIG_GUIDE.md       # 配置指南
```

## 🚀 快速开始

### 1. 编译项目

```bash
# 使用构建脚本（推荐）
./build.sh build

# 或者手动编译
go build -o ros_exporter main.go

# 清理构建文件和临时文件
./build.sh clean
# 或快速清理
./scripts/quick-clean.sh
```

### 1.1 公司环境说明
  时序库 <your_vm_url>  <your_prometheus_url>  这里可以查看指标数据。 # 请填写你的时序数据库/Prometheus/Grafana 地址


### 2. 配置文件

ros_exporter 使用统一的 `config.yaml` 配置文件，支持多种部署环境。

**快速配置**: 首次运行会自动生成包含详细注释的默认配置文件。详细配置说明请参考 [配置指南](CONFIG_GUIDE.md)。

默认生成的 `config.yaml` 示例：

```yaml
exporter:
  push_interval: 15s
  instance: "your-hostname"
  log_level: "info"

victoria_metrics:
  endpoint: "http://localhost:8428/api/v1/import/prometheus"
  timeout: 30s
  extra_labels:
    job: "ros_exporter"
  retry:
    max_retries: 3
    retry_delay: 1s
    max_delay: 30s
    backoff_rate: 2.0

collectors:
  system:
    enabled: true
    collectors: ["cpu", "memory", "disk", "network", "load"]
    proc_path: "/proc"
    sys_path: "/sys"
    rootfs_path: "/"
    
    # 温度监控配置
    temperature:
      enabled: true
      sensors_cmd: "sensors"
      temp_source: "sensors"  # 可选: "sensors", "thermal_zone"
      thermal_zone: "/sys/class/thermal/thermal_zone0/temp"
    
    # 网络监控配置
    network:
      enabled: true
      interfaces: []  # 空表示监控所有接口，或指定: ["eth0", "wlan0"]
      bandwidth_enabled: true
      exclude_loopback: true
  
  bms:
    enabled: true
    interface_type: "unitree_sdk"  # 支持: unitree_sdk, serial, canbus
    device_path: "/dev/ttyUSB0"
    baud_rate: 115200
    can_interface: "can0"
  
  ros:
    enabled: true
    master_uri: "http://localhost:11311"
    topic_whitelist: []
    topic_blacklist: ["/rosout", "/rosout_agg"]
    node_whitelist: []
    node_blacklist: ["/rosout"]
    scrape_interval: 5s
```

### 3. 环境适配

根据部署环境调整关键配置：

```yaml
# 🖥️ 开发环境 - 禁用硬件相关监控
collectors:
  system:
    temperature:
      enabled: false
  bms:
    enabled: false

# 🧪 测试环境 - 指定网卡，禁用BMS
collectors:
  system:
    network:
      interfaces: ["ens160"]  # 虚拟机网卡
  bms:
    enabled: false

# 🤖 生产环境 - 启用所有监控
collectors:
  system:
    network:
      interfaces: ["eth0", "wlan0"]  # 机器人网卡
  bms:
    enabled: true
```


### 4. 运行导出器

```bash
# 使用默认配置
./ros_exporter

# 指定配置文件
./ros_exporter -config /path/to/config.yaml

# 指定监控的网络接口（类似原C++实现）
./ros_exporter -interfaces eth0,wlan0

# 查看版本
./ros_exporter -version
```

## 📊 指标说明

### 系统指标 (System Metrics)

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `node_cpu_seconds_total` | Counter | CPU时间统计（按模式分类） |
| `node_cpu_temperature_celsius` | Gauge | CPU温度 (°C) |
| `node_memory_*_bytes` | Gauge | 内存使用情况 |
| `node_disk_*_total` | Counter | 磁盘I/O统计 |
| `node_network_*_total` | Counter | 网络流量统计 |
| `node_network_bandwidth_up_mbps` | Gauge | 网络上行带宽 (Mbps) |
| `node_network_bandwidth_down_mbps` | Gauge | 网络下行带宽 (Mbps) |
| `node_load1/5/15` | Gauge | 系统负载 |

### BMS指标 (Battery Metrics)

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `robot_battery_voltage_volts` | Gauge | 电池电压 (V) |
| `robot_battery_current_amperes` | Gauge | 电池电流 (A) |
| `robot_battery_soc_percent` | Gauge | 电量百分比 (%) |
| `robot_battery_temperature_celsius` | Gauge | 电池温度 (°C) |
| `robot_battery_power_watts` | Gauge | 电池功率 (W) |
| `robot_battery_cycles_total` | Counter | 充电周期数 |
| `robot_battery_health_percent` | Gauge | 电池健康度 (%) |

### ROS指标 (ROS Metrics)

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `ros_nodes_total` | Gauge | ROS节点总数 |
| `ros_node_status` | Gauge | 节点状态 (1=运行, 0=停止) |
| `ros_topics_total` | Gauge | Topic总数 |
| `ros_topic_frequency_hz` | Gauge | Topic发布频率 (Hz) |
| `ros_topic_publishers_total` | Gauge | Publisher数量 |
| `ros_topic_subscribers_total` | Gauge | Subscriber数量 |
| `ros_parameters_total` | Gauge | 参数服务器参数数量 |
| `ros_master_status` | Gauge | ROS Master状态 |

### 导出器指标 (Exporter Metrics)

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `ros_exporter_up` | Gauge | 导出器运行状态 |
| `ros_exporter_collection_duration_seconds` | Gauge | 指标收集耗时 |
| `ros_exporter_last_collection_timestamp` | Gauge | 最后收集时间戳 |

## 🔧 高级配置

### BMS接口配置

#### 宇树SDK接口
```yaml
bms:
  enabled: true
  interface_type: "unitree_sdk"
```

#### 串口接口
```yaml
bms:
  enabled: true
  interface_type: "serial"
  device_path: "/dev/ttyUSB0"
  baud_rate: 115200
```

#### CAN总线接口
```yaml
bms:
  enabled: true
  interface_type: "canbus"
  can_interface: "can0"
```

### ROS过滤配置

```yaml
ros:
  # 只监控特定topic（如果为空则监控所有）
  topic_whitelist: ["/cmd_vel", "/robotstate"]
  
  # 排除特定topic
  topic_blacklist: ["/rosout", "/rosout_agg", "/tf"]
  
  # 只监控特定节点
  node_whitelist: ["/navigation", "/control"]
  
  # 排除特定节点
  node_blacklist: ["/rosout"]
```

## 🏗️ 架构设计

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   System        │    │   BMS            │    │   ROS           │
│   Collector     │    │   Collector      │    │   Collector     │
│                 │    │                  │    │                 │
│ • CPU/Memory    │    │ • Voltage/SOC    │    │ • Nodes/Topics  │
│ • Disk/Network  │    │ • Current/Temp   │    │ • Parameters    │
│ • Load Average  │    │ • Power/Health   │    │ • Master Status │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────────┐
                    │   Monitoring        │
                    │   Exporter          │
                    │                     │
                    │ • 指标聚合           │
                    │ • 时间戳管理         │
                    │ • 错误处理           │
                    └─────────────────────┘
                                 │
                    ┌─────────────────────┐
                    │   VictoriaMetrics   │
                    │   Push Client       │
                    │                     │
                    │ • Prometheus格式     │
                    │ • 重试机制           │
                    │ • 网络容错           │
                    └─────────────────────┘
                                 │
                    ┌─────────────────────┐
                    │   VictoriaMetrics   │
                    │   Server            │
                    │                     │
                    │ • 数据存储           │
                    │ • 查询接口           │
                    │ • Grafana集成        │
                    └─────────────────────┘
```

## 🐛 故障排除

### 常见问题

1. **VictoriaMetrics连接失败**
   ```
   检查endpoint配置是否正确
   确认VictoriaMetrics服务是否运行
   验证网络连接和防火墙设置
   ```

2. **BMS数据读取失败**
   ```
   检查interface_type配置
   验证设备路径或接口是否存在
   确认权限设置（串口/CAN设备）
   ```

3. **ROS监控无数据**
   ```
   检查ROS_MASTER_URI环境变量
   确认ROS Master是否运行
   验证节点和topic过滤配置
   ```

4. **CPU温度监控失败**
   ```
   安装lm-sensors: sudo apt-get install lm-sensors
   运行sensors-detect配置传感器
   或者使用thermal_zone模式: temp_source: "thermal_zone"
   ```

5. **网络带宽监控异常**
   ```
   检查网络接口名称是否正确
   确认/proc/net/dev文件可读
   验证指定的接口是否存在
   ```

### 日志分析

```bash
# 启动时查看详细日志
./ros_exporter -config config.yaml 2>&1 | tee tmp/logs/ros_exporter.log

# 查看特定错误
grep "ERROR\|WARN" tmp/logs/ros_exporter.log
```

## 🗂️ 临时文件管理

项目使用 `tmp/` 目录统一管理临时文件，保持项目根目录整洁。

### 目录结构
```
tmp/
├── build/      # 构建过程临时文件
├── cache/      # 缓存文件
├── logs/       # 日志文件
└── test/       # 测试临时文件
```

### 清理命令

```bash
# 快速清理（日常使用）
./scripts/quick-clean.sh

# 详细清理选项
./scripts/clean-tmp.sh --help

# 清理特定类型文件
./scripts/clean-tmp.sh build    # 只清理构建文件
./scripts/clean-tmp.sh logs     # 只清理日志文件
./scripts/clean-tmp.sh -f all   # 强制清理所有文件

# 查看临时文件大小
./scripts/clean-tmp.sh -s all
```

### 自动清理

构建脚本会自动使用 `tmp/` 目录：

```bash
./build.sh test     # 测试日志保存到 tmp/test/
./build.sh docker   # Docker构建使用 tmp/build/
./build.sh clean    # 自动清理临时文件
```

## 🤝 扩展开发

### 添加新的收集器

1. 在 `internal/collectors/` 下创建新文件
2. 实现 `Collector` 接口
3. 在 `exporter.go` 中注册新收集器
4. 更新配置结构

### 自定义指标格式

可以修改 `client/vm_client.go` 中的 `formatPrometheusText` 方法来自定义指标格式。

## 📝 许可证

MIT License

## 🙋‍♂️ 支持

如有问题或建议，请提交Issue或Pull Request。 
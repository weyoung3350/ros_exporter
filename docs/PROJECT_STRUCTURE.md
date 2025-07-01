# ros_exporter 项目功能分析

## 项目概述

ros_exporter是一个基于Go语言开发的企业级机器人监控解决方案，专为宇树机器人（支持G1和Go2）设计，提供全方位的系统监控、电池管理和ROS状态监控功能。

## 项目定位

- **独立项目**：完全独立的监控解决方案，不依赖其他项目
- **参考资料**：robot_control目录仅作为C++实现参考，不属于本项目
- **企业级设计**：生产就绪的监控导出器，支持大规模部署

## 核心功能架构

### 1. 系统监控模块 (System Collector)

**基础指标采集**：
- CPU使用率和多核心时间统计
- 内存使用详情（总量、可用、缓存等）
- 磁盘I/O统计和文件系统使用率
- 系统负载（1/5/15分钟平均值）

**温度监控**：
- 支持sensors命令获取CPU温度
- 支持thermal_zone文件读取
- 自动fallback机制

**网络监控**：
- 网络接口流量统计
- 实时带宽计算（上行/下行 Mbps）
- 支持指定接口监控
- 自动排除回环接口

### 2. BMS电池监控模块 (BMS Collector)

**多接口支持**：
- 宇树SDK接口（推荐）
- 串口通信接口
- CAN总线接口
- Mock接口（测试用）

**自动检测**：
- 机器人类型自动识别（G1/Go2）
- 基于主机名、系统文件、SDK响应检测
- 支持手动配置覆盖

**标准化指标**：
- 电池电压 (V)
- 电池电流 (A)
- 电量百分比 (SOC %)
- 电池温度 (°C)
- 功率计算 (W)
- 充电周期数
- 电池健康度 (%)

### 3. ROS监控模块 (ROS Collector)

**节点监控**：
- 节点运行状态
- 节点数量统计
- 黑白名单过滤
- 关键节点标记

**Topic监控**：
- 发布频率监控
- Publisher/Subscriber统计
- 消息计数和新鲜度
- 业务topic分类和标记

**特殊功能**：
- G1电池状态监控（通过/robotstate topic）
- ROS Master状态检查
- 参数服务器监控

### 4. 数据推送模块 (VM Client)

**推送机制**：
- 主动推送到VictoriaMetrics
- Prometheus文本格式
- 批量数据优化
- 时间戳精确控制

**容错设计**：
- 自动重试机制
- 指数退避算法
- 连接池管理
- 网络异常处理

## 技术架构

```
┌─────────────────────────────────────────────────────┐
│             ros_exporter                  │
├─────────────────────────────────────────────────────┤
│                  Configuration                      │
│              (YAML配置文件管理)                      │
├─────────────────────────────────────────────────────┤
│                   Collectors                        │
│  ┌─────────────┬──────────────┬─────────────┐     │
│  │   System    │     BMS      │     ROS     │     │
│  │  Collector  │  Collector   │  Collector  │     │
│  └─────────────┴──────────────┴─────────────┘     │
├─────────────────────────────────────────────────────┤
│                Exporter Core                       │
│         (调度、聚合、错误处理)                       │
├─────────────────────────────────────────────────────┤
│            VictoriaMetrics Client                   │
│          (数据格式化、批量推送)                      │
└─────────────────────────────────────────────────────┘
                           │
                           ▼
                  VictoriaMetrics Server
                           │
                           ▼
                    Grafana Dashboard
```

## 部署特性

### 标准化部署
- 统一部署脚本 (deploy.sh)
- 标准目录结构 (/opt/app/)
- systemd服务管理
- 自动网络接口检测

### 多架构支持
- Linux ARM64 (机器人本体)
- Linux AMD64 (服务器/虚拟机)
- 交叉编译支持

### 配置管理
- 单一YAML配置文件
- 环境特定配置示例
- 热更新支持（需重启服务）
- 配置验证机制

### 运维工具
- 状态检查脚本 (status.sh)
- 启动/停止/重启脚本
- 日志管理和轮转
- 临时文件自动清理

## 监控指标体系

### 系统指标 (node_*)
```
node_cpu_seconds_total         # CPU时间统计
node_cpu_temperature_celsius   # CPU温度
node_memory_*_bytes           # 内存指标
node_disk_*_total            # 磁盘I/O
node_network_*_total         # 网络流量
node_network_bandwidth_*_mbps # 实时带宽
node_load1/5/15             # 系统负载
```

### BMS指标 (robot_battery_*)
```
robot_battery_voltage_volts        # 电压
robot_battery_current_amperes      # 电流
robot_battery_soc_percent         # 电量
robot_battery_temperature_celsius  # 温度
robot_battery_power_watts         # 功率
robot_battery_cycles_total        # 循环次数
robot_battery_health_percent      # 健康度
```

### ROS指标 (ros_*)
```
ros_nodes_total                      # 节点总数
ros_node_status                      # 节点状态
ros_topics_total                     # Topic总数
ros_topic_frequency_hz               # 发布频率
ros_topic_publishers_total           # 发布者数
ros_topic_subscribers_total          # 订阅者数
ros_topic_last_message_age_seconds   # 数据新鲜度
```

### Exporter指标 (ros_exporter_*)
```
ros_exporter_up                    # 运行状态
ros_exporter_collection_duration   # 采集耗时
ros_exporter_metrics_count        # 指标数量
ros_exporter_push_duration        # 推送耗时
```

## 使用场景

### 1. 开发环境
- 禁用硬件相关监控（BMS、温度）
- 使用本地VictoriaMetrics
- 调试日志级别

### 2. 测试环境
- 虚拟机或测试机器人
- 指定测试网络接口
- 中等推送频率

### 3. 生产环境
- 完整功能启用
- 高频数据推送
- 生产级错误处理

## 项目优势

1. **统一监控**：一个导出器覆盖所有监控需求
2. **主动推送**：解决机器人动态IP和网络不稳定问题
3. **标准化**：Prometheus格式，易于集成
4. **高可用**：完善的错误处理和自动恢复
5. **易部署**：标准化部署流程，开箱即用
6. **可扩展**：模块化设计，易于添加新功能

## 与参考项目的关系

robot_control是一个C++ ROS项目，提供了以下参考价值：
- BMS数据结构定义参考
- 系统监控算法参考（CPU、内存、带宽计算）
- JSON数据格式参考

但ros_exporter是完全独立的实现：
- 使用Go语言重新实现所有功能
- 采用不同的架构设计（主动推送 vs ROS发布）
- 提供更完整的企业级特性
- 不依赖ROS环境（可选支持）

---

*文档更新时间：2024年12月*
*版本：2.0.0* 
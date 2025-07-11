# 🚀 ROS Exporter Grafana MCP 快速开始指南

## ✅ 已完成

### 1. MCP Server 已启动

MCP Server 正在运行在 `http://localhost:8080`，提供以下功能：

- ✅ **Dashboard 配置**: 基于 ros-exporter 所有指标的完整监控面板
- ✅ **数据源配置**: VictoriaMetrics 连接配置  
- ✅ **健康检查**: 服务状态监控

### 2. API 端点

```bash
# 健康检查
curl http://localhost:8080/health

# 获取 Dashboard 配置
curl http://localhost:8080/api/dashboards

# 获取数据源配置  
curl http://localhost:8080/api/datasources
```

## 🔄 下一步操作

### 步骤 1: 启动 VictoriaMetrics

```bash
# 启动 VictoriaMetrics 时序数据库
docker run -d --name victoria-metrics -p 8428:8428 \
  victoriametrics/victoria-metrics:latest \
  --storageDataPath=/victoria-metrics-data \
  --httpListenAddr=:8428 \
  --retentionPeriod=30d
```

### 步骤 2: 配置 ros-exporter

确保你的 `config.yaml` 中的 VictoriaMetrics 端点配置正确：

```yaml
victoria_metrics:
  endpoint: "http://localhost:8428/api/v1/import/prometheus"
  timeout: 30s
  extra_labels:
    job: "ros_exporter"
    environment: "production"
```

### 步骤 3: 启动 Grafana

```bash
# 启动 Grafana
docker run -d --name grafana -p 3000:3000 \
  -e GF_SECURITY_ADMIN_PASSWORD=admin123 \
  -e GF_FEATURE_TOGGLES_ENABLE=managedDashboards \
  -e GF_MANAGED_DASHBOARDS_ENABLED=true \
  -e GF_MANAGED_DASHBOARDS_URL=http://host.docker.internal:8080 \
  grafana/grafana:latest
```

### 步骤 4: 访问 Grafana

1. 打开浏览器访问: http://localhost:3000
2. 登录: `admin` / `admin123`
3. Dashboard 会通过 MCP 自动加载

## 📊 监控面板内容

Dashboard 包含以下监控模块：

### 🖥️ 系统监控
- CPU 使用率和温度
- 内存使用情况
- 网络 I/O 和带宽
- 系统负载

### 🤖 ROS 监控
- ROS Master 状态
- 节点数量和状态
- Topic 频率和健康度
- 业务 Topic 监控

### 🔋 电池监控
- 电池电量百分比
- 电压、电流、功率
- 电池温度和健康度
- 充电周期统计

### 🐕 B2 机器狗监控
- 运动速度和负载
- 关节温度和扭矩
- 传感器状态
- 安全和稳定性评分

### 📈 Exporter 性能
- 指标收集数量
- 推送耗时统计
- 数据新鲜度

## 🔧 支持的指标

### 系统指标
```
node_cpu_seconds_total          # CPU 使用时间
node_cpu_temperature_celsius    # CPU 温度
node_memory_*_bytes            # 内存使用情况
node_network_*_total           # 网络流量统计
node_load1/5/15               # 系统负载
```

### ROS 指标
```
ros_nodes_total               # ROS 节点总数
ros_topics_total             # Topic 总数
ros_topic_frequency_hz       # Topic 发布频率
ros_master_status           # ROS Master 状态
```

### 电池指标
```
robot_battery_soc_percent           # 电池电量百分比
robot_battery_voltage_volts         # 电池电压
robot_battery_current_amperes       # 电池电流
robot_battery_temperature_celsius   # 电池温度
robot_battery_health_percent        # 电池健康度
```

### B2 机器狗指标
```
b2_current_speed_mps           # 当前速度
b2_joint_temperature_celsius   # 关节温度
b2_emergency_stop             # 急停状态
b2_collision_risk_score       # 碰撞风险评分
```

## 🛠️ 管理命令

```bash
# 停止 MCP Server
./stop-simple.sh

# 重启 MCP Server
./stop-simple.sh && ./start-simple.sh

# 查看 MCP Server 日志
tail -f mcp-server.log  # 如果有日志文件

# 检查服务状态
curl http://localhost:8080/health
```

## 🔍 故障排除

### MCP Server 无法访问

```bash
# 检查进程是否运行
ps aux | grep mcp-server-simple.py

# 检查端口是否监听
netstat -tulpn | grep 8080

# 重启服务
./stop-simple.sh && ./start-simple.sh
```

### Dashboard 未加载到 Grafana

1. 检查 Grafana 的 MCP 配置
2. 确认 MCP Server 可访问: `curl http://localhost:8080/api/dashboards`
3. 查看 Grafana 日志中的 MCP 相关信息

### 数据不显示

1. 确认 ros-exporter 正在运行: `curl http://localhost:9100/metrics`
2. 检查 VictoriaMetrics 中是否有数据: `curl "http://localhost:8428/api/v1/query?query=ros_exporter_up"`
3. 验证数据源配置正确

## 📝 自定义配置

### 修改 Dashboard

1. 编辑 `dashboards/ros-exporter-dashboard.json`
2. 重启 MCP Server: `./stop-simple.sh && ./start-simple.sh`
3. Grafana 会自动重新加载配置

### 添加新指标

1. 在 ros-exporter 中添加新的指标收集
2. 更新 Dashboard JSON 添加对应的面板
3. 重启 MCP Server

## 🎯 完整架构

```
ros-exporter (localhost:9100) 
    ↓ (推送指标)
VictoriaMetrics (localhost:8428)
    ↓ (查询数据) 
Grafana (localhost:3000)
    ↓ (获取配置)
MCP Server (localhost:8080)
```

## ✨ 特色功能

- 🔄 **自动配置**: Dashboard 通过 MCP 自动加载
- 📊 **全面监控**: 涵盖系统、ROS、电池、机器狗所有指标
- 🎨 **美观界面**: 现代化的深色主题 Dashboard
- 🚨 **智能告警**: 基于阈值的颜色编码和告警
- 📱 **响应式设计**: 适配不同屏幕尺寸

现在你的 ROS Exporter Grafana MCP 监控环境已经完全配置好了！🎉 
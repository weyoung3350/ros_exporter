# Grafana Dashboard 部署指南

## 📊 ros_exporter Dashboard

### 🎯 Dashboard概述

专为ros_exporter设计的Grafana监控面板，包含以下监控视图：

#### 📈 核心系统指标
1. **CPU使用率** - 实时CPU利用率百分比
2. **内存使用率** - 内存利用率和详细分布
3. **系统负载** - 1/5/15分钟系统负载平均值
4. **磁盘使用情况** - 文件系统空间使用状态

#### 🌐 网络监控
5. **网络I/O速率** - ens160接口的实时收发速率
6. **实时网络带宽** - ros_exporter特有的带宽计算指标

#### 🤖 Robot Exporter状态
7. **Exporter状态仪表盘** - 监控导出器健康状态
8. **CPU时间分布饼图** - CPU各模式时间占比
9. **Exporter性能指标** - 推送指标数量和耗时

### 🚀 部署步骤

#### 1. 配置VictoriaMetrics数据源

在Grafana中添加VictoriaMetrics数据源：

```
数据源类型: Prometheus
名称: VictoriaMetrics
URL: <your_vm_url> # 请填写你的时序数据库地址
Access: Server (default)
```

**验证连接**：
```bash
# 测试VictoriaMetrics连通性
curl <your_vm_url>/api/v1/query?query=up

# 验证ros_exporter指标
curl -s '<your_vm_url>/api/v1/export' | grep 'instance="ros2-ubuntu-179"' | head -5
```

#### 2. 导入Dashboard

**方法1: JSON文件导入**
1. 登录Grafana (通常是 http://your-grafana:3000)
2. 点击 "+" → "Import"
3. 上传 `grafana-dashboard.json` 文件
4. 选择VictoriaMetrics作为数据源
5. 点击"Import"

**方法2: JSON内容粘贴**
1. 复制 `grafana-dashboard.json` 文件内容
2. 在Grafana Import页面选择"Import via panel json"
3. 粘贴JSON内容
4. 配置数据源并导入

#### 3. Dashboard配置验证

导入后检查以下配置：

```yaml
Dashboard设置:
  - 标题: "ros_exporter - ROS2 Ubuntu Dashboard"
  - UID: "ros-exporter-ros2"
  - 刷新间隔: 5秒
  - 时间范围: 最近1小时
  - 标签: robot, monitoring, ros2, system
```

### 📊 面板说明

#### 系统监控面板

1. **CPU使用率** (时间序列)
   ```promql
   100 - (avg(irate(node_cpu_seconds_total{instance="ros2-ubuntu-179",mode="idle"}[5m])) * 100)
   ```

2. **内存使用率** (时间序列)
   ```promql
   (1 - (node_memory_MemAvailable_bytes{instance="ros2-ubuntu-179"} / node_memory_MemTotal_bytes{instance="ros2-ubuntu-179"})) * 100
   ```

3. **系统负载** (时间序列)
   ```promql
   node_load1{instance="ros2-ubuntu-179"}
   node_load5{instance="ros2-ubuntu-179"}
   node_load15{instance="ros2-ubuntu-179"}
   ```

#### 网络监控面板

4. **网络I/O速率** (时间序列)
   ```promql
   irate(node_network_receive_bytes_total{instance="ros2-ubuntu-179",device="ens160"}[5m])
   irate(node_network_transmit_bytes_total{instance="ros2-ubuntu-179",device="ens160"}[5m])
   ```

5. **实时网络带宽** (ros_exporter特有)
   ```promql
   node_network_bandwidth_up_mbps{instance="ros2-ubuntu-179",device="ens160"} * 1024 * 1024 / 8
   node_network_bandwidth_down_mbps{instance="ros2-ubuntu-179",device="ens160"} * 1024 * 1024 / 8
   ```

#### Exporter状态面板

6. **ros_exporter状态** (仪表盘)
   ```promql
   ros_exporter_up{instance="ros2-ubuntu-179"}
   ```

7. **Exporter性能指标** (时间序列)
   ```promql
   ros_exporter_metrics_count{instance="ros2-ubuntu-179"}
ros_exporter_push_duration_seconds{instance="ros2-ubuntu-179"}
   ```

### 🔧 自定义配置

#### 修改实例标识
如果需要监控其他实例，修改查询中的instance标签：
```promql
# 将 instance="ros2-ubuntu-179" 替换为您的实例名
node_cpu_seconds_total{instance="your-instance-name",mode="idle"}
```

#### 添加告警规则
在Grafana中为关键指标配置告警：

```yaml
告警建议:
  - CPU使用率 > 80%
  - 内存使用率 > 85%
  - 磁盘使用率 > 90%
  - Robot Exporter状态 = 0 (离线)
  - 系统负载 > CPU核心数
```

#### 网络接口适配
如果网络接口不是ens160，修改相关查询：
```promql
# 替换device="ens160"为实际网卡名称
node_network_receive_bytes_total{instance="ros2-ubuntu-179",device="your-interface"}
```

### 📱 移动端适配

Dashboard已优化移动端显示：
- 响应式布局
- 合理的面板大小
- 清晰的图例和标签

### 🎨 主题和样式

- **默认主题**: Dark (深色主题)
- **调色板**: palette-classic (经典配色)
- **刷新频率**: 5秒 (适合实时监控)

### 🔍 故障排除

#### 1. 数据源连接问题
```bash
# 检查VictoriaMetrics状态
curl <your_vm_url>/metrics

# 检查Robot Exporter指标
curl -s '<your_vm_url>/api/v1/export' | grep ros_exporter_up
```

#### 2. 指标缺失问题
```bash
# 确认Robot Exporter运行状态
ssh -i ~/.ssh/id_rsa_ubuntu_vm dna@<your_ip> "ps aux | grep ros_exporter" # 请填写你的服务器IP

# 检查推送日志
ssh -i ~/.ssh/id_rsa_ubuntu_vm dna@<your_ip> "cd ros_exporter-1.0.0 && tail -f nohup.out" # 请填写你的服务器IP
```

#### 3. 网络接口不匹配
```bash
# 检查实际网络接口
ssh -i ~/.ssh/id_rsa_ubuntu_vm dna@<your_ip> "ip addr show" # 请填写你的服务器IP

# 更新Dashboard查询中的device标签
```

### 📈 性能优化

1. **查询优化**: 使用适当的时间范围和采样间隔
2. **面板限制**: 避免同时显示过多高频数据
3. **缓存设置**: 配置合理的缓存策略

### 🎯 扩展功能

可以基于此Dashboard扩展：
1. **ROS2特定指标** - 添加ROS2节点和话题监控
2. **BMS监控** - 集成电池管理系统指标
3. **自定义告警** - 配置邮件/钉钉通知
4. **历史数据分析** - 长期趋势分析面板

### 📝 维护建议

1. **定期备份**: 导出Dashboard JSON配置
2. **版本控制**: 跟踪Dashboard配置变更
3. **性能监控**: 监控Grafana自身性能
4. **数据清理**: 定期清理过期监控数据 
# ros_exporter - ROS2部署指南

## 📋 VMware Fusion ROS2环境部署

### 🎯 部署目标
在VMware Fusion虚拟机中的ROS2环境上部署和测试ros_exporter。

### 📦 部署包准备

已生成的跨平台发布包：
```
dist/ros_exporter-1.0.0.tar.gz  (21MB)
```

包含文件：
- `ros_exporter-linux-amd64` - Linux x64可执行文件
- `ros_exporter-linux-arm64` - Linux ARM64可执行文件  
- `config.example.yaml` - 配置文件模板
- `start.sh` - 启动脚本
- `README.md` - 完整文档

### 🚀 部署步骤

#### 1. 传输文件到ROS2虚拟机

```bash
# 方法1: 通过共享文件夹
# 在VMware Fusion中设置共享文件夹，将tar.gz文件复制到共享目录

# 方法2: 通过SCP (如果虚拟机有SSH)
scp dist/ros_exporter-1.0.0.tar.gz user@vm-ip:/home/user/

# 方法3: 通过HTTP服务器
# 在宿主机上：python3 -m http.server 8000
# 在虚拟机中：wget http://host-ip:8000/ros_exporter-1.0.0.tar.gz
```

#### 2. 在ROS2虚拟机中解压和配置

```bash
# 解压发布包
tar -xzf ros_exporter-1.0.0.tar.gz
cd ros_exporter-1.0.0

# 复制配置文件
cp config.example.yaml config.yaml

# 添加执行权限
chmod +x ros_exporter-linux-amd64
chmod +x start.sh
```

#### 3. 配置VictoriaMetrics (测试环境)

```bash
# 快速启动VictoriaMetrics (用于测试)
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.93.0/victoria-metrics-linux-amd64-v1.93.0.tar.gz
tar -xzf victoria-metrics-linux-amd64-v1.93.0.tar.gz

# 启动VictoriaMetrics
./victoria-metrics-prod &

# 验证运行
curl <your_vm_url>/metrics # 请填写你的时序数据库地址
```

#### 4. 配置文件调整

编辑 `config.yaml`：

```yaml
exporter:
  push_interval: 15s
  instance: "ros2-vm-test"
  log_level: "info"

victoria_metrics:
  endpoint: "<your_endpoint>" # 请填写你的推送地址
  timeout: 30s
  extra_labels:
    job: "ros_exporter"
    environment: "ros2-test"

collectors:
  system:
    enabled: true
    collectors: ["cpu", "memory", "disk", "network", "load"]
    
    # 温度监控配置
    temperature:
      enabled: true
      temp_source: "thermal_zone"  # 虚拟机中通常使用thermal_zone
      thermal_zone: "/sys/class/thermal/thermal_zone0/temp"
    
    # 网络监控配置  
    network:
      enabled: true
      interfaces: ["ens33", "eth0"]  # 常见的虚拟机网卡名称
      bandwidth_enabled: true
      exclude_loopback: true
  
  bms:
    enabled: false  # 虚拟机中禁用BMS监控
  
  ros:
    enabled: true
    master_uri: "http://localhost:11311"  # ROS2不需要，但保留兼容性
```

### 🧪 测试步骤

#### 1. 基础功能测试

```bash
# 检查版本
./ros_exporter-linux-amd64 -version

# 测试配置文件加载
./ros_exporter-linux-amd64 -config config.yaml &
PID=$!

# 等待几秒后检查日志
sleep 5
kill $PID
```

#### 2. 系统监控测试

```bash
# 指定网络接口测试
./ros_exporter-linux-amd64 -interfaces ens33 &

# 检查VictoriaMetrics中的数据
curl -s '<your_vm_url>/api/v1/export' | grep ros_exporter # 请填写你的时序数据库地址
```

#### 3. 温度监控测试

```bash
# 检查thermal_zone是否可用
ls /sys/class/thermal/thermal_zone*/temp
cat /sys/class/thermal/thermal_zone0/temp

# 如果thermal_zone不可用，尝试sensors
sudo apt-get update
sudo apt-get install lm-sensors
sensors-detect --auto
sensors
```

#### 4. 网络带宽监控测试

```bash
# 查看网络接口
ip addr show

# 生成网络流量进行测试
ping -c 10 8.8.8.8 &
wget -O /dev/null http://speedtest.ftp.otenet.gr/files/test1Mb.db &

# 观察带宽数据
curl -s '<your_vm_url>/api/v1/export' | grep bandwidth # 请填写你的时序数据库地址
```

### 📊 验证指标

成功部署后应该能看到以下指标：

```bash
# 系统指标
node_cpu_seconds_total{instance="ros2-vm-test"}
node_memory_MemTotal_bytes{instance="ros2-vm-test"}
node_load1{instance="ros2-vm-test"}

# 温度指标
node_cpu_temperature_celsius{instance="ros2-vm-test",sensor="cpu"}

# 网络带宽指标
node_network_bandwidth_up_mbps{instance="ros2-vm-test",device="ens33"}
node_network_bandwidth_down_mbps{instance="ros2-vm-test",device="ens33"}

# Exporter健康指标
ros_exporter_up{instance="ros2-vm-test",version="1.0.0"}
```

### 🔧 故障排除

#### 1. 网络接口识别
```bash
# 查看所有网络接口
ip link show
cat /proc/net/dev

# 常见虚拟机网卡名称
# VMware: ens33, ens32
# VirtualBox: enp0s3, enp0s8
# QEMU/KVM: ens3, ens4
```

#### 2. 权限问题
```bash
# 如果遇到权限问题
sudo ./ros_exporter-linux-amd64 -config config.yaml

# 或者调整文件权限
sudo chown $USER:$USER ros_exporter-linux-amd64
```

#### 3. 温度监控问题
```bash
# 检查thermal_zone
find /sys -name "*thermal*" -type d 2>/dev/null
ls /sys/class/thermal/

# 如果没有thermal_zone，禁用温度监控
# 在config.yaml中设置: temperature.enabled: false
```

### 🚀 生产环境部署建议

1. **服务化部署**：
```bash
# 创建systemd服务
sudo cp ros_exporter-linux-amd64 /usr/local/bin/
sudo cp config.yaml /opt/app/ros_exporter/

# 创建服务文件 /etc/systemd/system/ros_exporter.service
```

2. **日志管理**：
```bash
# 配置日志轮转
sudo mkdir -p /var/log/ros_exporter
```

3. **监控告警**：
```bash
# 配置Grafana仪表板
# 设置告警规则
```

### 📝 测试清单

- [ ] 可执行文件正常启动
- [ ] 配置文件正确加载  
- [ ] VictoriaMetrics连接成功
- [ ] 系统指标正常收集
- [ ] CPU温度监控工作
- [ ] 网络带宽计算正确
- [ ] 指标推送到VictoriaMetrics
- [ ] 优雅退出功能正常

### 🎯 下一步

部署成功后，可以：
1. 集成到ROS2工作流中
2. 配置Grafana可视化
3. 设置告警规则
4. 与原C++进程管理系统协同工作 
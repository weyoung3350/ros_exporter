# ros_exporter 机器人本体部署指南

## 概述

本指南用于在宇树Go2机器人本体上部署ros_exporter，实现电池管理系统(BMS)监控、系统资源监控和ROS节点监控。

## 部署包内容

- `ros_exporter-linux-arm64`: ARM64架构的可执行文件
- `config.yaml`: 统一配置文件，支持多种部署环境
- `deploy.sh`: 标准化部署脚本

## 系统要求

### 硬件要求
- **架构**: Linux ARM64 (aarch64) 或 x86_64
- **内存**: 至少512MB可用内存
- **存储**: 至少50MB可用磁盘空间
- **网络**: 能够访问VictoriaMetrics服务器

### 软件要求
- **操作系统**: Linux (Ubuntu 18.04+ 推荐)
- **权限**: root或sudo权限
- **systemd**: 支持systemd服务管理
- **网络工具**: curl, ip命令

## 部署步骤

### 1. 传输部署包

将生成的tar.gz部署包传输到机器人本体：

```bash
# 使用scp传输（假设机器人IP为192.168.1.100）
scp ros_exporter-deployment-*.tar.gz robot@<your_ip>:/tmp/ # 请填写你的服务器IP

# 或使用其他传输方式（U盘、网络共享等）
```

### 2. 解压部署包

在机器人本体上：

```bash
cd /tmp
tar -xzf ros_exporter-deployment-*.tar.gz
cd ros_exporter-deployment-*
```

### 3. 执行部署

运行标准化部署脚本：

```bash
sudo ./deploy.sh
```

部署脚本将自动执行以下操作：
- 检查系统架构和权限
- 使用root用户运行服务
- 停止现有服务（如果存在）
- 创建标准化目录结构
- 安装可执行文件到 `/opt/app/ros_exporter/`
- 安装配置文件到 `/opt/app/ros_exporter/`
- 自动检测网络接口并更新配置
- 创建systemd服务（以root用户运行）
- 启动监控服务
- 创建管理脚本

### 4. 验证部署

检查服务状态：

```bash
# 查看服务状态
systemctl status ros_exporter

# 查看实时日志
journalctl -u ros_exporter -f

# 使用管理脚本
/opt/app/ros_exporter/status.sh
```

## 配置说明

### 主要配置文件位置（标准化部署规范）
- **应用目录**: `/opt/app/ros_exporter/`
- **应用配置**: `/opt/app/ros_exporter/config.yaml`
- **应用日志**: `/opt/logs/ros_exporter/`
- **管理脚本**: `/opt/app/ros_exporter/scripts/`

### 关键配置项

#### VictoriaMetrics配置
```yaml
victoria_metrics:
  endpoint: "<your_endpoint>" # 请填写你的推送地址
  timeout: 30s
```

#### BMS监控配置
```yaml
collectors:
  bms:
    enabled: true
    interface_type: "unitree_sdk"
    update_interval: 5s
```

#### 网络监控配置
```yaml
collectors:
  system:
    network:
      enabled: true
      interfaces: ["eth0", "wlan0"]  # 自动检测
      bandwidth_enabled: true
```

## 服务管理

### 常用命令

```bash
# 启动服务
systemctl start ros_exporter

# 停止服务
systemctl stop ros_exporter

# 重启服务
systemctl restart ros_exporter

# 查看服务状态
systemctl status ros_exporter

# 开机自启动
systemctl enable ros_exporter

# 禁用开机自启动
systemctl disable ros_exporter
```

### 管理脚本

```bash
# 状态检查
/opt/app/ros_exporter/scripts/status.sh

# 重启服务（通过systemd）
systemctl restart ros_exporter

# 停止服务（通过systemd）
systemctl stop ros_exporter
```

## 监控数据

### 采集的指标

1. **BMS数据**:
   - 电池电压、电流、温度
   - 电量百分比
   - 充电状态

2. **系统资源**:
   - CPU使用率、温度
   - 内存使用情况
   - 磁盘使用情况
   - 网络流量

3. **ROS状态**:
   - 节点运行状态
   - Topic发布频率
   - 服务可用性

### 数据推送

监控数据每10秒推送一次到VictoriaMetrics服务器，可通过以下方式查看：
- Grafana仪表板
- VictoriaMetrics Web UI
- Prometheus兼容的查询接口

## 故障排除

### 常见问题

#### 1. 服务无法启动

**症状**: `systemctl start ros_exporter` 失败

**解决步骤**:
```bash
# 查看详细错误信息
journalctl -u ros_exporter -n 50

# 检查可执行文件权限
ls -la /opt/app/ros_exporter/ros_exporter

# 检查配置文件语法
cat /opt/app/ros_exporter/config.yaml

# 手动测试启动
/opt/app/ros_exporter/ros_exporter -config /opt/app/ros_exporter/config.yaml
```

#### 2. 网络连接问题

**症状**: 数据无法推送到VictoriaMetrics

**解决步骤**:
```bash
# 测试网络连接
curl -I <your_endpoint> # 请填写你的推送地址

# 检查防火墙设置
iptables -L

# 检查DNS解析
nslookup <your_endpoint_host>
```

#### 3. BMS数据无法获取

**症状**: 电池监控数据缺失

**解决步骤**:
```bash
# 检查Unitree SDK
ls -la /usr/local/lib/libunitree_sdk2*

# 检查DDS服务
ps aux | grep dds

# 检查设备权限
ls -la /dev/
```

#### 4. 系统资源监控异常

**症状**: CPU、内存等数据异常

**解决步骤**:
```bash
# 检查系统工具
which free top htop

# 检查温度传感器
ls -la /sys/class/thermal/

# 检查网络接口
ip link show
```

### 日志分析

#### 查看日志
```bash
# 实时日志
journalctl -u ros_exporter -f

# 历史日志
journalctl -u ros_exporter -n 100

# 错误日志
journalctl -u ros_exporter -p err

# 应用日志文件
tail -f /opt/logs/ros_exporter/exporter.log
```

#### 日志级别调整
编辑配置文件 `/opt/app/ros_exporter/config.yaml`:
```yaml
exporter:
  log_level: "debug"  # info, warn, error, debug
```

然后重启服务：
```bash
systemctl restart ros_exporter
```

## 维护操作

### 配置更新

1. 编辑配置文件：
   ```bash
   sudo nano /opt/app/ros_exporter/config.yaml
   ```

2. 验证配置语法：
   ```bash
   /opt/app/ros_exporter/ros_exporter -config /opt/app/ros_exporter/config.yaml -validate
   ```

3. 重启服务：
   ```bash
   systemctl restart ros_exporter
   ```

### 版本升级

1. 停止现有服务：
   ```bash
   systemctl stop ros_exporter
   ```

2. 备份配置：
   ```bash
   cp /opt/app/ros_exporter/config.yaml /tmp/config.yaml.backup
   ```

3. 部署新版本（使用新的部署包）

4. 恢复自定义配置（如果需要）

### 卸载

```bash
# 停止并禁用服务
systemctl stop ros_exporter
systemctl disable ros_exporter

# 删除服务文件
rm /etc/systemd/system/ros_exporter.service
systemctl daemon-reload

# 删除程序文件（标准化路径）
rm -rf /opt/app/ros_exporter
rm -rf /opt/logs/ros_exporter

# 清理完成
```

## 性能调优

### 采集频率调整

根据需要调整监控频率：

```yaml
exporter:
  push_interval: 15s  # 推送间隔：5s-60s

collectors:
  bms:
    update_interval: 10s  # BMS采集间隔：1s-30s
```

### 资源限制

systemd服务已配置资源限制：
- 最大文件句柄：65536
- 最大进程数：32768

如需调整，编辑：`/etc/systemd/system/ros_exporter.service`

## 安全特性

标准化部署包含以下安全特性：
- 严格的文件系统权限控制
- systemd安全沙箱设置
- 日志轮转和管理
- 资源限制配置

## 联系支持

如遇到无法解决的问题，请提供以下信息：
- 机器人型号和系统版本
- 错误日志 (`journalctl -u ros_exporter -n 100`)
- 系统信息 (`uname -a`, `free -h`, `df -h`)
- 网络配置 (`ip addr show`)

---

**文档版本**: 2.0.0  
**更新时间**: 2025-06-30  
**适用版本**: ros_exporter 1.0.0 (标准化部署规范) 

**注意：所有管理脚本部署后与主程序同级，直接用 `./start.sh` 方式调用。** 
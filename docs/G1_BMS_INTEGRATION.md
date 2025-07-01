# 宇树G1电池监控集成说明

## 概述

ros_exporter已经实现了对宇树G1机器人电池监控系统（BMS）的支持。本文档说明G1电池监控的实现状态、配置方法和使用指南。

## 实现状态

### ✅ 已实现功能

1. **机器人类型自动检测**
   - 支持G1和Go2机器人的自动识别
   - 基于主机名、系统文件等多种检测方法
   - 可手动配置机器人类型

2. **G1特定的BMS接口**
   - 独立的G1连接和数据读取逻辑
   - 支持G1电池数据格式和规格
   - 错误处理和连接状态管理

3. **标准化数据输出**
   - 统一的电池指标格式
   - 支持电压、电流、SOC、温度等完整数据
   - 兼容VictoriaMetrics和Prometheus

4. **灵活配置**
   - 支持自动检测或手动指定机器人类型
   - 可配置网络接口和更新频率
   - 向后兼容Go2配置

### 🚧 待完善功能

1. **真实SDK集成**
   - 当前使用模拟数据，需要集成真实的宇树G1 SDK
   - 需要CGO绑定或C库调用
   - DDS通信初始化和消息订阅

2. **完整数据映射**
   - 40节电池单体电压监控
   - 12个温度传感器数据
   - 电池健康状态和循环次数

3. **错误恢复机制**
   - SDK连接失败的重试逻辑
   - 数据读取异常的处理
   - 网络中断的恢复机制

## 配置方法

### 基本配置

在 `config.yaml` 中配置G1电池监控：

```yaml
collectors:
  bms:
    enabled: true
    interface_type: "unitree_sdk"
    robot_type: "g1"  # 明确指定G1机器人
    network_interface: "eth0"  # DDS通信网络接口
    update_interval: 5s
```

### 自动检测配置

让系统自动检测机器人类型：

```yaml
collectors:
  bms:
    enabled: true
    interface_type: "unitree_sdk"
    robot_type: "auto"  # 自动检测
    network_interface: "eth0"
    update_interval: 5s
```

### 检测机制

系统使用以下方法检测机器人类型：

1. **主机名检测**：检查主机名是否包含"g1"或"go2"
2. **系统文件**：读取 `/etc/robot_type` 文件（如果存在）
3. **网络配置**：基于网络接口特征推断
4. **SDK响应**：从SDK响应数据推断类型
5. **默认值**：如果无法检测，默认为Go2（向后兼容）

## 监控指标

### G1电池指标

| 指标名称 | 类型 | 说明 | G1特定值 |
|---------|------|------|----------|
| `robot_battery_voltage_volts` | Gauge | 电池电压 (V) | 25.2V典型值 |
| `robot_battery_current_amperes` | Gauge | 电池电流 (A) | 正值放电，负值充电 |
| `robot_battery_soc_percent` | Gauge | 电量百分比 (%) | 0-100% |
| `robot_battery_temperature_celsius` | Gauge | 电池温度 (°C) | 12个传感器平均值 |
| `robot_battery_power_watts` | Gauge | 电池功率 (W) | 计算值 |
| `robot_battery_cycles_total` | Counter | 充电周期数 | 累计值 |
| `robot_battery_health_percent` | Gauge | 电池健康度 (%) | SOH值 |

### 标签信息

所有指标包含以下标签：

```yaml
labels:
  instance: "g1-robot-001"  # 机器人实例名
  battery_id: "main"        # 电池ID
  interface: "unitree_sdk"  # 接口类型
  robot_type: "g1"         # 机器人类型
```

## 部署说明

### 1. 配置更新

如果要明确指定G1机器人，更新配置文件：

```bash
# 编辑配置文件
sudo nano /opt/app/ros_exporter/config.yaml

# 修改robot_type为"g1"
collectors:
  bms:
    robot_type: "g1"
```

### 2. 重启服务

```bash
sudo systemctl restart ros_exporter
```

### 3. 验证监控

```bash
# 查看日志确认G1检测
journalctl -u ros_exporter -f | grep -i g1

# 检查指标数据
curl -s <your_vm_url>/api/v1/export | grep robot_battery # 请填写你的时序数据库地址
```

## 故障排除

### 常见问题

#### 1. 机器人类型检测错误

**症状**：日志显示检测为Go2但实际是G1

**解决方案**：
```bash
# 方法1：手动配置
sudo nano /opt/app/ros_exporter/config.yaml
# 设置 robot_type: "g1"

# 方法2：创建系统标识文件
echo "g1" | sudo tee /etc/robot_type

# 方法3：修改主机名
sudo hostnamectl set-hostname g1-robot-001
```

#### 2. BMS数据无法获取

**症状**：电池指标显示为0或异常值

**解决方案**：
```bash
# 检查网络接口
ip link show

# 检查DDS通信
ps aux | grep dds

# 验证宇树SDK
ls -la /usr/local/lib/libunitree*
```

#### 3. 连接频繁断开

**症状**：日志显示SDK连接不稳定

**解决方案**：
```yaml
# 调整更新频率
collectors:
  bms:
    update_interval: 10s  # 降低频率

# 检查网络稳定性
ping -c 10 <target_ip>
```

## 开发指南

### 完善SDK集成

如需完善真实的G1 SDK集成，需要修改以下文件：

1. **`internal/collectors/bms.go`**
   - `connectG1()` 函数：实现真实的DDS初始化
   - `readG1BMSData()` 函数：调用真实的SDK API

2. **添加CGO绑定**
   ```go
   /*
   #cgo CFLAGS: -I/usr/local/include
   #cgo LDFLAGS: -L/usr/local/lib -lunitree_sdk2
   #include <unitree/robot/g1/bms.h>
   */
   import "C"
   ```

3. **错误处理增强**
   - 添加重试机制
   - 实现连接状态监控
   - 数据验证和异常处理

### 测试方法

```bash
# 编译测试版本
cd ros_exporter
go build -tags test -o test-exporter main.go

# 运行测试
./test-exporter -config config.yaml -test-mode g1
```

## 参考资料

- [宇树G1技术文档](https://www.unitree.com/g1)
- [Unitree SDK2文档](https://github.com/unitreerobotics/unitree_sdk2)
- [DDS通信协议](https://www.omg.org/dds/)
- [ros_exporter架构](./README.md)

---

**版本**: 1.0.0  
**更新时间**: 2024-12-19  
**状态**: 框架完成，SDK集成待完善 
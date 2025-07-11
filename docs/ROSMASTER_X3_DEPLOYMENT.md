# ROSMaster-X3机器人部署指南

## 概述

本指南详细说明如何在ROSMaster-X3机器人上部署和配置ROS-Exporter监控系统。

## 硬件规格

### ROSMaster-X3标准配置
- **主控**: 树莓派5 (8GB RAM)
- **激光雷达**: 思岚A1M8 (12米测距，10Hz扫描)
- **深度相机**: Astra Pro Plus
- **电池**: 12.6V 6000mAh锂电池组
- **驱动**: 4个直流减速电机 + 麦克纳姆轮
- **传感器**: MPU6050 IMU、编码器
- **网络**: WiFi 802.11ac + 以太网

## 详细硬件清单与运行环境

### ROSMASTER X3 标准版物品清单
- 主控：树莓派5-8GB
- Astra Pro Plus 深度相机
- 思岚A1M8激光雷达
- 电池组（12.6V，6000mAh）
- 64G TF卡
- 摇臂悬挂架
- 防撞架
- 电机底板
- 车架主控固定板
- 摇臂挂支架
- 灯条固定板
- 码盘底板
- ROS小车扩展板
- USB HUB扩展板
- 电机4
- OLED屏扩展板
- 联轴器
- LED灯条
- 排线若干
- 数据线
- 螺丝刀
- 游戏手柄+7号电池
- 电池盒
- USB 3.0
- 电池充电器
- 零件包
- 手机支架
- 塑料轮6（含驱动轮4、从动轮2）
- 麦克纳姆轮4

### 语言交互包
- 语音交互模块
- Type-C数据线
- 语音蜂鸣包
- 喇叭

### 运行环境
- 机器人本体 IP: 192.168.31.109 (pi/yahboom)
- VictoriaMetrics 和 Grafana（admin/admin123）也部署在此机器上（Docker）

## 系统要求

### 软件环境
- **操作系统**: Ubuntu 20.04 LTS (ARM64)
- **ROS版本**: ROS1 Noetic
- **Go版本**: 1.21+ (用于编译)
- **Python**: 3.8+ (ROS依赖)

### 网络配置
- **VictoriaMetrics服务器**: 可访问的时序数据库
- **网络延迟**: < 100ms (推荐局域网)
- **带宽**: 最小1Mbps上行

## 安装步骤

### 1. 准备ROS环境

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装ROS Noetic (如果未安装)
sudo sh -c 'echo "deb http://packages.ros.org/ros/ubuntu $(lsb_release -sc) main" > /etc/apt/sources.list.d/ros-latest.list'
sudo apt-key adv --keyserver 'hkp://keyserver.ubuntu.com:80' --recv-key C1CF6E31E6BADE8868B172B4F42ED6FBAB17C654
sudo apt update
sudo apt install ros-noetic-desktop-full

# 初始化rosdep
sudo rosdep init
rosdep update

# 设置环境变量
echo "source /opt/ros/noetic/setup.bash" >> ~/.bashrc
source ~/.bashrc
```

### 2. 安装ROSMaster-X3驱动

```bash
# 克隆ROSMaster-X3驱动包
cd ~/catkin_ws/src
git clone https://github.com/YahboomTechnology/rosmaster_x3.git

# 安装依赖
rosdep install --from-paths . --ignore-src -r -y

# 编译工作空间
cd ~/catkin_ws
catkin_make

# 更新环境变量
echo "source ~/catkin_ws/devel/setup.bash" >> ~/.bashrc
source ~/.bashrc
```

### 3. 配置硬件接口

```bash
# 激光雷达权限设置
sudo usermod -a -G dialout $USER
sudo chmod 666 /dev/ttyUSB*

# 设置udev规则 (持久化设备名称)
sudo tee /etc/udev/rules.d/99-rosmaster-x3.rules > /dev/null <<EOF
# 思岚A1M8激光雷达
SUBSYSTEM=="tty", ATTRS{idVendor}=="10c4", ATTRS{idProduct}=="ea60", SYMLINK+="rplidar"

# BMS电池管理
SUBSYSTEM=="tty", ATTRS{idVendor}=="0403", ATTRS{idProduct}=="6001", SYMLINK+="bms"

# Arduino控制器 (电机控制)
SUBSYSTEM=="tty", ATTRS{idVendor}=="2341", ATTRS{idProduct}=="0043", SYMLINK+="arduino"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger
```

### 4. 部署ROS-Exporter

```bash
# 创建部署目录
sudo mkdir -p /opt/ros-exporter
cd /opt/ros-exporter

# 下载预编译版本 (推荐)
wget https://github.com/your-repo/ros_exporter/releases/latest/download/ros_exporter-linux-arm64
chmod +x ros_exporter-linux-arm64
sudo ln -sf /opt/ros-exporter/ros_exporter-linux-arm64 /usr/local/bin/ros_exporter

# 或者从源码编译
git clone https://github.com/your-repo/ros_exporter.git
cd ros_exporter
./build.sh build
sudo cp ros_exporter /usr/local/bin/
```

### 5. 配置监控系统

```bash
# 复制ROSMaster-X3专用配置
sudo cp config_rosmaster_x3.yaml /etc/ros-exporter/config.yaml

# 编辑配置文件
sudo nano /etc/ros-exporter/config.yaml
```

关键配置项：
```yaml
# 修改VictoriaMetrics端点
victoria_metrics:
  endpoint: "http://YOUR_VICTORIA_METRICS_SERVER:8428/api/v1/import/prometheus"

# 设置机器人ID
exporter:
  instance: "rosmaster-x3-001"  # 修改为实际机器人编号

# 根据实际网络接口调整
collectors:
  system:
    network:
      interfaces: ["wlan0", "eth0"]  # 检查实际接口名称
```

### 6. 创建系统服务

```bash
# 创建systemd服务文件
sudo tee /etc/systemd/system/ros-exporter.service > /dev/null <<EOF
[Unit]
Description=ROS Exporter for ROSMaster-X3
After=network.target
Requires=network.target

[Service]
Type=simple
User=pi
Group=pi
Environment=ROS_MASTER_URI=http://localhost:11311
Environment=ROS_IP=$(hostname -I | awk '{print $1}')
ExecStartPre=/bin/bash -c 'source /opt/ros/noetic/setup.bash && source /home/pi/catkin_ws/devel/setup.bash'
ExecStart=/usr/local/bin/ros_exporter -config /etc/ros-exporter/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# 启用并启动服务
sudo systemctl daemon-reload
sudo systemctl enable ros-exporter
sudo systemctl start ros-exporter

# 检查服务状态
sudo systemctl status ros-exporter
```

## ROS话题映射

### 标准ROSMaster-X3话题列表

```bash
# 运动控制
/cmd_vel                    # 速度控制命令
/odom                      # 里程计数据
/joint_states              # 关节状态

# 传感器数据
/scan                      # 激光雷达扫描数据
/imu                       # IMU数据
/camera/rgb/image_raw      # RGB图像
/camera/depth/image_raw    # 深度图像

# 导航
/amcl_pose                 # AMCL定位
/move_base/goal            # 导航目标
/move_base/status          # 导航状态
/map                       # 地图数据
/path                      # 路径规划

# 机器人状态 (需要自定义发布)
/rosmaster/battery_state   # 电池状态
/rosmaster/motor_state     # 电机状态
/rosmaster/system_state    # 系统状态
```

### 自定义状态发布节点

创建 `/home/pi/catkin_ws/src/rosmaster_monitor/scripts/state_publisher.py`:

```python
#!/usr/bin/env python3

import rospy
import psutil
import serial
from std_msgs.msg import Float32MultiArray, Bool, Int32
from geometry_msgs.msg import Twist

class ROSMasterStatePublisher:
    def __init__(self):
        rospy.init_node('rosmaster_state_publisher')
        
        # 发布器
        self.battery_pub = rospy.Publisher('/rosmaster/battery_state', Float32MultiArray, queue_size=1)
        self.motor_pub = rospy.Publisher('/rosmaster/motor_state', Float32MultiArray, queue_size=1) 
        self.system_pub = rospy.Publisher('/rosmaster/system_state', Float32MultiArray, queue_size=1)
        
        # 串口连接 (BMS)
        try:
            self.bms_serial = serial.Serial('/dev/bms', 9600, timeout=1)
        except:
            self.bms_serial = None
            rospy.logwarn("BMS串口连接失败")
        
        # 定时器
        rospy.Timer(rospy.Duration(1.0), self.publish_battery_state)
        rospy.Timer(rospy.Duration(0.5), self.publish_motor_state)
        rospy.Timer(rospy.Duration(2.0), self.publish_system_state)
        
    def publish_battery_state(self, event):
        msg = Float32MultiArray()
        
        if self.bms_serial:
            try:
                # 读取BMS数据 (需要根据实际协议调整)
                self.bms_serial.write(b'\\x01\\x03\\x00\\x00\\x00\\x0A\\xC5\\xCD')
                data = self.bms_serial.read(23)
                if len(data) >= 23:
                    voltage = int.from_bytes(data[3:5], 'big') * 0.01
                    current = int.from_bytes(data[5:7], 'big') * 0.01
                    soc = int.from_bytes(data[9:11], 'big') * 0.01
                    temp = int.from_bytes(data[11:13], 'big') * 0.1 - 273.15
                    
                    msg.data = [voltage, current, soc, temp]
                else:
                    msg.data = [12.3, 2.5, 75.0, 35.0]  # 默认值
            except:
                msg.data = [12.3, 2.5, 75.0, 35.0]  # 默认值
        else:
            msg.data = [12.3, 2.5, 75.0, 35.0]  # 默认值
            
        self.battery_pub.publish(msg)
    
    def publish_motor_state(self, event):
        msg = Float32MultiArray()
        # 模拟电机数据 (实际应从Arduino读取)
        temp_data = [45.0, 46.0, 44.0, 45.5]  # 4个电机温度
        torque_data = [2.1, 2.3, 2.0, 2.2]    # 4个电机扭矩
        speed_data = [120, 125, 118, 122]      # 4个电机转速
        
        msg.data = temp_data + torque_data + speed_data
        self.motor_pub.publish(msg)
    
    def publish_system_state(self, event):
        msg = Float32MultiArray()
        
        # 系统状态
        cpu_usage = psutil.cpu_percent()
        memory_usage = psutil.virtual_memory().percent
        disk_usage = psutil.disk_usage('/').percent
        
        # 网络状态
        wifi_signal = -45.0  # 需要实际检测
        
        # 错误统计 (可以从日志文件读取)
        error_count = 0
        warning_count = 1
        safety_score = 0.85
        
        msg.data = [cpu_usage, memory_usage, disk_usage, wifi_signal, 
                   error_count, warning_count, safety_score]
        self.system_pub.publish(msg)

if __name__ == '__main__':
    try:
        publisher = ROSMasterStatePublisher()
        rospy.spin()
    except rospy.ROSInterruptException:
        pass
```

## 验证和测试

### 1. 检查ROS话题

```bash
# 检查所有话题
rostopic list

# 检查特定话题数据
rostopic echo /scan
rostopic echo /odom
rostopic echo /rosmaster/battery_state

# 检查话题频率
rostopic hz /scan
rostopic hz /imu
```

### 2. 验证监控数据

```bash
# 检查ROS-Exporter日志
sudo journalctl -u ros-exporter -f

# 测试HTTP端点
curl http://localhost:9100/health
curl http://localhost:9100/metrics

# 检查指标推送
tail -f /var/log/ros-exporter.log
```

### 3. 网络连接测试

```bash
# 测试到VictoriaMetrics的连接
curl -X POST http://YOUR_VM_SERVER:8428/api/v1/import/prometheus \
  -H "Content-Type: text/plain" \
  -d "test_metric 123"

# 检查网络延迟
ping YOUR_VM_SERVER
```

## 故障排除

### 常见问题

1. **激光雷达无数据**
   ```bash
   # 检查设备权限
   ls -la /dev/ttyUSB*
   sudo chmod 666 /dev/ttyUSB0
   
   # 检查思岚驱动
   roslaunch rplidar_ros rplidar.launch
   ```

2. **IMU数据异常**
   ```bash
   # 重新校准IMU
   rostopic pub /imu/calibrate std_msgs/Empty "{}"
   
   # 检查I2C连接
   sudo i2cdetect -y 1
   ```

3. **电池数据读取失败**
   ```bash
   # 检查串口连接
   sudo dmesg | grep tty
   
   # 测试串口通信
   sudo minicom -D /dev/ttyUSB1 -b 9600
   ```

4. **网络推送失败**
   ```bash
   # 检查网络配置
   ip route show
   
   # 测试DNS解析
   nslookup YOUR_VM_SERVER
   
   # 检查防火墙
   sudo ufw status
   ```

### 性能优化

1. **降低CPU使用率**
   ```yaml
   # 调整采集频率
   exporter:
     push_interval: 15s  # 从10s改为15s
   
   collectors:
     rosmaster_x3:
       update_interval: 10s  # 从5s改为10s
   ```

2. **减少网络带宽**
   ```yaml
   # 启用数据压缩
   victoria_metrics:
     extra_labels:
       compress: "gzip"
   
   # 过滤低价值话题
   collectors:
     rosmaster_x3:
       topic_blacklist: ["/tf", "/tf_static", "/clock"]
   ```

3. **存储空间管理**
   ```bash
   # 设置日志轮转
   sudo tee /etc/logrotate.d/ros-exporter > /dev/null <<EOF
   /var/log/ros-exporter.log {
       daily
       rotate 7
       compress
       missingok
       notifempty
       copytruncate
   }
   EOF
   ```

## 监控仪表板

部署完成后，可以在Grafana中导入ROSMaster-X3专用仪表板，实现可视化监控。

仪表板配置文件位于：`grafana-mcp/dashboards/rosmaster-x3-dashboard.json`

## 支持和维护

- **日志位置**: `/var/log/ros-exporter.log`
- **配置文件**: `/etc/ros-exporter/config.yaml`
- **服务管理**: `systemctl status/start/stop/restart ros-exporter`
- **更新检查**: `ros_exporter -version`

如需技术支持，请提供完整的日志信息和系统配置详情。
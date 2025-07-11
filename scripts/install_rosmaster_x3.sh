#!/bin/bash

# ROSMaster-X3 自动化安装脚本
# 用于在ROSMaster-X3机器人上快速部署ROS-Exporter监控系统

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 显示欢迎信息
show_welcome() {
    echo ""
    echo "========================================"
    echo "  ROSMaster-X3 监控系统安装向导"
    echo "========================================"
    echo ""
    echo "此脚本将自动安装和配置以下组件："
    echo "• ROS-Exporter监控程序"
    echo "• ROSMaster-X3专用配置"
    echo "• 系统服务和自启动"
    echo "• 硬件接口配置"
    echo ""
}

# 检查系统环境
check_environment() {
    log_step "检查系统环境..."
    
    # 检查操作系统
    if [[ $(uname -m) != "aarch64" ]]; then
        log_error "此脚本仅支持ARM64架构 (aarch64)"
        exit 1
    fi
    
    # 检查是否为root权限
    if [[ $EUID -eq 0 ]]; then
        log_error "请勿使用root权限运行此脚本"
        exit 1
    fi
    
    # 检查sudo权限
    if ! sudo -n true 2>/dev/null; then
        log_error "需要sudo权限，请确保当前用户具有sudo权限"
        exit 1
    fi
    
    # 检查ROS环境
    if [[ -z "$ROS_DISTRO" ]]; then
        log_warn "未检测到ROS环境，将在后续步骤中配置"
    else
        log_info "检测到ROS环境: $ROS_DISTRO"
    fi
    
    # 检查网络连接
    if ! ping -c 1 google.com &> /dev/null; then
        log_warn "网络连接异常，可能影响下载过程"
    fi
    
    log_info "系统环境检查完成"
}

# 安装依赖
install_dependencies() {
    log_step "安装系统依赖..."
    
    # 更新包列表
    sudo apt update
    
    # 安装基础依赖
    sudo apt install -y \
        curl \
        wget \
        unzip \
        git \
        python3-pip \
        python3-serial \
        python3-psutil \
        lm-sensors \
        minicom \
        usbutils \
        i2c-tools
    
    # 安装ROS依赖 (如果需要)
    if [[ -z "$ROS_DISTRO" ]]; then
        log_step "安装ROS Noetic..."
        
        # 添加ROS源
        sudo sh -c 'echo "deb http://packages.ros.org/ros/ubuntu $(lsb_release -sc) main" > /etc/apt/sources.list.d/ros-latest.list'
        
        # 添加密钥
        curl -s https://raw.githubusercontent.com/ros/rosdistro/master/ros.asc | sudo apt-key add -
        
        # 更新并安装
        sudo apt update
        sudo apt install -y ros-noetic-desktop-full
        
        # 初始化rosdep
        sudo rosdep init || true
        rosdep update
        
        # 设置环境变量
        echo "source /opt/ros/noetic/setup.bash" >> ~/.bashrc
        source /opt/ros/noetic/setup.bash
        
        log_info "ROS Noetic安装完成"
    fi
    
    log_info "依赖安装完成"
}

# 配置硬件接口
configure_hardware() {
    log_step "配置硬件接口..."
    
    # 添加用户到dialout组
    sudo usermod -a -G dialout $USER
    
    # 创建udev规则
    sudo tee /etc/udev/rules.d/99-rosmaster-x3.rules > /dev/null <<EOF
# ROSMaster-X3 硬件设备规则

# 思岚A1M8激光雷达
SUBSYSTEM=="tty", ATTRS{idVendor}=="10c4", ATTRS{idProduct}=="ea60", SYMLINK+="rplidar", MODE="0666"

# BMS电池管理系统
SUBSYSTEM=="tty", ATTRS{idVendor}=="0403", ATTRS{idProduct}=="6001", SYMLINK+="bms", MODE="0666"

# Arduino控制器 (电机控制)
SUBSYSTEM=="tty", ATTRS{idVendor}=="2341", ATTRS{idProduct}=="0043", SYMLINK+="arduino", MODE="0666"

# USB转串口通用规则
SUBSYSTEM=="tty", ATTRS{idVendor}=="1a86", ATTRS{idProduct}=="7523", MODE="0666"
SUBSYSTEM=="tty", ATTRS{idVendor}=="067b", ATTRS{idProduct}=="2303", MODE="0666"
EOF
    
    # 重新加载udev规则
    sudo udevadm control --reload-rules
    sudo udevadm trigger
    
    # 配置I2C权限 (用于IMU)
    sudo usermod -a -G i2c $USER
    
    log_info "硬件接口配置完成"
}

# 下载并安装ROS-Exporter
install_ros_exporter() {
    log_step "安装ROS-Exporter..."
    
    # 创建安装目录
    sudo mkdir -p /opt/ros-exporter
    sudo mkdir -p /etc/ros-exporter
    sudo mkdir -p /var/log/ros-exporter
    
    # 下载二进制文件 (模拟，实际应从GitHub Releases下载)
    cd /tmp
    
    # 如果有预编译版本，从这里下载
    BINARY_URL="https://github.com/your-repo/ros_exporter/releases/latest/download/ros_exporter-linux-arm64"
    
    if curl -f -L -o ros_exporter-linux-arm64 "$BINARY_URL" 2>/dev/null; then
        log_info "下载预编译版本成功"
        sudo cp ros_exporter-linux-arm64 /opt/ros-exporter/
        sudo chmod +x /opt/ros-exporter/ros_exporter-linux-arm64
        sudo ln -sf /opt/ros-exporter/ros_exporter-linux-arm64 /usr/local/bin/ros_exporter
    else
        log_warn "预编译版本下载失败，将从源码编译"
        
        # 检查Go环境
        if ! command -v go &> /dev/null; then
            log_step "安装Go语言环境..."
            
            GO_VERSION="1.21.5"
            GO_TARBALL="go${GO_VERSION}.linux-arm64.tar.gz"
            
            wget "https://golang.org/dl/${GO_TARBALL}"
            sudo tar -C /usr/local -xzf "${GO_TARBALL}"
            
            # 添加Go到PATH
            echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
            export PATH=$PATH:/usr/local/go/bin
            
            rm "${GO_TARBALL}"
            log_info "Go安装完成"
        fi
        
        # 从源码编译
        log_step "从源码编译ROS-Exporter..."
        
        git clone https://github.com/your-repo/ros_exporter.git
        cd ros_exporter
        
        # 编译 (禁用CGO以避免依赖问题)
        CGO_ENABLED=0 go build -o ros_exporter main.go
        
        sudo cp ros_exporter /opt/ros-exporter/
        sudo ln -sf /opt/ros-exporter/ros_exporter /usr/local/bin/ros_exporter
        
        cd ..
        rm -rf ros_exporter
        
        log_info "源码编译完成"
    fi
    
    # 设置权限
    sudo chown -R $USER:$USER /var/log/ros-exporter
    
    log_info "ROS-Exporter安装完成"
}

# 配置监控系统
configure_monitoring() {
    log_step "配置监控系统..."
    
    # 获取用户输入
    echo ""
    echo "请输入配置信息："
    
    read -p "VictoriaMetrics服务器地址 (例: 192.168.1.100:8428): " VM_SERVER
    read -p "机器人实例名称 (例: rosmaster-x3-001): " ROBOT_INSTANCE
    read -p "机器人部署位置 (例: lab): " ROBOT_LOCATION
    
    # 检测网络接口
    INTERFACES=$(ip -o link show | awk -F': ' '{print $2}' | grep -E '^(eth|wlan|enp|wlp)' | head -2 | tr '\n' ',' | sed 's/,$//')
    
    if [[ -z "$INTERFACES" ]]; then
        INTERFACES="wlan0,eth0"
        log_warn "无法自动检测网络接口，使用默认值: $INTERFACES"
    else
        log_info "检测到网络接口: $INTERFACES"
    fi
    
    # 生成配置文件
    sudo tee /etc/ros-exporter/config.yaml > /dev/null <<EOF
# ROSMaster-X3机器人监控配置
# 自动生成于: $(date)

exporter:
  push_interval: 10s
  instance: "${ROBOT_INSTANCE:-rosmaster-x3-001}"
  log_level: "info"
  
  http_server:
    enabled: true
    port: 9100
    address: "0.0.0.0"
    endpoints: ["health", "status", "metrics"]

victoria_metrics:
  endpoint: "http://${VM_SERVER:-localhost:8428}/api/v1/import/prometheus"
  timeout: 30s
  extra_labels:
    job: "ros_exporter"
    robot_type: "rosmaster_x3"
    location: "${ROBOT_LOCATION:-unknown}"
  retry:
    max_retries: 5
    retry_delay: 2s
    max_delay: 60s
    backoff_rate: 2.0

collectors:
  # 系统监控 - 针对树莓派5优化
  system:
    enabled: true
    collectors: ["cpu", "memory", "disk", "network", "load"]
    proc_path: "/proc"
    sys_path: "/sys"
    rootfs_path: "/"
    
    # 树莓派5温度监控
    temperature:
      enabled: true
      sensors_cmd: "vcgencmd measure_temp"
      temp_source: "thermal_zone"
      thermal_zone: "/sys/class/thermal/thermal_zone0/temp"
    
    # 网络监控
    network:
      enabled: true
      interfaces: [$(echo "$INTERFACES" | sed 's/,/", "/g' | sed 's/^/"/' | sed 's/$/"/' )]
      bandwidth_enabled: true
      exclude_loopback: true
    
    # 进程监控
    process:
      enabled: true
      monitor_all: false
      include_names: ["rosmaster", "roscore", "roslaunch", "python.*ros.*"]
      exclude_names: ["kthreadd", "ksoftirqd.*", "migration.*", "rcu_.*"]
      include_users: ["pi", "ros"]
      min_cpu_percent: 1.0
      min_memory_mb: 10.0
      collect_detailed: true

  # BMS监控
  bms:
    enabled: true
    interface_type: "serial"
    robot_type: "rosmaster_x3"
    network_interface: "wlan0"
    update_interval: 5s
    device_path: "/dev/bms"
    baud_rate: 9600
    can_interface: "can0"

  # 通用ROS监控
  ros:
    enabled: true
    master_uri: "http://localhost:11311"
    topic_whitelist: []
    topic_blacklist: ["/rosout", "/rosout_agg", "/tf_static", "/clock"]
    node_whitelist: []
    node_blacklist: ["/rosout"]
    scrape_interval: 3s

  # B2收集器 - 禁用
  b2:
    enabled: false

  # ROSMaster-X3专用收集器
  rosmaster_x3:
    enabled: true
    master_uri: "http://localhost:11311"
    robot_id: "${ROBOT_INSTANCE:-rosmaster-x3-001}"
    update_interval: 5s

    # 监控配置
    monitor_motors: true
    monitor_battery: true
    monitor_lidar: true
    monitor_imu: true
    monitor_navigation: true
    monitor_camera: true

    # 话题过滤
    topic_whitelist: [
      "/cmd_vel", "/odom", "/joint_states",
      "/scan", "/imu", 
      "/camera/rgb/image_raw", "/camera/depth/image_raw",
      "/amcl_pose", "/move_base/goal", "/move_base/status", "/map", "/path",
      "/rosmaster/battery_state", "/rosmaster/motor_state", "/rosmaster/system_state"
    ]
    
    topic_blacklist: [
      "/rosout", "/rosout_agg", "/tf_static", "/clock", "/diagnostics_agg"
    ]

    # 告警阈值
    max_motor_temp: 70.0
    max_battery_temp: 55.0
    min_battery_voltage: 10.5
    min_battery_soc: 15.0
    max_linear_velocity: 1.5
    max_angular_velocity: 1.5
EOF
    
    log_info "配置文件生成完成: /etc/ros-exporter/config.yaml"
}

# 创建系统服务
create_service() {
    log_step "创建系统服务..."
    
    # 创建systemd服务文件
    sudo tee /etc/systemd/system/ros-exporter.service > /dev/null <<EOF
[Unit]
Description=ROS Exporter for ROSMaster-X3
Documentation=https://github.com/your-repo/ros_exporter
After=network.target
Requires=network.target

[Service]
Type=simple
User=$USER
Group=$USER
Environment=HOME=/home/$USER
Environment=ROS_MASTER_URI=http://localhost:11311
Environment=ROS_IP=\$(hostname -I | awk '{print \$1}')
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/go/bin
ExecStartPre=/bin/bash -c 'if [ -f /opt/ros/noetic/setup.bash ]; then source /opt/ros/noetic/setup.bash; fi'
ExecStartPre=/bin/bash -c 'if [ -f /home/$USER/catkin_ws/devel/setup.bash ]; then source /home/$USER/catkin_ws/devel/setup.bash; fi'
ExecStart=/usr/local/bin/ros_exporter -config /etc/ros-exporter/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=append:/var/log/ros-exporter/ros-exporter.log
StandardError=append:/var/log/ros-exporter/ros-exporter.log

# 安全设置
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=no
ReadWritePaths=/var/log/ros-exporter
ReadOnlyPaths=/etc/ros-exporter

[Install]
WantedBy=multi-user.target
EOF
    
    # 创建日志轮转配置
    sudo tee /etc/logrotate.d/ros-exporter > /dev/null <<EOF
/var/log/ros-exporter/*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    copytruncate
    su $USER $USER
}
EOF
    
    # 重新加载systemd
    sudo systemctl daemon-reload
    
    log_info "系统服务创建完成"
}

# 启动服务
start_service() {
    log_step "启动监控服务..."
    
    # 启用自启动
    sudo systemctl enable ros-exporter
    
    # 启动服务
    sudo systemctl start ros-exporter
    
    # 等待启动
    sleep 3
    
    # 检查服务状态
    if sudo systemctl is-active --quiet ros-exporter; then
        log_info "✓ ROS-Exporter服务启动成功"
        
        # 显示服务状态
        echo ""
        echo "服务状态："
        sudo systemctl status ros-exporter --no-pager -l
        
    else
        log_error "✗ ROS-Exporter服务启动失败"
        echo ""
        echo "错误日志："
        sudo journalctl -u ros-exporter -n 20 --no-pager
        exit 1
    fi
}

# 验证安装
verify_installation() {
    log_step "验证安装结果..."
    
    # 检查HTTP端点
    if curl -f http://localhost:9100/health &>/dev/null; then
        log_info "✓ HTTP健康检查端点正常"
    else
        log_warn "✗ HTTP健康检查端点异常"
    fi
    
    # 检查指标端点
    if curl -f http://localhost:9100/metrics &>/dev/null; then
        log_info "✓ 指标端点正常"
    else
        log_warn "✗ 指标端点异常"
    fi
    
    # 检查日志
    if [[ -f "/var/log/ros-exporter/ros-exporter.log" ]]; then
        log_info "✓ 日志文件正常"
        
        # 显示最近的日志
        echo ""
        echo "最近的日志 (最后10行)："
        tail -10 /var/log/ros-exporter/ros-exporter.log
    else
        log_warn "✗ 日志文件未找到"
    fi
}

# 显示完成信息
show_completion() {
    echo ""
    echo "========================================"
    echo "  🎉 安装完成!"
    echo "========================================"
    echo ""
    echo "📊 监控仪表板: http://$(hostname -I | awk '{print $1}'):9100"
    echo "📈 Grafana导入: grafana-mcp/dashboards/rosmaster-x3-dashboard.json"
    echo "⚙️  配置文件: /etc/ros-exporter/config.yaml"
    echo "📋 日志文件: /var/log/ros-exporter/ros-exporter.log"
    echo ""
    echo "🔧 常用命令:"
    echo "  sudo systemctl status ros-exporter    # 查看服务状态"
    echo "  sudo systemctl restart ros-exporter   # 重启服务"
    echo "  sudo journalctl -u ros-exporter -f    # 查看实时日志"
    echo "  curl http://localhost:9100/health     # 健康检查"
    echo ""
    echo "📚 下一步:"
    echo "1. 配置VictoriaMetrics接收端点"
    echo "2. 在Grafana中导入监控仪表板"
    echo "3. 根据实际ROS话题调整配置"
    echo "4. 设置告警规则和通知"
    echo ""
    log_info "ROSMaster-X3监控系统安装成功！"
}

# 主函数
main() {
    show_welcome
    
    # 检查是否需要帮助
    if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
        echo "用法: $0 [选项]"
        echo ""
        echo "选项:"
        echo "  -h, --help     显示此帮助信息"
        echo "  --skip-deps    跳过依赖安装"
        echo "  --config-only  仅生成配置文件"
        echo ""
        exit 0
    fi
    
    # 执行安装步骤
    check_environment
    
    if [[ "$1" != "--config-only" ]]; then
        if [[ "$1" != "--skip-deps" ]]; then
            install_dependencies
        fi
        
        configure_hardware
        install_ros_exporter
    fi
    
    configure_monitoring
    
    if [[ "$1" != "--config-only" ]]; then
        create_service
        start_service
        verify_installation
        show_completion
    else
        log_info "配置文件已生成: /etc/ros-exporter/config.yaml"
    fi
}

# 运行主函数
main "$@"
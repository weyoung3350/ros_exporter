#!/bin/bash

# ROSMaster-X3部署包创建脚本
# 创建包含所有必要文件的部署包

set -e

DEPLOY_DIR="rosmaster_x3_deploy"
PACKAGE_NAME="rosmaster_x3_monitor_$(date +%Y%m%d_%H%M%S).tar.gz"

echo "=== 创建ROSMaster-X3部署包 ==="

# 清理旧的部署目录
rm -rf $DEPLOY_DIR

# 创建部署目录结构
mkdir -p $DEPLOY_DIR/{bin,config,scripts,docs,grafana}

# 复制二进制文件
echo "打包二进制文件..."
if [ -f "ros_exporter_nocgo" ]; then
    cp ros_exporter_nocgo $DEPLOY_DIR/bin/ros_exporter
    chmod +x $DEPLOY_DIR/bin/ros_exporter
else
    echo "错误: 找不到ros_exporter_nocgo文件"
    exit 1
fi

# 复制配置文件
echo "打包配置文件..."
cp config_rosmaster_x3.yaml $DEPLOY_DIR/config/
cp config.yaml $DEPLOY_DIR/config/config_default.yaml 2>/dev/null || true

# 复制脚本
echo "打包安装脚本..."
cp scripts/install_rosmaster_x3.sh $DEPLOY_DIR/scripts/
cp test_rosmaster_x3.sh $DEPLOY_DIR/scripts/
chmod +x $DEPLOY_DIR/scripts/*.sh

# 复制文档
echo "打包文档..."
cp docs/ROSMASTER_X3_DEPLOYMENT.md $DEPLOY_DIR/docs/
cp rosmaster_x3_metrics.md $DEPLOY_DIR/docs/
cp README.md $DEPLOY_DIR/docs/ 2>/dev/null || true

# 复制Grafana配置
echo "打包Grafana配置..."
cp grafana-mcp/dashboards/rosmaster-x3-dashboard.json $DEPLOY_DIR/grafana/

# 创建快速部署脚本
cat > $DEPLOY_DIR/quick_deploy.sh << 'EOF'
#!/bin/bash

# ROSMaster-X3快速部署脚本
# 在目标机器人上运行此脚本完成安装

set -e

echo "=== ROSMaster-X3监控系统快速部署 ==="
echo ""

# 检查是否为ARM64架构
if [[ $(uname -m) != "aarch64" && $(uname -m) != "arm64" ]]; then
    echo "错误: 此包仅支持ARM64架构的设备"
    exit 1
fi

# 运行完整安装脚本
echo "正在运行安装脚本..."
./scripts/install_rosmaster_x3.sh

echo ""
echo "快速部署完成！"
echo "请使用以下命令查看服务状态:"
echo "  sudo systemctl status ros-exporter"
EOF

chmod +x $DEPLOY_DIR/quick_deploy.sh

# 创建README
cat > $DEPLOY_DIR/README.txt << EOF
ROSMaster-X3 监控系统部署包
============================

此部署包包含ROSMaster-X3机器人监控系统的完整安装文件。

快速安装:
1. 将此包传输到ROSMaster-X3机器人
2. 解压: tar -xzf $PACKAGE_NAME
3. 进入目录: cd rosmaster_x3_deploy
4. 运行安装: ./quick_deploy.sh

文件说明:
- bin/ros_exporter              # 监控程序二进制文件
- config/config_rosmaster_x3.yaml # ROSMaster-X3专用配置
- scripts/install_rosmaster_x3.sh  # 完整安装脚本
- scripts/test_rosmaster_x3.sh     # 功能测试脚本
- docs/                            # 详细文档
- grafana/rosmaster-x3-dashboard.json # Grafana仪表板

支持架构: ARM64 (aarch64)
适用平台: Ubuntu 20.04 LTS + ROS Noetic
创建时间: $(date)

技术支持: 请参考docs/目录下的文档
EOF

# 创建部署包
echo "创建压缩包..."
tar -czf $PACKAGE_NAME $DEPLOY_DIR

# 显示结果
echo ""
echo "=== 部署包创建完成 ==="
echo "包文件: $PACKAGE_NAME"
echo "包大小: $(du -h $PACKAGE_NAME | cut -f1)"
echo ""
echo "传输到目标机器的命令示例:"
echo "scp $PACKAGE_NAME pi@ROBOT_IP:~/"
echo ""
echo "在目标机器上的安装命令:"
echo "tar -xzf $PACKAGE_NAME && cd rosmaster_x3_deploy && ./quick_deploy.sh"
echo ""

# 显示包内容
echo "包内容:"
tar -tzf $PACKAGE_NAME

# 清理临时目录
rm -rf $DEPLOY_DIR

echo ""
echo "部署包已准备就绪！"
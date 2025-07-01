# ros_exporter 部署目录

本目录包含了 ros_exporter 的所有部署相关文件，提供完整的编译、打包和部署解决方案。

## 📁 目录结构

```
deploy/
├── README.md                           # 本说明文档
├── deploy_to_production.sh             # 生产环境自动化部署脚本
├── ssh_config_production               # SSH配置文件
├── ros_exporter-linux-arm64  # ARM64二进制文件 (G1机器人)
├── ros_exporter-linux-amd64  # x86_64二进制文件 (开发环境)
├── config.yaml                         # 统一配置文件(支持多环境)
├── deploy_robot.sh                     # 部署脚本
├── start.sh                           # 启动脚本
├── shutdown.sh                        # 停止脚本
├── status.sh                          # 状态检查脚本
├── restart.sh                         # 重启脚本
└── *.tar.gz                          # 打包好的部署包
```

## 🎯 部署包说明

### ARM64 版本 (生产环境)
- **文件**: `ros_exporter-deployment-*.tar.gz`
- **目标**: G1机器人本体 (ARM64 Linux)
- **特性**: 
  - 真实G1 SDK集成 (CGO)
  - 40节电池监控 + 12个温度传感器
  - ROS1环境支持
  - VictoriaMetrics数据推送

### x86_64 版本 (开发环境)
- **文件**: `ros_exporter-x86-deployment-*.tar.gz`
- **目标**: 开发服务器 (x86_64 Linux)
- **特性**:
  - 模拟G1 SDK (无CGO依赖)
  - 纯Go实现
  - ROS1环境支持
  - 适合开发和测试

## 🚀 快速部署

### 方式一：自动化部署 (推荐)

```bash
# 进入deploy目录
cd deploy

# 完整自动化部署到生产环境
./deploy_to_production.sh

# 或分步骤部署
./deploy_to_production.sh setup-ssh    # 配置SSH免登录
./deploy_to_production.sh test         # 测试连接
./deploy_to_production.sh deploy       # 执行部署
```

### 方式二：手动部署

```bash
# 1. 上传部署包
scp ros_exporter-deployment-*.tar.gz robot@<your_ip>:/tmp/ # 请填写你的服务器IP

# 2. SSH连接到目标主机
ssh robot@<your_ip> # 请填写你的服务器IP

# 3. 解压并部署
cd /tmp
tar -xzf ros_exporter-deployment-*.tar.gz
sudo ./deploy_robot.sh
```

## 🔧 生产环境配置

### 目标主机信息
- **IP地址**: <your_ip> # 请填写你的服务器IP
- **操作系统**: Ubuntu Linux
- **用户名**: robot
- **密码**: 123123

### SSH免登录设置
脚本会自动配置SSH密钥认证：
- **密钥位置**: `~/.ssh/robot_production`
- **配置文件**: `ssh_config_production`

使用SSH配置文件连接：
```bash
ssh -F ssh_config_production robot-production
```

## 📊 服务管理

部署完成后，服务将以systemd服务形式运行：

```bash
# 查看服务状态
sudo systemctl status ros_exporter

# 启动服务
sudo systemctl start ros_exporter

# 停止服务
sudo systemctl stop ros_exporter

# 重启服务
sudo systemctl restart ros_exporter

# 查看日志
sudo journalctl -u ros_exporter -f
```

## 🌐 访问端点

部署成功后可以访问以下端点：

- **健康检查**: http://<your_ip>:8080/health # 请填写你的服务器IP
- **指标数据**: http://<your_ip>:8080/metrics # 请填写你的服务器IP
- **配置信息**: http://<your_ip>:8080/config # 请填写你的服务器IP

## 📈 监控数据

系统会自动将监控数据推送到：
- **VictoriaMetrics**: <your_vm_url> # 请填写你的时序数据库地址
- **推送间隔**: 10秒
- **数据类型**: Prometheus格式

### 主要监控指标

1. **系统指标**
   - CPU使用率
   - 内存使用率
   - 磁盘使用率
   - 网络流量

2. **电池指标** (G1 SDK)
   - 电池电压 (40节)
   - 电池温度 (12个传感器)
   - 充放电状态
   - 健康度评分

3. **ROS1指标**
   - 节点状态
   - Topic频率
   - 服务可用性
   - 参数监控

## 🔍 故障排除

### 常见问题

1. **网络连接问题**
   ```
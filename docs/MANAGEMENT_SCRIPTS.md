# ros_exporter 管理脚本

## 📋 脚本概述

ros_exporter提供了三个管理脚本，用于方便地管理服务的生命周期：

### 🚀 **start.sh** - 启动脚本
- **功能**: 启动ros_exporter服务
- **特点**: 自动检测架构，后台运行，记录PID

### 🔄 **restart.sh** - 重启脚本  
- **功能**: 停止当前运行的服务并重新启动
- **特点**: 优雅停止 + 自动启动，完整的状态检查

### 🛑 **shutdown.sh** - 关闭脚本
- **功能**: 安全停止ros_exporter服务
- **特点**: 优雅关闭，可选日志清理，完整的清理流程

## 🔧 使用方法

### 启动服务

```bash
# 基本启动
./start.sh

# 指定配置文件启动
./start.sh -config custom.yaml

# 指定网络接口启动
./start.sh -interfaces ens160
```

### 重启服务

```bash
# 重启服务
./restart.sh
```

**重启流程**：
1. 检查当前运行状态
2. 优雅停止现有进程
3. 等待进程完全退出
4. 启动新的服务实例
5. 验证启动成功

### 关闭服务

```bash
# 正常关闭（保留日志）
./shutdown.sh

# 关闭并清理日志
./shutdown.sh --clean-logs

# 查看帮助
./shutdown.sh --help
```

**关闭流程**：
1. 检查运行状态
2. 显示关闭前日志
3. 发送TERM信号优雅关闭
4. 等待最多30秒
5. 必要时强制终止
6. 清理PID文件和可选日志

## 📊 状态管理

### 进程管理

所有脚本都支持：
- **PID文件管理** - 使用`exporter.pid`跟踪进程
- **多重检测** - 通过PID文件和进程名双重确认
- **优雅关闭** - 发送TERM信号，等待优雅退出
- **强制终止** - 超时后使用KILL信号

### 日志管理

- **日志文件**: `exporter.log`
- **PID文件**: `exporter.pid` 
- **自动轮转**: 每次启动创建新日志
- **可选清理**: shutdown.sh支持`--clean-logs`参数

### 架构自适应

脚本自动检测系统架构：
- **x86_64**: 使用`ros_exporter-linux-amd64`
- **aarch64/arm64**: 使用`ros_exporter-linux-arm64`

## 🎨 输出样式

脚本使用彩色输出提高可读性：
- 🔵 **[INFO]** - 一般信息（蓝色）
- 🟢 **[SUCCESS]** - 成功操作（绿色）
- 🟡 **[WARNING]** - 警告信息（黄色）
- 🔴 **[ERROR]** - 错误信息（红色）

## 📝 使用示例

### 典型工作流程

```bash
# 1. 首次启动
./start.sh

# 2. 检查状态
ps aux | grep ros_exporter

# 3. 查看日志
tail -f exporter.log

# 4. 重启服务（配置更新后）
./restart.sh

# 5. 正常关闭
./shutdown.sh
```

### 故障排除

```bash
# 如果启动失败，查看详细日志
cat exporter.log

# 强制清理所有残留进程
./shutdown.sh
pkill -f ros_exporter

# 重新启动
./start.sh
```

### 服务化部署

```bash
# 创建systemd服务文件
sudo tee /etc/systemd/system/ros_exporter.service > /dev/null <<EOF
[Unit]
Description=ros_exporter
After=network.target

[Service]
Type=forking
User=dna
WorkingDirectory=/home/dna/ros_exporter-1.0.0
ExecStart=/home/dna/ros_exporter-1.0.0/start.sh
ExecReload=/home/dna/ros_exporter-1.0.0/restart.sh
ExecStop=/home/dna/ros_exporter-1.0.0/shutdown.sh
PIDFile=/home/dna/ros_exporter-1.0.0/exporter.pid
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# 启用并启动服务
sudo systemctl daemon-reload
sudo systemctl enable ros_exporter
sudo systemctl start ros_exporter

# 检查服务状态
sudo systemctl status ros_exporter
```

## 🔍 脚本特性

### restart.sh 特性

- ✅ **智能检测**: 自动检测可执行文件架构
- ✅ **状态验证**: 启动前后都进行状态检查
- ✅ **优雅重启**: 先停止再启动，避免端口冲突
- ✅ **错误处理**: 完整的错误检查和日志记录
- ✅ **进度显示**: 实时显示重启进度

### shutdown.sh 特性

- ✅ **优雅关闭**: 30秒优雅关闭窗口
- ✅ **强制终止**: 超时后自动强制终止
- ✅ **完整清理**: 清理PID文件和可选日志
- ✅ **状态验证**: 确认服务完全停止
- ✅ **日志保留**: 默认保留日志，可选清理

### 通用特性

- ✅ **彩色输出**: 清晰的状态指示
- ✅ **错误处理**: 完善的错误检查机制
- ✅ **信号处理**: 支持Ctrl+C中断
- ✅ **帮助信息**: 内置帮助和使用说明

## 🚨 注意事项

1. **权限要求**: 脚本需要执行权限 (`chmod +x *.sh`)
2. **配置文件**: 确保`config.yaml`存在且配置正确
3. **网络连接**: 确保VictoriaMetrics服务可达
4. **资源监控**: 定期检查系统资源使用情况
5. **日志轮转**: 长期运行建议配置日志轮转

## 🔗 相关文件

- `config.yaml` - 主配置文件
- `config.example.yaml` - 配置模板
- `exporter.log` - 运行日志
- `exporter.pid` - 进程ID文件
- `ros_exporter-linux-*` - 可执行文件

## 📞 故障支持

如果遇到问题：
1. 检查`exporter.log`日志文件
2. 确认配置文件语法正确
3. 验证网络连接到VictoriaMetrics
4. 检查系统资源是否充足
5. 确认可执行文件权限正确

**注意：所有管理脚本部署后与主程序同级，直接用 `./start.sh` 方式调用。** 
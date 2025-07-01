#!/bin/bash

# ros_exporter - 标准化部署脚本
# 按照企业级部署规范进行部署

set -e

# 部署配置
APP_NAME="ros_exporter"
APP_DIR="/opt/app/${APP_NAME}"
LOG_DIR="/opt/logs/${APP_NAME}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# 检查权限
check_permissions() {
    if [ "$EUID" -ne 0 ]; then
        log_error "需要root权限执行部署，请使用sudo运行"
        exit 1
    fi
    log_info "权限检查通过"
}

# 检查系统环境
check_system() {
    log_step "检查系统环境..."
    
    # 检查操作系统
    if [ ! -f /etc/os-release ]; then
        log_error "无法识别操作系统"
        exit 1
    fi
    
    OS_NAME=$(grep '^NAME=' /etc/os-release | cut -d'"' -f2)
    log_info "操作系统: $OS_NAME"
    
    # 检查架构
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) BINARY_SUFFIX="linux-amd64" ;;
        aarch64|arm64) BINARY_SUFFIX="linux-arm64" ;;
        *) log_error "不支持的架构: $ARCH"; exit 1 ;;
    esac
    
    log_info "系统架构: $ARCH"
    log_info "二进制文件后缀: $BINARY_SUFFIX"
}

# 创建目录结构
create_directories() {
    log_step "创建目录结构..."
    
    # 创建应用目录
    mkdir -p "$APP_DIR"
    mkdir -p "$LOG_DIR"
    
    # 设置目录权限
    chmod 755 "$APP_DIR"
    chmod 755 "$LOG_DIR"
    
    log_info "应用目录: $APP_DIR"
    log_info "日志目录: $LOG_DIR"
}

# 部署应用文件
deploy_application() {
    log_step "部署应用文件..."
    
    # 检查并部署二进制文件
    BINARY_NAME="${APP_NAME}-${BINARY_SUFFIX}"
    if [ ! -f "$BINARY_NAME" ]; then
        log_error "找不到二进制文件: $BINARY_NAME"
        exit 1
    fi
    
    # 复制二进制文件
    cp "$BINARY_NAME" "$APP_DIR/$APP_NAME"
    chmod 755 "$APP_DIR/$APP_NAME"
    log_info "部署二进制文件: $APP_DIR/$APP_NAME"
    
    # 部署配置文件
    if [ -f "config.yaml" ]; then
        cp "config.yaml" "$APP_DIR/config.yaml"
        chmod 644 "$APP_DIR/config.yaml"
        log_info "部署配置文件: $APP_DIR/config.yaml"
    else
        log_warn "未找到config.yaml，请手动创建配置文件"
    fi
}

# 部署管理脚本
deploy_scripts() {
    log_step "部署管理脚本..."
    
    local scripts=("start.sh" "restart.sh" "shutdown.sh" "status.sh")
    
    for script in "${scripts[@]}"; do
        if [ -f "$script" ]; then
            # 直接复制脚本文件（路径已在脚本中硬编码）
            cp "scripts/$script" "$APP_DIR/$script"
            chmod 755 "$APP_DIR/$script"
            log_info "部署脚本: $APP_DIR/$script"
        else
            log_warn "未找到脚本文件: $script"
        fi
    done
}

# 创建systemd服务
create_systemd_service() {
    log_step "创建systemd服务..."
    
    cat > "/etc/systemd/system/${APP_NAME}.service" << SERVICE_EOF
[Unit]
Description=ros_exporter
Documentation=https://github.com/your-org/ros_exporter
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/${APP_NAME} -config ${APP_DIR}/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
KillMode=process
Restart=on-failure
RestartSec=10
StandardOutput=append:${LOG_DIR}/exporter.log
StandardError=append:${LOG_DIR}/exporter.log

# 安全配置
NoNewPrivileges=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=${LOG_DIR}

# 资源限制
LimitNOFILE=65536
LimitNPROC=32768

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    # 重新加载systemd配置
    systemctl daemon-reload
    log_info "创建systemd服务: ${APP_NAME}.service"
}

# 配置日志轮转
setup_log_rotation() {
    log_step "配置日志轮转..."
    
    cat > "/etc/logrotate.d/${APP_NAME}" << LOGROTATE_EOF
${LOG_DIR}/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    copytruncate
}
LOGROTATE_EOF

    log_info "配置日志轮转: /etc/logrotate.d/${APP_NAME}"
}

# 设置防火墙规则
setup_firewall() {
    log_step "配置防火墙..."
    
    # 检查是否有防火墙服务
    if command -v ufw >/dev/null 2>&1; then
        log_info "检测到ufw防火墙，请手动配置所需端口"
    elif command -v firewall-cmd >/dev/null 2>&1; then
        log_info "检测到firewalld防火墙，请手动配置所需端口"
    elif command -v iptables >/dev/null 2>&1; then
        log_info "检测到iptables防火墙，请手动配置所需端口"
    else
        log_info "未检测到防火墙服务"
    fi
}

# 验证部署
verify_deployment() {
    log_step "验证部署..."
    
    # 检查文件完整性
    local files_ok=true
    
    if [ ! -f "$APP_DIR/$APP_NAME" ]; then
        log_error "缺少二进制文件: $APP_DIR/$APP_NAME"
        files_ok=false
    fi
    
    if [ ! -f "$APP_DIR/config.yaml" ]; then
        log_warn "缺少配置文件: $APP_DIR/config.yaml"
    fi
    
    local scripts=("start.sh" "restart.sh" "shutdown.sh" "status.sh")
    for script in "${scripts[@]}"; do
        if [ ! -f "$APP_DIR/$script" ]; then
            log_warn "缺少脚本: $APP_DIR/$script"
        fi
    done
    
    if [ ! -f "/etc/systemd/system/${APP_NAME}.service" ]; then
        log_error "缺少systemd服务文件"
        files_ok=false
    fi
    
    if [ "$files_ok" = true ]; then
        log_info "文件完整性检查通过"
    else
        log_error "文件完整性检查失败"
        exit 1
    fi
    
    # 测试配置文件
    if [ -f "$APP_DIR/config.yaml" ]; then
        log_info "配置文件存在，建议手动验证配置内容"
    fi
}

# 显示帮助信息
show_help() {
    cat << HELP_EOF
ros_exporter - 标准化部署脚本

用法: $0 [选项]

选项:
  --app-dir DIR    指定应用安装目录 (默认: /opt/app/ros_exporter)
--log-dir DIR    指定日志目录 (默认: /opt/logs/ros_exporter)
  --no-systemd     不创建systemd服务
  --no-logrotate   不配置日志轮转
  --help, -h       显示帮助信息

部署结构:
  ${APP_DIR}/                    # 应用目录
  ├── ros_exporter    # 主程序
  ├── config.yaml              # 配置文件
  ├── scripts/start.sh         # 启动脚本
  ├── scripts/restart.sh       # 重启脚本
  ├── scripts/shutdown.sh      # 停止脚本
  └── scripts/status.sh        # 状态脚本
  
  ${LOG_DIR}/                   # 日志目录
  ├── exporter.log             # 应用日志
└── exporter.pid             # 进程PID文件

系统服务:
  /etc/systemd/system/ros_exporter.service

HELP_EOF
}

# 主函数
main() {
    local create_systemd=true
    local create_logrotate=true
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --app-dir)
                APP_DIR="$2"
                shift 2
                ;;
            --log-dir)
                LOG_DIR="$2"
                shift 2
                ;;
            --no-systemd)
                create_systemd=false
                shift
                ;;
            --no-logrotate)
                create_logrotate=false
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    echo "============================================"
    echo "ros_exporter 标准化部署"
    echo "============================================"
    
    # 执行部署步骤
    check_permissions
    check_system
    create_directories
    deploy_application
    deploy_scripts
    
    if [ "$create_systemd" = true ]; then
        create_systemd_service
    fi
    
    if [ "$create_logrotate" = true ]; then
        setup_log_rotation
    fi
    
    setup_firewall
    verify_deployment
    
    echo ""
    echo "============================================"
    echo "部署完成！"
    echo "============================================"
    echo ""
    log_info "部署信息:"
    echo "  应用目录: $APP_DIR"
    echo "  日志目录: $LOG_DIR"
    echo "  运行用户: root"
    echo ""
    log_info "管理命令:"
    echo "  启动服务: systemctl start ${APP_NAME}"
    echo "  停止服务: systemctl stop ${APP_NAME}"
    echo "  重启服务: systemctl restart ${APP_NAME}"
    echo "  查看状态: systemctl status ${APP_NAME}"
    echo "  开机自启: systemctl enable ${APP_NAME}"
    echo ""
    log_info "手动管理:"
    echo "  启动: $APP_DIR/start.sh"
    echo "  状态: $APP_DIR/status.sh"
    echo "  停止: $APP_DIR/shutdown.sh"
    echo ""
    log_info "日志查看:"
    echo "  实时日志: tail -f $LOG_DIR/exporter.log"
    echo "  系统日志: journalctl -u ${APP_NAME} -f"
    echo ""
    log_warn "请检查并修改配置文件: $APP_DIR/config.yaml"
    
}

# 执行主函数
main "$@"

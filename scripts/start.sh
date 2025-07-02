#!/bin/sh

# ros_exporter 启动脚本 - 标准化部署版本

set -e

# 标准部署路径配置
EXPORTER_NAME="ros_exporter"
APP_DIR="/opt/app/ros_exporter"
LOG_DIR="/opt/logs/ros_exporter"
CONFIG_FILE="$APP_DIR/config.yaml"
LOG_FILE="$LOG_DIR/exporter.log"
PID_FILE="$LOG_DIR/exporter.pid"

# 切换到应用目录
cd "$APP_DIR"

# 简单日志函数
log_info() { echo "[INFO] $1"; }
log_success() { echo "[SUCCESS] $1"; }
log_warning() { echo "[WARNING] $1"; }
log_error() { echo "[ERROR] $1"; }

# 检查是否已经运行
check_running() {
    # 检查PID文件
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_warning "ros_exporter 已经在运行 (PID: $pid)"
            return 0
        else
            log_warning "发现过期的PID文件，清理中..."
            rm -f "$PID_FILE"
        fi
    fi
    
    # 检查进程
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_warning "发现运行中的Exporter进程: $pids"
        log_warning "请使用 './scripts/shutdown.sh' 停止现有进程，或使用 './scripts/restart.sh' 重启"
        return 0
    fi
    
    return 1
}

# 启动Exporter
start_exporter() {
    log_info "正在启动 ros_exporter..."
    
    # 检查配置文件
    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "配置文件不存在: $CONFIG_FILE"
        exit 1
    fi
    
    # 检查可执行文件权限
    if [ ! -x "$EXPORTER_NAME" ]; then
        log_info "添加执行权限到 $EXPORTER_NAME"
        chmod +x "$EXPORTER_NAME"
    fi
    
    # 确保日志目录存在
    mkdir -p "$LOG_DIR"
    
    # 启动Exporter（后台运行，与终端分离）
    log_info "启动命令: ./$EXPORTER_NAME -config $CONFIG_FILE"
    log_info "后台启动中，进程将与终端分离..."
    
    # 使用nohup启动后台进程，重定向输出到日志文件
    nohup "./$EXPORTER_NAME" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    pid=$!
    
    # 保存PID
    echo "$pid" > "$PID_FILE"
    
    # 等待一段时间检查启动是否成功
    sleep 3
    
    if kill -0 "$pid" 2>/dev/null; then
        log_success "ros_exporter 启动成功"
        log_info "进程ID: $pid"
        log_info "日志文件: $LOG_FILE"
        log_info "PID文件: $PID_FILE"
        
        # 显示最新的几行日志
        if [ -f "$LOG_FILE" ]; then
            log_info "最新日志:"
            tail -3 "$LOG_FILE" | sed 's/^/  /'
        fi
        
        echo ""
        log_info "使用以下命令管理服务:"
        log_info "  ./scripts/status.sh   - 检查状态"
        log_info "  ./scripts/shutdown.sh - 停止服务"
        log_info "  ./scripts/restart.sh  - 重启服务"
        
    else
        log_error "ros_exporter 启动失败"
        rm -f "$PID_FILE"
        
        if [ -f "$LOG_FILE" ]; then
            log_error "错误日志:"
            tail -5 "$LOG_FILE" | sed 's/^/  /'
        fi
        exit 1
    fi
}

# 主函数
main() {
    case "${1:-}" in
        --force)
            force_start=true
            ;;
        --help|-h)
            echo "ros_exporter 启动脚本"
            echo "用法: $0 [--force]"
            exit 0
            ;;
        "") ;;
        *)
            log_error "未知参数: $1"
            exit 1
            ;;
    esac
    
    echo "============================================"
    echo "ros_exporter 启动脚本"
    echo "============================================"
    
    # 检查是否已经运行
    if check_running; then
        if [ "$force_start" = "true" ]; then
            log_info "强制启动模式，停止现有进程..."
            if [ -x "./scripts/shutdown.sh" ]; then
                ./scripts/shutdown.sh
                sleep 2
            fi
        else
            exit 1
        fi
    fi
    
    # 启动服务
    start_exporter
    
    echo ""
    echo "============================================"
}

# 执行主函数
main "$@" 
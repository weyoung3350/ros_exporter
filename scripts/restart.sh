#!/bin/sh

# ros_exporter 重启脚本 - 标准化部署版本

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

# 停止导出器
stop_exporter() {
    log_info "正在停止 ros_exporter..."
    
    # 方法1: 通过PID文件停止
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_info "通过PID文件停止进程 (PID: $pid)"
            kill -TERM "$pid" 2>/dev/null || true
            
            # 等待进程退出
            count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                sleep 1
                count=$((count + 1))
            done
            
            # 如果进程仍在运行，强制杀死
            if kill -0 "$pid" 2>/dev/null; then
                log_warning "进程未能优雅退出，强制终止"
                kill -KILL "$pid" 2>/dev/null || true
            fi
            
            log_success "PID文件中的进程已停止"
        else
            log_warning "PID文件中的进程不存在，清理PID文件"
        fi
        rm -f "$PID_FILE"
    fi
    
    # 方法2: 通过进程名停止所有相关进程
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_info "发现运行中的导出器进程: $pids"
        
        for pid in $pids; do
            log_info "停止进程 PID: $pid"
            if kill -0 "$pid" 2>/dev/null; then
                kill -TERM "$pid" 2>/dev/null || true
                sleep 2
                if kill -0 "$pid" 2>/dev/null; then
                    kill -KILL "$pid" 2>/dev/null || true
                fi
            fi
        done
        
        log_success "所有导出器进程已处理完毕"
    else
        log_info "未发现运行中的导出器进程"
    fi
    
    # 确保PID文件被清理
    rm -f "$PID_FILE"
}

# 启动导出器
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
    
    # 启动导出器（后台运行）
    log_info "启动命令: ./$EXPORTER_NAME -config $CONFIG_FILE"
    
    # 启动并记录PID
    nohup "./$EXPORTER_NAME" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
    pid=$!
    
    # 保存PID
    echo "$pid" > "$PID_FILE"
    
    # 等待一段时间检查启动是否成功
    sleep 3
    
    if kill -0 "$pid" 2>/dev/null; then
        log_success "ros_exporter 启动成功 (PID: $pid)"
        log_info "日志文件: $LOG_FILE"
        log_info "PID文件: $PID_FILE"
        
        # 显示最新的几行日志
        if [ -f "$LOG_FILE" ]; then
            log_info "最新日志:"
            tail -3 "$LOG_FILE" | sed 's/^/  /'
        fi
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
    echo "============================================"
    echo "ros_exporter 重启脚本"
    echo "============================================"
    
    # 停止服务
    stop_exporter
    
    # 等待一段时间
    sleep 2
    
    # 启动服务
    start_exporter
    
    log_success "ros_exporter 重启完成!"
    echo "============================================"
}

# 执行主函数
main "$@" 
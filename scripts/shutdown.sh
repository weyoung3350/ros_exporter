#!/bin/sh

# ros_exporter 停止脚本 - 标准化部署版本

set -e

# 标准部署路径配置
EXPORTER_NAME="ros_exporter"
APP_DIR="/opt/app/ros_exporter"
LOG_DIR="/opt/logs/ros_exporter"
LOG_FILE="$LOG_DIR/exporter.log"
PID_FILE="$LOG_DIR/exporter.pid"

# 切换到应用目录
cd "$APP_DIR"

# 简单日志函数
log_info() { echo "[INFO] $1"; }
log_success() { echo "[SUCCESS] $1"; }
log_warning() { echo "[WARNING] $1"; }
log_error() { echo "[ERROR] $1"; }

# 停止Exporter进程
stop_exporter() {
    log_info "正在停止 ros_exporter..."
    
    stopped_any=false
    
    # 方法1: 通过PID文件停止
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_info "通过PID文件停止进程 (PID: $pid)"
            kill -TERM "$pid" 2>/dev/null || true
            
            # 等待进程优雅退出
            count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 15 ]; do
                sleep 1
                count=$((count + 1))
                printf "."
            done
            echo ""
            
            # 如果进程仍在运行，强制终止
            if kill -0 "$pid" 2>/dev/null; then
                log_warning "进程未能优雅退出，强制终止"
                kill -KILL "$pid" 2>/dev/null || true
                sleep 1
            fi
            
            # 验证进程已停止
            if ! kill -0 "$pid" 2>/dev/null; then
                log_success "PID文件中的进程已停止"
                stopped_any=true
            else
                log_error "无法停止PID文件中的进程"
            fi
        else
            log_warning "PID文件中的进程不存在，清理PID文件"
        fi
        rm -f "$PID_FILE"
    fi
    
    # 方法2: 通过进程名停止所有相关进程
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_info "发现运行中的Exporter进程: $pids"
        
        # 逐个停止进程
        for pid in $pids; do
            if kill -0 "$pid" 2>/dev/null; then
                log_info "停止进程 PID: $pid"
                kill -TERM "$pid" 2>/dev/null || true
                
                # 等待单个进程退出
                count=0
                while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
                    sleep 1
                    count=$((count + 1))
                done
                
                # 如果进程仍在运行，强制杀死
                if kill -0 "$pid" 2>/dev/null; then
                    log_warning "强制终止进程 PID: $pid"
                    kill -KILL "$pid" 2>/dev/null || true
                    sleep 1
                fi
                
                # 验证进程已终止
                if ! kill -0 "$pid" 2>/dev/null; then
                    log_success "进程 PID: $pid 已停止"
                    stopped_any=true
                else
                    log_error "无法停止进程 PID: $pid"
                fi
            fi
        done
    else
        log_info "未发现运行中的Exporter进程"
        if [ "$stopped_any" = false ]; then
            log_info "ros_exporter 已经处于停止状态"
        fi
    fi
    
    # 最终清理检查
    sleep 1
    final_check=$(pgrep -f "ros_exporter" 2>/dev/null || true)
    if [ -n "$final_check" ]; then
        log_warning "发现遗漏的进程，强制清理: $final_check"
        for pid in $final_check; do
            kill -KILL "$pid" 2>/dev/null || true
        done
        sleep 1
        stopped_any=true
    fi
    
    # 确保PID文件被清理
    rm -f "$PID_FILE"
    
    if [ "$stopped_any" = true ]; then
        log_success "ros_exporter 已完全停止"
    else
        log_info "ros_exporter 已经处于停止状态"
    fi
}

# 显示进程状态
show_status() {
    pids=$(pgrep -f "ros_exporter" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_info "当前运行的相关进程:"
        ps aux | head -1
        for pid in $pids; do
            ps aux | grep "$pid" | grep -v grep || true
        done
    else
        log_info "没有发现运行中的相关进程"
    fi
}

# 主函数
main() {
    force_kill=false
    show_status_only=false
    
    case "${1:-}" in
        --force)
            force_kill=true
            ;;
        --status)
            show_status_only=true
            ;;
        --help|-h)
            echo "ros_exporter 停止脚本"
            echo "用法: $0 [--force|--status]"
            exit 0
            ;;
        "") ;;
        *)
            log_error "未知参数: $1"
            exit 1
            ;;
    esac
    
    echo "============================================"
    echo "ros_exporter 停止脚本"
    echo "============================================"
    
    if [ "$show_status_only" = true ]; then
        show_status
        exit 0
    fi
    
    # 显示当前状态
    show_status
    
    # 停止服务
    stop_exporter
    
    # 显示最终状态
    echo ""
    log_info "最终状态检查:"
    show_status
    
    echo ""
    echo "============================================"
}

# 执行主函数
main "$@" 
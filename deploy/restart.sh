#!/bin/sh

# ros_exporter 重启脚本
# 兼容 bash 和 sh

set -e

# 配置
EXPORTER_NAME="ros_exporter"
CONFIG_FILE="config.yaml"
LOG_FILE="exporter.log"
PID_FILE="exporter.pid"

# 获取脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
cd "$SCRIPT_DIR"

# 简单日志函数
log_info() {
    echo "[INFO] $1"
}

log_success() {
    echo "[SUCCESS] $1"
}

log_warning() {
    echo "[WARNING] $1"
}

log_error() {
    echo "[ERROR] $1"
}

# 检测可执行文件
detect_executable() {
    arch=$(uname -m)
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    
    case "$arch" in
        x86_64)
            EXECUTABLE="${EXPORTER_NAME}-${os}-amd64"
            ;;
        aarch64|arm64)
            EXECUTABLE="${EXPORTER_NAME}-${os}-arm64"
            ;;
        *)
            log_error "不支持的架构: $arch"
            return 1
            ;;
    esac
    
    if [ ! -f "$EXECUTABLE" ]; then
        log_error "可执行文件不存在: $EXECUTABLE"
        return 1
    fi
    
    log_info "使用可执行文件: $EXECUTABLE"
    return 0
}

# 停止Exporter
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
        log_info "发现运行中的Exporter进程: $pids"
        
        # 逐个停止进程
        for pid in $pids; do
            log_info "停止进程 PID: $pid"
            if kill -0 "$pid" 2>/dev/null; then
                kill -TERM "$pid" 2>/dev/null || true
                
                # 等待单个进程退出
                count=0
                while kill -0 "$pid" 2>/dev/null && [ $count -lt 15 ]; do
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
                else
                    log_error "无法停止进程 PID: $pid"
                fi
            fi
        done
        
        log_success "所有Exporter进程已处理完毕"
    else
        log_info "未发现运行中的Exporter进程"
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
    fi
    
    # 确保PID文件被清理
    rm -f "$PID_FILE"
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
    if [ ! -x "$EXECUTABLE" ]; then
        log_info "添加执行权限到 $EXECUTABLE"
        chmod +x "$EXECUTABLE"
    fi
    
    # 启动Exporter（后台运行）
    log_info "启动命令: ./$EXECUTABLE -config $CONFIG_FILE"
    
    # 启动并记录PID
    nohup "./$EXECUTABLE" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
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

    # 检查Exporter状态
check_status() {
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_info "ros_exporter 运行状态:"
        ps aux | head -1
        for pid in $pids; do
            ps aux | grep "$pid" | grep -v grep || true
        done
        return 0
    else
        log_warning "ros_exporter 未运行"
        return 1
    fi
}

# 主函数
main() {
    echo "============================================"
    echo "ros_exporter 重启脚本"
    echo "============================================"
    
    # 检测可执行文件
    if ! detect_executable; then
        exit 1
    fi
    
    # 检查当前状态
    echo ""
    log_info "检查当前状态..."
    if check_status; then
        echo ""
        log_info "准备重启服务..."
    else
        echo ""
        log_info "服务未运行，准备启动..."
    fi
    
    # 停止服务
    echo ""
    stop_exporter
    
    # 等待一段时间
    sleep 2
    
    # 启动服务
    echo ""
    start_exporter
    
    # 最终状态检查
    echo ""
    log_info "重启完成，最终状态:"
    check_status
    
    echo ""
    log_success "ros_exporter 重启完成!"
    echo "============================================"
}

# 执行主函数
main "$@" 
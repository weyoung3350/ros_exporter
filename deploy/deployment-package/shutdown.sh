#!/bin/sh

# ros_exporter 停止脚本
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
                
                # 如果进程仍在运行，强制终止
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
    fi
    
    # 方法3: 使用systemctl停止服务（如果作为系统服务运行）
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl is-active --quiet ros_exporter 2>/dev/null; then
            log_info "停止systemd服务..."
            systemctl stop ros_exporter 2>/dev/null || true
            stopped_any=true
            log_success "systemd服务已停止"
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
    
    if [ "$stopped_any" = "true" ]; then
        log_success "ros_exporter 已停止"
    else
        log_info "未发现运行中的ros_exporter进程"
    fi
}

# 检查进程状态
check_status() {
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        return 0  # 有进程运行
    else
        return 1  # 没有进程运行
    fi
}

# 显示进程信息
show_processes() {
            pids=$(pgrep -f "ros_exporter" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo ""
        echo "当前运行的相关进程:"
        ps aux | head -1
        for pid in $pids; do
            ps aux | grep "$pid" | grep -v grep || true
        done
    fi
}

# 显示帮助
show_help() {
    echo "ros_exporter 停止脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --force      强制停止所有相关进程"
    echo "  --check      仅检查进程状态"
    echo "  --help, -h   显示此帮助信息"
    echo ""
    echo "说明:"
    echo "  此脚本会优雅地停止ros_exporter"
    echo "  首先发送SIGTERM信号，等待进程自行退出"
    echo "  如果进程未响应，则使用SIGKILL强制终止"
    echo ""
    echo "示例:"
    echo "  $0           # 正常停止"
    echo "  $0 --force   # 强制停止所有相关进程"
    echo "  $0 --check   # 仅检查状态"
}

# 主函数
main() {
    force_stop=false
    check_only=false
    
    # 解析命令行参数
    case "${1:-}" in
        --force)
            force_stop=true
            ;;
        --check)
            check_only=true
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        "")
            # 正常停止
            ;;
        *)
            log_error "未知参数: $1"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
    
    echo "============================================"
    echo "ros_exporter 停止脚本"
    echo "============================================"
    
    # 检查当前状态
    if check_status; then
        log_info "ros_exporter 正在运行"
        show_processes
        
        if [ "$check_only" = "true" ]; then
            echo ""
            log_info "仅检查模式，不执行停止操作"
            exit 0
        fi
        
        echo ""
        if [ "$force_stop" = "true" ]; then
            log_info "强制停止模式"
        else
            log_info "开始停止操作..."
        fi
        
        # 执行停止
        stop_exporter
        
        # 验证停止结果
        echo ""
        if check_status; then
            log_error "停止失败，仍有进程在运行"
            show_processes
            echo ""
            log_info "建议使用 '$0 --force' 强制停止"
            exit 1
        else
            log_success "所有ros_exporter进程已停止"
        fi
        
    else
        log_info "ros_exporter 未运行"
        
        # 清理残留文件
        if [ -f "$PID_FILE" ]; then
            log_info "清理残留的PID文件"
            rm -f "$PID_FILE"
        fi
        
        if [ "$check_only" = "true" ]; then
            echo ""
            log_info "状态检查完成"
        fi
    fi
    
    echo ""
    echo "============================================"
    echo "停止操作完成"
    echo "============================================"
}

# 执行主函数
main "$@" 
#!/bin/sh

# ros_exporter 状态检查脚本
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

# 检查进程状态
check_process() {
    echo ""
    echo "=== 进程状态检查 ==="
    
    # 查找exporter进程
    pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    
    if [ -n "$pids" ]; then
        log_success "ros_exporter 正在运行"
        echo ""
        echo "运行中的进程:"
        ps aux | head -1
        for pid in $pids; do
            ps aux | grep "$pid" | grep -v grep || true
        done
        
        # 检查进程数量
        process_count=$(echo "$pids" | wc -w)
        if [ "$process_count" -gt 1 ]; then
            log_warning "检测到 $process_count 个Exporter进程运行"
            log_warning "可能导致端口冲突，建议运行 './restart.sh'"
        fi
        
        return 0
    else
        log_error "ros_exporter 未运行"
        
        # 检查PID文件
        if [ -f "$PID_FILE" ]; then
            stored_pid=$(cat "$PID_FILE")
            log_warning "发现PID文件残留 (PID: $stored_pid)"
        fi
        
        return 1
    fi
}

# 检查配置文件
check_config() {
    echo ""
    echo "=== 配置文件检查 ==="
    
    if [ -f "$CONFIG_FILE" ]; then
        log_success "配置文件存在: $CONFIG_FILE"
        echo ""
        echo "配置文件内容 (前5行):"
        head -5 "$CONFIG_FILE" | sed 's/^/  /'
    else
        log_error "配置文件不存在: $CONFIG_FILE"
        return 1
    fi
}

# 检查日志文件
check_logs() {
    echo ""
    echo "=== 日志文件检查 ==="
    
    if [ -f "$LOG_FILE" ]; then
        log_size=$(du -h "$LOG_FILE" | cut -f1)
        log_lines=$(wc -l < "$LOG_FILE")
        log_success "日志文件存在: $LOG_FILE (大小: $log_size, 行数: $log_lines)"
        
        echo ""
        echo "最新日志 (最后5行):"
        echo "----------------------------------------"
        tail -5 "$LOG_FILE" | sed 's/^/  /'
        echo "----------------------------------------"
        
        # 简单错误检查
        error_count=$(grep -i "error\|failed\|panic" "$LOG_FILE" 2>/dev/null | wc -l || echo "0")
        if [ "$error_count" -gt 0 ]; then
            log_warning "发现 $error_count 个错误日志条目"
        else
            log_success "未发现错误日志"
        fi
    else
        log_warning "日志文件不存在: $LOG_FILE"
    fi
}

# 检查网络连接 (已移除硬编码检查)
check_network() {
    echo ""
    echo "=== 网络连接检查 ==="
    
    log_info "网络连接检查已禁用"
    log_info "VictoriaMetrics连接状态请查看应用日志"
}

# 检查系统资源
check_system() {
    echo ""
    echo "=== 系统资源检查 ==="
    
    # 内存使用
    if command -v free >/dev/null 2>&1; then
        echo "内存使用情况:"
        free -h | sed 's/^/  /'
    fi
    
    # 磁盘使用
    echo ""
    echo "磁盘使用情况:"
    df -h . | sed 's/^/  /'
    
    # 系统负载
    if [ -f /proc/loadavg ]; then
        load_avg=$(cat /proc/loadavg | awk '{print $1, $2, $3}')
        echo ""
        echo "系统负载: $load_avg (1min 5min 15min)"
    fi
}

# 显示帮助
show_help() {
    echo "ros_exporter 状态检查脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --process    仅检查进程状态"
    echo "  --config     仅检查配置文件"
    echo "  --logs       仅检查日志文件"
    echo "  --network    仅检查网络连接"
    echo "  --system     仅检查系统资源"
    echo "  --help, -h   显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0           # 完整状态检查"
    echo "  $0 --process # 仅检查进程状态"
}

# 主函数
main() {
    case "${1:-}" in
        --process)
            check_process
            ;;
        --config)
            check_config
            ;;
        --logs)
            check_logs
            ;;
        --network)
            check_network
            ;;
        --system)
            check_system
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        "")
            # 完整检查
            echo "============================================"
            echo "ros_exporter 状态检查"
            echo "检查时间: $(date)"
            echo "============================================"
            
            check_process
            check_config
            check_logs
            check_network
            check_system
            
            echo ""
            echo "============================================"
            echo "状态检查完成"
            echo "============================================"
            ;;
        *)
            log_error "未知参数: $1"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@" 
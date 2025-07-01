#!/bin/sh

# ros_exporter 状态检查脚本 - 标准化部署版本

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

# 检查HTTP endpoints
check_http_endpoints() {
    log_info "检查HTTP服务端点..."
    
    # 默认端口9100，从配置文件获取
    port=9100
    if [ -f "$CONFIG_FILE" ]; then
        config_port=$(grep -E '^\s*port:' "$CONFIG_FILE" | awk '{print $2}' | tr -d '"' 2>/dev/null || echo "9100")
        if [ -n "$config_port" ] && [ "$config_port" -ne 0 ] 2>/dev/null; then
            port=$config_port
        fi
    fi
    
    # 检查健康状态
    if command -v curl >/dev/null 2>&1; then
        echo "  健康检查: curl http://localhost:$port/health"
        if curl -s --connect-timeout 3 "http://localhost:$port/health" >/dev/null 2>&1; then
            log_success "HTTP health endpoint 响应正常"
        else
            log_warning "HTTP health endpoint 无响应"
        fi
        
        echo "  状态查询: curl http://localhost:$port/status"
        if curl -s --connect-timeout 3 "http://localhost:$port/status" >/dev/null 2>&1; then
            log_success "HTTP status endpoint 响应正常"
        else
            log_warning "HTTP status endpoint 无响应"
        fi
    else
        log_warning "curl命令不可用，跳过HTTP检查"
    fi
}

# 检查进程状态
check_process_status() {
    log_info "检查进程状态..."
    
    running_pids=""
    
    # 通过PID文件检查
    if [ -f "$PID_FILE" ]; then
        pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_success "PID文件中的进程正在运行 (PID: $pid)"
            running_pids="$pid"
        else
            log_warning "PID文件存在但进程不在运行，PID文件可能过期"
        fi
    else
        log_info "PID文件不存在: $PID_FILE"
    fi
    
    # 通过进程名检查
    all_pids=$(pgrep -f "ros_exporter.*-config" 2>/dev/null || true)
    if [ -n "$all_pids" ]; then
        log_info "发现相关进程: $all_pids"
        for pid in $all_pids; do
            if [ "$pid" != "$running_pids" ]; then
                log_warning "发现PID文件外的进程: $pid"
            fi
        done
        running_pids="$all_pids"
    fi
    
    if [ -z "$running_pids" ]; then
        log_warning "未发现运行中的ros_exporter进程"
        return 1
    else
        log_success "ros_exporter 正在运行"
        return 0
    fi
}

# 显示进程详情
show_process_details() {
    pids=$(pgrep -f "ros_exporter" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo ""
        log_info "进程详情:"
        printf "%-8s %-8s %-8s %-8s %s\n" "PID" "PPID" "CPU%" "MEM%" "COMMAND"
        printf "%-8s %-8s %-8s %-8s %s\n" "---" "----" "----" "----" "-------"
        
        for pid in $pids; do
            if kill -0 "$pid" 2>/dev/null; then
                ps -o pid,ppid,pcpu,pmem,args -p "$pid" | tail -n +2
            fi
        done
    fi
}

# 检查文件状态
check_files() {
    log_info "检查文件状态..."
    
    # 检查可执行文件
    if [ -f "$APP_DIR/$EXPORTER_NAME" ]; then
        if [ -x "$APP_DIR/$EXPORTER_NAME" ]; then
            log_success "可执行文件存在且可执行: $APP_DIR/$EXPORTER_NAME"
        else
            log_warning "可执行文件存在但不可执行: $APP_DIR/$EXPORTER_NAME"
        fi
    else
        log_error "可执行文件不存在: $APP_DIR/$EXPORTER_NAME"
    fi
    
    # 检查配置文件
    if [ -f "$CONFIG_FILE" ]; then
        log_success "配置文件存在: $CONFIG_FILE"
    else
        log_error "配置文件不存在: $CONFIG_FILE"
    fi
    
    # 检查日志文件
    if [ -f "$LOG_FILE" ]; then
        log_size=$(wc -c < "$LOG_FILE" 2>/dev/null || echo "unknown")
        log_success "日志文件存在: $LOG_FILE (大小: $log_size bytes)"
    else
        log_info "日志文件不存在: $LOG_FILE"
    fi
    
    # 检查日志目录权限
    if [ -d "$LOG_DIR" ]; then
        if [ -w "$LOG_DIR" ]; then
            log_success "日志目录可写: $LOG_DIR"
        else
            log_warning "日志目录不可写: $LOG_DIR"
        fi
    else
        log_warning "日志目录不存在: $LOG_DIR"
    fi
}

# 显示最新日志
show_recent_logs() {
    if [ -f "$LOG_FILE" ]; then
        echo ""
        log_info "最新日志 (最后10行):"
        echo "----------------------------------------"
        tail -10 "$LOG_FILE" | sed 's/^/  /'
        echo "----------------------------------------"
    else
        log_info "日志文件不存在，无法显示日志"
    fi
}

# 显示系统信息
show_system_info() {
    echo ""
    log_info "系统信息:"
    echo "  主机名: $(hostname)"
    echo "  系统: $(uname -s) $(uname -r)"
    echo "  架构: $(uname -m)"
    echo "  当前时间: $(date)"
    echo "  运行时长: $(uptime | awk -F'up ' '{print $2}' | awk -F',' '{print $1}')"
}

# 主函数
main() {
    show_logs=false
    show_system=false
    check_http=false
    
    case "${1:-}" in
        --logs)
            show_logs=true
            ;;
        --system)
            show_system=true
            ;;
        --http)
            check_http=true
            ;;
        --all)
            show_logs=true
            show_system=true
            check_http=true
            ;;
        --help|-h)
            echo "ros_exporter 状态检查脚本"
            echo "用法: $0 [--logs|--system|--http|--all]"
            echo ""
            echo "选项:"
            echo "  --logs      显示最新日志"
            echo "  --system    显示系统信息"
            echo "  --http      检查HTTP endpoints"
            echo "  --all       显示所有信息"
            exit 0
            ;;
        "") ;;
        *)
            log_error "未知参数: $1"
            exit 1
            ;;
    esac
    
    echo "============================================"
    echo "ros_exporter 状态检查"
    echo "============================================"
    
    # 基础检查
    check_files
    echo ""
    
    # 进程状态检查
    if check_process_status; then
        show_process_details
        service_status=0
    else
        service_status=1
    fi
    
    # HTTP检查
    if [ "$check_http" = true ]; then
        echo ""
        check_http_endpoints
    fi
    
    # 显示日志
    if [ "$show_logs" = true ]; then
        show_recent_logs
    fi
    
    # 显示系统信息
    if [ "$show_system" = true ]; then
        show_system_info
    fi
    
    echo ""
    echo "============================================"
    
    if [ $service_status -eq 0 ]; then
        log_success "ros_exporter 运行正常"
        echo ""
        log_info "管理命令:"
        echo "  ./scripts/shutdown.sh  - 停止服务"
        echo "  ./scripts/restart.sh   - 重启服务"
        echo "  ./scripts/status.sh --all - 显示详细状态"
        echo "  ./scripts/start.sh     - 启动服务"
    else
        log_warning "ros_exporter 未运行"
        echo ""
        log_info "启动命令:"
        echo "  ./scripts/start.sh     - 启动服务"
        echo "  systemctl start ros_exporter  - 通过systemd启动"
    fi
    
    echo "============================================"
    
    exit $service_status
}

# 执行主函数
main "$@" 
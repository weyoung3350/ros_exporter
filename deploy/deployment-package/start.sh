#!/bin/sh

# ros_exporter 启动脚本
# 兼容 bash 和 sh - 支持后台运行

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
        log_warning "请使用 './shutdown.sh' 停止现有进程，或使用 './restart.sh' 重启"
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
        log_info "请确保配置文件存在，或使用 config.example.yaml 作为模板"
        exit 1
    fi
    
    # 检查可执行文件权限
    if [ ! -x "$EXECUTABLE" ]; then
        log_info "添加执行权限到 $EXECUTABLE"
        chmod +x "$EXECUTABLE"
    fi
    
    # 启动Exporter（后台运行，与终端分离）
    log_info "启动命令: ./$EXECUTABLE -config $CONFIG_FILE"
    log_info "后台启动中，进程将与终端分离..."
    
    # 使用nohup启动后台进程，重定向输出到日志文件
    nohup "./$EXECUTABLE" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
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
        log_info "进程已与终端分离，可以安全关闭终端"
        
        # 显示最新的几行日志
        if [ -f "$LOG_FILE" ]; then
            log_info "最新日志:"
            tail -3 "$LOG_FILE" | sed 's/^/  /'
        fi
        
        echo ""
        log_info "使用以下命令管理服务:"
        log_info "  ./status.sh   - 检查状态"
        log_info "  ./shutdown.sh - 停止服务"
        log_info "  ./restart.sh  - 重启服务"
        
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

# 显示帮助
show_help() {
    echo "ros_exporter 启动脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --force      强制启动（停止现有进程）"
    echo "  --help, -h   显示此帮助信息"
    echo ""
    echo "说明:"
            echo "  此脚本将以后台daemon方式启动Exporter"
    echo "  进程将与终端分离，关闭终端不会影响运行"
    echo ""
    echo "示例:"
    echo "  $0           # 正常启动"
    echo "  $0 --force   # 强制启动（先停止现有进程）"
}

# 主函数
main() {
    force_start=false
    
    # 解析命令行参数
    case "${1:-}" in
        --force)
            force_start=true
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        "")
            # 正常启动
            ;;
        *)
            log_error "未知参数: $1"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
    
    echo "============================================"
    echo "ros_exporter 启动脚本"
    echo "============================================"
    
    # 检测可执行文件
    if ! detect_executable; then
        exit 1
    fi
    
    # 检查是否已经运行
    echo ""
    if check_running; then
        if [ "$force_start" = "true" ]; then
            log_info "强制启动模式，停止现有进程..."
            if command -v ./shutdown.sh >/dev/null 2>&1; then
                ./shutdown.sh
                sleep 2
            else
                log_warning "shutdown.sh不存在，手动清理进程..."
                pids=$(pgrep -f "ros_exporter" 2>/dev/null || true)
                if [ -n "$pids" ]; then
                    for pid in $pids; do
                        kill -TERM "$pid" 2>/dev/null || true
                    done
                    sleep 2
                    # 强制杀死残留进程
                    remaining=$(pgrep -f "ros_exporter" 2>/dev/null || true)
                    if [ -n "$remaining" ]; then
                        for pid in $remaining; do
                            kill -KILL "$pid" 2>/dev/null || true
                        done
                    fi
                fi
                rm -f "$PID_FILE"
            fi
        else
            echo ""
            log_info "如果要重新启动，请使用:"
            log_info "  ./restart.sh    - 重启服务"
            log_info "  $0 --force      - 强制启动"
            exit 1
        fi
    fi
    
    # 启动服务
    echo ""
    start_exporter
    
    echo ""
    echo "============================================"
}

# 执行主函数
main "$@" 
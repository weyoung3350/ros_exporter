#!/bin/bash

# ros_exporter 构建脚本
# 用于编译、打包和部署Exporter

set -e

APP_NAME="ros_exporter"
VERSION="1.0.0"
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# 显示帮助信息
show_help() {
    cat << EOF
ros_exporter 构建脚本

用法: $0 [选项]

选项:
  build         编译应用程序
  clean         清理构建文件
  test          运行测试
  package       打包发布版本
  install       安装到系统
  docker        构建Docker镜像
  help          显示此帮助信息

示例:
  $0 build                # 编译应用程序
  $0 package              # 打包发布版本
  $0 docker               # 构建Docker镜像

EOF
}

# 检查Go环境
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go未安装或不在PATH中"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}')
    log_info "使用Go版本: $GO_VERSION"
}

# 清理构建文件
clean() {
    log_step "清理构建文件..."
    rm -f $APP_NAME
    rm -rf dist/
    rm -f *.log
    
    # 使用专用清理脚本清理临时文件
    if [ -x "./scripts/clean-tmp.sh" ]; then
        ./scripts/clean-tmp.sh -f build
    fi
    
    log_info "清理完成"
}

# 编译应用程序
build() {
    log_step "编译 $APP_NAME..."
    
    # 设置构建变量
    LDFLAGS="-X main.Version=$VERSION -X 'main.BuildTime=$BUILD_TIME' -X main.GitCommit=$GIT_COMMIT"
    
    # 编译
    go build -ldflags "$LDFLAGS" -o $APP_NAME main.go
    
    if [ -f $APP_NAME ]; then
        log_info "编译成功: $APP_NAME"
        # 显示文件信息
        ls -lh $APP_NAME
    else
        log_error "编译失败"
        exit 1
    fi
}

# 运行测试
test() {
    log_step "运行测试..."
    
    # 确保测试临时目录存在
    mkdir -p tmp/test
    
    # 运行测试，输出保存到临时目录
    go test -v ./... 2>&1 | tee tmp/test/test-$(date +%Y%m%d-%H%M%S).log
    
    log_info "测试完成，日志保存到 tmp/test/"
}

# 打包发布版本
package() {
    log_step "打包发布版本..."
    
    # 创建发布目录
    DIST_DIR="dist/${APP_NAME}-${VERSION}"
    mkdir -p $DIST_DIR
    
    # 编译不同平台版本
    platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64")
    
    for platform in "${platforms[@]}"; do
        platform_split=(${platform//\// })
        GOOS=${platform_split[0]}
        GOARCH=${platform_split[1]}
        
        output_name="${APP_NAME}-${GOOS}-${GOARCH}"
        if [ $GOOS = "windows" ]; then
            output_name+='.exe'
        fi
        
        log_info "编译 $GOOS/$GOARCH..."
        
        LDFLAGS="-X main.Version=$VERSION -X 'main.BuildTime=$BUILD_TIME' -X main.GitCommit=$GIT_COMMIT"
        env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -o $DIST_DIR/$output_name main.go
        
        if [ $? -ne 0 ]; then
            log_error "编译 $GOOS/$GOARCH 失败"
            exit 1
        fi
    done
    
    # 复制配置文件和文档
    cp README.md $DIST_DIR/
    
    # 创建示例配置文件
    cat > $DIST_DIR/config.example.yaml << EOF
# ros_exporter 配置示例
# 复制此文件为 config.yaml 并根据需要修改

exporter:
  push_interval: 15s
  instance: "robot-001"
  log_level: "info"
  
  # HTTP服务器配置 - 提供健康检查和状态查询接口
  http_server:
    enabled: true
    port: 9100
    address: "127.0.0.1"
    endpoints: ["health", "status", "metrics"]

victoria_metrics:
  endpoint: "http://localhost:8428/api/v1/import/prometheus"
  timeout: 30s
  extra_labels:
    job: "ros_exporter"
    environment: "production"

collectors:
  system:
    enabled: true
    collectors: ["cpu", "memory", "disk", "network", "load"]
  
  bms:
    enabled: true
    interface_type: "unitree_sdk"
  
  ros:
    enabled: true
    master_uri: "http://localhost:11311"
EOF
    
    # 创建启动脚本
    cat > $DIST_DIR/start.sh << 'EOF'
#!/bin/bash
# ros_exporter 启动脚本

APP_NAME="ros_exporter"
CONFIG_FILE="config.yaml"

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo "配置文件 $CONFIG_FILE 不存在"
    echo "请复制 config.example.yaml 为 config.yaml 并进行配置"
    exit 1
fi

# 检查可执行文件
if [ ! -f "$APP_NAME" ]; then
    # 尝试找到对应平台的可执行文件
    PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $ARCH in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
    esac
    
    APP_NAME="${APP_NAME}-${PLATFORM}-${ARCH}"
    
    if [ ! -f "$APP_NAME" ]; then
        echo "找不到可执行文件: $APP_NAME"
        exit 1
    fi
fi

echo "启动 ros_exporter..."
./$APP_NAME -config $CONFIG_FILE
EOF
    
    chmod +x $DIST_DIR/start.sh
    
    # 创建tar包
    cd dist
    tar -czf "${APP_NAME}-${VERSION}.tar.gz" "${APP_NAME}-${VERSION}"
    cd ..
    
    log_info "打包完成: dist/${APP_NAME}-${VERSION}.tar.gz"
}

# 安装到系统
install() {
    log_step "安装到系统..."
    
    if [ ! -f $APP_NAME ]; then
        log_error "可执行文件不存在，请先运行 build"
        exit 1
    fi
    
    # 安装到 /usr/local/bin
    sudo cp $APP_NAME /usr/local/bin/
    sudo chmod +x /usr/local/bin/$APP_NAME
    
    # 创建配置目录
    sudo mkdir -p /etc/ros_exporter
    
    # 如果配置文件不存在，创建示例配置
    if [ ! -f /etc/ros_exporter/config.yaml ]; then
        sudo cp config.example.yaml /etc/ros_exporter/config.yaml 2>/dev/null || true
    fi
    
    log_info "安装完成"
    log_info "可执行文件: /usr/local/bin/$APP_NAME"
    log_info "配置目录: /etc/ros_exporter/"
}

# 构建Docker镜像
docker_build() {
    log_step "构建Docker镜像..."
    
    # 确保tmp/build目录存在
    mkdir -p tmp/build
    
    # 创建Dockerfile (保存到tmp目录)
    cat > tmp/build/Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ros_exporter main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/ros_exporter .
COPY config.example.yaml config.yaml

CMD ["./ros_exporter"]
EOF
    
    # 构建镜像
    docker build -f tmp/build/Dockerfile -t ros_exporter:$VERSION .
    docker tag ros_exporter:$VERSION ros_exporter:latest
    
    # 清理临时Dockerfile
    rm -f tmp/build/Dockerfile
    
    log_info "Docker镜像构建完成"
    log_info "镜像标签: ros_exporter:$VERSION, ros_exporter:latest"
}

# 主逻辑
main() {
    check_go
    
    case "${1:-build}" in
        build)
            build
            ;;
        clean)
            clean
            ;;
        test)
            test
            ;;
        package)
            clean
            build
            package
            ;;
        install)
            build
            install
            ;;
        docker)
            docker_build
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@" 
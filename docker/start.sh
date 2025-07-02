#!/bin/bash

# ROS Exporter Docker开发环境启动脚本
# 适用于 macOS Apple 芯片

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${BLUE}🚀 启动 ROS Exporter Docker 开发环境${NC}"
echo "=================================================="

# 检查Docker是否运行
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}❌ Docker 未运行，请先启动 Docker Desktop${NC}"
    exit 1
fi

# 检查Docker Compose版本
if ! command -v docker-compose >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  未找到 docker-compose，尝试使用 docker compose${NC}"
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# 进入docker目录
cd "$SCRIPT_DIR"

echo -e "${BLUE}📋 环境信息:${NC}"
echo "  项目根目录: $PROJECT_ROOT"
echo "  Docker目录: $SCRIPT_DIR"
echo "  Docker Compose: $DOCKER_COMPOSE"
echo ""

# 检查必要文件
required_files=("docker-compose.yml" "Dockerfile" "config.yaml")
for file in "${required_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}❌ 缺少必要文件: $file${NC}"
        exit 1
    fi
done

echo -e "${GREEN}✅ 所有必要文件检查通过${NC}"
echo ""

# 清理旧容器（如果存在）
echo -e "${YELLOW}🧹 清理旧容器...${NC}"
$DOCKER_COMPOSE down --remove-orphans 2>/dev/null || true

# 构建和启动服务
echo -e "${BLUE}🔨 构建镜像...${NC}"
$DOCKER_COMPOSE build --no-cache

echo -e "${BLUE}🚀 启动服务...${NC}"
$DOCKER_COMPOSE up -d

# 等待服务启动
echo -e "${YELLOW}⏳ 等待服务启动...${NC}"
sleep 10

# 检查服务状态
echo -e "${BLUE}📊 检查服务状态:${NC}"
$DOCKER_COMPOSE ps

echo ""
echo -e "${GREEN}🎉 环境启动完成！${NC}"
echo "=================================================="
echo -e "${BLUE}📍 访问地址:${NC}"
echo "  • VictoriaMetrics:  http://localhost:8428"
echo "  • ROS Exporter:     http://localhost:9100"
echo "  • ROS Master:       http://localhost:11311"
echo ""
echo -e "${BLUE}🔧 常用命令:${NC}"
echo "  • 查看日志:         ./logs.sh"
echo "  • 停止环境:         ./stop.sh"
echo "  • 重启环境:         ./restart.sh"
echo "  • 进入容器:         docker exec -it <container_name> bash"
echo ""
echo -e "${BLUE}🧪 测试命令:${NC}"
echo "  • 健康检查:         curl http://localhost:9100/health"
echo "  • 监控指标:         curl http://localhost:9100/metrics"
echo "  • VM查询:           curl http://localhost:8428/api/v1/query?query=up"
echo "  • ROS话题列表:      docker exec ubuntu-ros1 bash -c 'source /opt/ros/noetic/setup.bash && rostopic list'"
echo ""

# 自动进行健康检查
echo -e "${YELLOW}🏥 进行健康检查...${NC}"
sleep 5

# 检查ROS Exporter
if curl -s http://localhost:9100/health >/dev/null; then
    echo -e "${GREEN}✅ ROS Exporter 健康检查通过${NC}"
else
    echo -e "${RED}❌ ROS Exporter 健康检查失败${NC}"
fi

# 检查VictoriaMetrics
if curl -s http://localhost:8428/health >/dev/null; then
    echo -e "${GREEN}✅ VictoriaMetrics 健康检查通过${NC}"
else
    echo -e "${RED}❌ VictoriaMetrics 健康检查失败${NC}"
fi

echo ""
echo -e "${GREEN}🎯 开发环境已就绪，开始愉快的开发吧！${NC}" 
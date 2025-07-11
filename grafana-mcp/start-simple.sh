#!/bin/bash

set -e

echo "🚀 启动简化版 ROS Exporter Grafana 监控环境..."

# 检查 Python 是否可用
if ! command -v python3 > /dev/null 2>&1; then
    echo "❌ Python3 未安装"
    exit 1
fi

# 检查必要文件
echo "🔍 检查配置文件..."
required_files=(
    "dashboards/ros-exporter-dashboard.json"
    "mcp-server-simple.py"
)

for file in "${required_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo "❌ 缺少文件: $file"
        exit 1
    fi
done

# 给 Python 脚本添加执行权限
chmod +x mcp-server-simple.py

echo "🔨 启动 MCP Server (Python 版本)..."
# 在后台启动 MCP Server
python3 mcp-server-simple.py &
MCP_PID=$!

# 等待 MCP Server 启动
sleep 3

# 检查 MCP Server 是否正常运行
if curl -s http://localhost:8080/health > /dev/null; then
    echo "✅ MCP Server 启动成功"
else
    echo "❌ MCP Server 启动失败"
    kill $MCP_PID 2>/dev/null || true
    exit 1
fi

echo ""
echo "✅ 简化版监控环境启动完成！"
echo ""
echo "🌐 MCP Server: http://localhost:8080"
echo "📊 API 端点："
echo "  - Dashboard: http://localhost:8080/api/dashboards"
echo "  - 数据源: http://localhost:8080/api/datasources"
echo "  - 健康检查: http://localhost:8080/health"
echo ""
echo "📝 下一步："
echo "1. 启动 VictoriaMetrics: docker run -p 8428:8428 victoriametrics/victoria-metrics"
echo "2. 启动 Grafana 并配置 MCP: http://localhost:8080"
echo "3. 确保 ros-exporter 推送数据到: http://localhost:8428/api/v1/import/prometheus"
echo ""
echo "🛑 停止 MCP Server: kill $MCP_PID"
echo "📜 MCP Server PID: $MCP_PID"

# 保存 PID 到文件
echo $MCP_PID > mcp-server.pid
echo "💾 PID 已保存到 mcp-server.pid" 
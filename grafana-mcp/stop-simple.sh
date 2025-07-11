#!/bin/bash

echo "🛑 停止简化版 MCP Server..."

# 从 PID 文件读取进程 ID
if [[ -f "mcp-server.pid" ]]; then
    MCP_PID=$(cat mcp-server.pid)
    if kill $MCP_PID 2>/dev/null; then
        echo "✅ MCP Server (PID: $MCP_PID) 已停止"
    else
        echo "⚠️  进程 $MCP_PID 可能已经停止"
    fi
    rm -f mcp-server.pid
else
    echo "⚠️  PID 文件不存在，尝试查找进程..."
    # 尝试查找并停止 Python MCP Server 进程
    pkill -f "mcp-server-simple.py" && echo "✅ MCP Server 进程已停止" || echo "⚠️  未找到运行中的 MCP Server"
fi

echo "✅ 简化版监控环境已停止" 
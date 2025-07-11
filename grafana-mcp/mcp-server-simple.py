#!/usr/bin/env python3
"""
简单的 MCP Server - 为 Grafana 提供 Dashboard 配置
不需要 Docker，可以直接运行
"""

import json
import os
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse
import logging

# 配置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class MCPHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed_path = urlparse(self.path)
        
        if parsed_path.path == '/api/dashboards':
            self.handle_dashboards()
        elif parsed_path.path == '/api/datasources':
            self.handle_datasources()
        elif parsed_path.path == '/health':
            self.handle_health()
        else:
            self.send_error(404, "Not Found")
    
    def handle_dashboards(self):
        try:
            # 读取 dashboard JSON 文件
            dashboard_path = os.path.join(os.path.dirname(__file__), 'dashboards', 'ros-exporter-dashboard.json')
            
            if not os.path.exists(dashboard_path):
                self.send_error(404, "Dashboard file not found")
                return
            
            with open(dashboard_path, 'r', encoding='utf-8') as f:
                dashboard_content = f.read()
            
            # 构造 MCP 响应
            response = {
                "dashboards": [
                    {
                        "uid": "ros-exporter-dashboard",
                        "title": "ROS Exporter 监控面板",
                        "definition": json.loads(dashboard_content)
                    }
                ]
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()
            
            self.wfile.write(json.dumps(response, ensure_ascii=False).encode('utf-8'))
            logger.info("成功返回 dashboard 配置")
            
        except Exception as e:
            logger.error(f"处理 dashboard 请求失败: {e}")
            self.send_error(500, f"Internal server error: {e}")
    
    def handle_datasources(self):
        try:
            # 配置数据源
            response = {
                "datasources": [
                    {
                        "uid": "victoria-metrics-ros-exporter",
                        "name": "VictoriaMetrics (ROS Exporter)",
                        "type": "prometheus",
                        "url": "http://localhost:8428",
                        "settings": {
                            "httpMethod": "POST",
                            "queryTimeout": "60s"
                        }
                    }
                ]
            }
            
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()
            
            self.wfile.write(json.dumps(response, ensure_ascii=False).encode('utf-8'))
            logger.info("成功返回数据源配置")
            
        except Exception as e:
            logger.error(f"处理数据源请求失败: {e}")
            self.send_error(500, f"Internal server error: {e}")
    
    def handle_health(self):
        try:
            # 检查 dashboard 文件是否存在
            dashboard_path = os.path.join(os.path.dirname(__file__), 'dashboards', 'ros-exporter-dashboard.json')
            dashboard_exists = os.path.exists(dashboard_path)
            
            response = {
                "status": "healthy" if dashboard_exists else "unhealthy",
                "dashboard_file": dashboard_exists,
                "timestamp": int(__import__('time').time())
            }
            
            status_code = 200 if dashboard_exists else 503
            self.send_response(status_code)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()
            
            self.wfile.write(json.dumps(response).encode('utf-8'))
            
        except Exception as e:
            logger.error(f"健康检查失败: {e}")
            self.send_error(500, f"Health check failed: {e}")
    
    def log_message(self, format, *args):
        # 重写日志方法，使用我们的 logger
        logger.info(f"{self.address_string()} - {format % args}")

def main():
    port = int(os.environ.get('PORT', 8080))
    
    logger.info(f"🚀 启动 MCP Server (Python 版本) 在端口 {port}")
    
    # 检查 dashboard 文件
    dashboard_path = os.path.join(os.path.dirname(__file__), 'dashboards', 'ros-exporter-dashboard.json')
    if os.path.exists(dashboard_path):
        logger.info(f"✅ 找到 dashboard 文件: {dashboard_path}")
    else:
        logger.warning(f"⚠️  dashboard 文件不存在: {dashboard_path}")
    
    # 启动服务器
    try:
        server = HTTPServer(('0.0.0.0', port), MCPHandler)
        logger.info(f"🌐 MCP Server 正在监听 http://0.0.0.0:{port}")
        logger.info("📊 API 端点:")
        logger.info(f"  - GET http://localhost:{port}/api/dashboards")
        logger.info(f"  - GET http://localhost:{port}/api/datasources")
        logger.info(f"  - GET http://localhost:{port}/health")
        
        server.serve_forever()
        
    except KeyboardInterrupt:
        logger.info("🛑 收到停止信号，关闭服务器")
        server.shutdown()
    except Exception as e:
        logger.error(f"❌ 服务器启动失败: {e}")
        exit(1)

if __name__ == '__main__':
    main() 
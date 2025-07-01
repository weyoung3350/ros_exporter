# Grafana MCP集成使用指南

## 概述

本项目已成功集成Grafana MCP（Model Context Protocol）服务器，允许在Cursor IDE中通过自然语言直接与Grafana交互，查询机器人监控数据。

## 配置信息

### MCP服务器配置
- **服务器地址**: <your_grafana_url> # 请填写你的Grafana地址
- **MCP服务器**: 官方 `mcp/grafana` Docker镜像
- **认证方式**: Service Account API Key
- **传输协议**: stdio (标准输入输出)

### 配置文件位置
```
~/.cursor/mcp.json
```

## 可用功能

通过Grafana MCP服务器，您可以：

### 1. Dashboard管理
- 搜索现有的dashboard
- 获取dashboard详细信息
- 更新或创建新的dashboard

### 2. 数据查询
- **Prometheus查询**: 执行PromQL查询获取机器人指标
- **Loki日志查询**: 查询和检索系统日志
- **数据源管理**: 列出和获取数据源信息

### 3. 监控指标
- 机器人电池状态（SOC、电压、电流、温度）
- ROS系统状态（节点数、话题数、业务话题）
- B2工业机器狗状态（速度、负载、关节温度）
- 系统资源（CPU、内存、磁盘、网络）

### 4. 告警管理
- 查看告警规则状态
- 获取联系点信息

## 使用示例

在Cursor中，您可以使用自然语言与Grafana交互：

### 基本查询示例
```
"显示机器人电池的当前状态"
"查询最近1小时的CPU使用率"
"获取ROS节点总数"
"查看系统负载情况"
"检查B2机器狗的当前速度"
```

### 高级查询示例
```
"查询robot_battery_soc_percent指标的最新值"
"获取过去24小时内的错误日志"
"显示ros_exporter dashboard的详细信息"
"查询prometheus中的所有可用指标"
```

### Dashboard相关操作
```
"搜索包含'robot'的所有dashboard"
"获取ros_exporter dashboard的JSON配置"
"列出所有可用的数据源"
```

## 配置详情

### 当前MCP配置
```json
{
  "grafana": {
    "type": "stdio",
    "command": "docker",
    "args": [
      "run", "--rm", "-i",
      "-e", "GRAFANA_URL",
      "-e", "GRAFANA_API_KEY",
      "mcp/grafana", "-t", "stdio"
    ],
    "env": {
      "GRAFANA_URL": "<your_grafana_url>", # 请填写你的Grafana地址
      "GRAFANA_API_KEY": "eyJrIjoiRmlUVVJxR3NZcFJDUVdsaFZzYmVQODlEUUVQd3FiVHQiLCJuIjoieWFuZ3d5LWN1cnNvciIsImlkIjoxfQ=="
    }
  }
}
```

### 支持的工具类别
- `search`: Dashboard搜索功能
- `datasource`: 数据源管理
- `prometheus`: Prometheus查询
- `loki`: Loki日志查询
- `alerting`: 告警管理
- `dashboard`: Dashboard管理

## 故障排除

### 常见问题

1. **连接失败**
   - 检查Grafana服务器是否运行: `curl <your_grafana_url>/api/health` # 请填写你的Grafana地址
   - 验证API密钥有效性

2. **权限错误**
   - 确保服务账户有足够的权限
   - 检查API密钥是否过期

3. **Docker相关问题**
   - 确保Docker服务运行正常
   - 验证镜像是否正确拉取: `docker images mcp/grafana`

### 重启Cursor
配置更改后需要重启Cursor IDE以使MCP配置生效。

## 安全注意事项

- API密钥已配置在环境变量中，请妥善保管
- 建议定期轮换API密钥
- 监控服务账户的使用情况

## 项目集成

此MCP集成与现有的ros_exporter完美配合：
- 直接访问已配置的监控指标
- 与现有的Grafana dashboard兼容
- 支持所有当前的数据源（Prometheus、Node Exporter等）
- 可以查询历史数据和实时状态

## 更新和维护

- MCP服务器配置文件: `~/.cursor/mcp.json`
- Docker镜像更新: `docker pull mcp/grafana:latest`
- 配置修改后需重启Cursor 
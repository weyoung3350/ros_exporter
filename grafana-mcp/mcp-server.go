package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Dashboard represents a Grafana dashboard
type Dashboard struct {
	UID        string          `json:"uid"`
	Title      string          `json:"title"`
	Definition json.RawMessage `json:"definition"`
}

// DashboardsResponse represents the MCP server response
type DashboardsResponse struct {
	Dashboards []Dashboard `json:"dashboards"`
}

// DataSource represents a Grafana data source
type DataSource struct {
	UID      string                 `json:"uid"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	URL      string                 `json:"url"`
	Settings map[string]interface{} `json:"settings"`
}

// DataSourcesResponse represents the MCP server data sources response
type DataSourcesResponse struct {
	DataSources []DataSource `json:"datasources"`
}

func main() {
	log.Println("启动 Grafana MCP Server...")

	// 设置路由
	http.HandleFunc("/api/dashboards", handleDashboards)
	http.HandleFunc("/api/datasources", handleDataSources)
	http.HandleFunc("/health", handleHealth)

	// 启动服务器
	port := ":8080"
	log.Printf("MCP Server 正在监听端口 %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// handleDashboards 处理 dashboard 请求
func handleDashboards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取 dashboard 配置文件
	dashboardPath := "/data/dashboards/ros-exporter-dashboard.json"
	dashboardJSON, err := os.ReadFile(dashboardPath)
	if err != nil {
		log.Printf("读取 dashboard 文件失败: %v", err)
		http.Error(w, "Dashboard not found", http.StatusNotFound)
		return
	}

	// 构造响应
	response := DashboardsResponse{
		Dashboards: []Dashboard{
			{
				UID:        "ros-exporter-dashboard",
				Title:      "ROS Exporter 监控面板",
				Definition: dashboardJSON,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("编码响应失败: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Println("成功返回 dashboard 配置")
}

// handleDataSources 处理数据源请求
func handleDataSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 配置 Prometheus 数据源
	response := DataSourcesResponse{
		DataSources: []DataSource{
			{
				UID:  "prometheus-ros-exporter",
				Name: "Prometheus (ROS Exporter)",
				Type: "prometheus",
				URL:  "http://victoria-metrics:8428",
				Settings: map[string]interface{}{
					"httpMethod":   "POST",
					"queryTimeout": "60s",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("编码数据源响应失败: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Println("成功返回数据源配置")
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	// 检查 dashboard 文件是否存在
	dashboardPath := "/data/dashboards/ros-exporter-dashboard.json"
	if _, err := os.Stat(dashboardPath); os.IsNotExist(err) {
		http.Error(w, "Dashboard file not found", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status": "healthy",
		"time":   filepath.Base(dashboardPath),
	}
	json.NewEncoder(w).Encode(response)
}

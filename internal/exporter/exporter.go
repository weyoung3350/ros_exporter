package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"ros_exporter/internal/client"
	"ros_exporter/internal/collectors"
	"ros_exporter/internal/config"
)

// Exporter 统一监控导出器
type Exporter struct {
	config              *config.Config
	vmClient            *client.VMClient
	systemCollector     *collectors.SystemCollector
	bmsCollector        *collectors.BMSCollector
	rosCollector        *collectors.ROSCollector
	b2Collector         *collectors.B2Collector
	rosmasterX3Collector *collectors.ROSMasterX3Collector

	// HTTP服务器
	httpServer *http.Server

	// 控制和状态
	running bool
	mu      sync.RWMutex
}

// New 创建新的Exporter
func New(cfg *config.Config) (*Exporter, error) {
	// 创建VictoriaMetrics客户端
	vmClient := client.NewVMClient(&cfg.VictoriaMetrics)

	// 创建收集器
	systemCollector := collectors.NewSystemCollector(&cfg.Collectors.System, cfg.Exporter.Instance)
	bmsCollector := collectors.NewBMSCollector(&cfg.Collectors.BMS, cfg.Exporter.Instance)
	rosCollector := collectors.NewROSCollector(&cfg.Collectors.ROS, cfg.Exporter.Instance)
	b2Collector := collectors.NewB2Collector(&cfg.Collectors.B2, cfg.Exporter.Instance)
	rosmasterX3Collector := collectors.NewROSMasterX3Collector(&cfg.Collectors.ROSMasterX3, cfg.Exporter.Instance)

	exporter := &Exporter{
		config:              cfg,
		vmClient:            vmClient,
		systemCollector:     systemCollector,
		bmsCollector:        bmsCollector,
		rosCollector:        rosCollector,
		b2Collector:         b2Collector,
		rosmasterX3Collector: rosmasterX3Collector,
		running:             false,
	}

	// 初始化HTTP服务器
	exporter.initHTTPServer()

	return exporter, nil
}

// Start 启动Exporter
func (e *Exporter) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = true
	e.mu.Unlock()

	log.Printf("Exporter启动中...")

	// 启动HTTP服务器
	e.startHTTPServer()

	// 执行健康检查
	if err := e.healthCheck(ctx); err != nil {
		log.Printf("健康检查失败: %v", err)
	}

	// 启动收集和推送循环
	ticker := time.NewTicker(e.config.Exporter.PushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("收到停止信号，退出监控循环")
			return ctx.Err()
		case <-ticker.C:
			if err := e.collectAndPush(ctx); err != nil {
				log.Printf("收集和推送指标失败: %v", err)
			}
		}
	}
}

// Stop 停止Exporter
func (e *Exporter) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	log.Printf("停止Exporter...")
	e.running = false

	// 停止HTTP服务器
	if err := e.stopHTTPServer(ctx); err != nil {
		log.Printf("停止HTTP服务器失败: %v", err)
	}

	// 关闭BMS收集器
	if err := e.bmsCollector.Close(); err != nil {
		log.Printf("关闭BMS收集器失败: %v", err)
	}

	// 关闭B2收集器
	if err := e.b2Collector.Close(); err != nil {
		log.Printf("关闭B2收集器失败: %v", err)
	}

	log.Printf("Exporter已停止")
	return nil
}

// collectAndPush 收集指标并推送到VictoriaMetrics
func (e *Exporter) collectAndPush(ctx context.Context) error {
	startTime := time.Now()
	var allMetrics []client.Metric

	// 收集系统指标
	if systemMetrics, err := e.systemCollector.Collect(ctx); err != nil {
		log.Printf("收集系统指标失败: %v", err)
	} else {
		allMetrics = append(allMetrics, systemMetrics...)
	}

	// 收集BMS指标
	if bmsMetrics, err := e.bmsCollector.Collect(ctx); err != nil {
		log.Printf("收集BMS指标失败: %v", err)
	} else {
		allMetrics = append(allMetrics, bmsMetrics...)
	}

	// 收集ROS指标
	if rosMetrics, err := e.rosCollector.Collect(ctx); err != nil {
		log.Printf("收集ROS指标失败: %v", err)
	} else {
		allMetrics = append(allMetrics, rosMetrics...)
	}

	// 收集B2指标
	if b2Metrics, err := e.b2Collector.Collect(ctx); err != nil {
		log.Printf("收集B2指标失败: %v", err)
	} else {
		allMetrics = append(allMetrics, b2Metrics...)
	}

	// 收集ROSMaster-X3指标
	if rosmasterX3Metrics, err := e.rosmasterX3Collector.Collect(ctx); err != nil {
		log.Printf("收集ROSMaster-X3指标失败: %v", err)
	} else {
		allMetrics = append(allMetrics, rosmasterX3Metrics...)
	}

	// 添加Exporter自身的指标
	exporterMetrics := e.generateExporterMetrics(startTime)
	allMetrics = append(allMetrics, exporterMetrics...)

	// 推送到VictoriaMetrics
	if len(allMetrics) > 0 {
		if err := e.vmClient.Push(ctx, allMetrics); err != nil {
			return err
		}
		log.Printf("成功推送 %d 个指标到VictoriaMetrics", len(allMetrics))

		// 添加推送成功的指标
		pushTime := time.Now()
		pushMetrics := []client.Metric{
			{
				Name:      "ros_exporter_metrics_count",
				Value:     float64(len(allMetrics)),
				Labels:    map[string]string{"instance": e.config.Exporter.Instance, "version": "1.0.0"},
				Timestamp: pushTime,
			},
			{
				Name:      "ros_exporter_push_success_total",
				Value:     1,
				Labels:    map[string]string{"instance": e.config.Exporter.Instance, "version": "1.0.0"},
				Timestamp: pushTime,
			},
		}

		// 立即推送这些指标（不等待下次循环）
		if err := e.vmClient.Push(ctx, pushMetrics); err != nil {
			log.Printf("推送Exporter性能指标失败: %v", err)
		}
	}

	return nil
}

// generateExporterMetrics 生成Exporter自身的指标
func (e *Exporter) generateExporterMetrics(startTime time.Time) []client.Metric {
	now := time.Now()
	labels := map[string]string{
		"instance": e.config.Exporter.Instance,
		"version":  "1.0.0",
	}

	var metrics []client.Metric

	// 基础状态指标
	metrics = append(metrics, client.Metric{
		Name:      "ros_exporter_up",
		Value:     1,
		Labels:    labels,
		Timestamp: now,
	})

	// 收集耗时指标
	collectionDuration := time.Since(startTime).Seconds()
	metrics = append(metrics, client.Metric{
		Name:      "ros_exporter_push_duration_seconds",
		Value:     collectionDuration,
		Labels:    labels,
		Timestamp: now,
	})

	// 最后收集时间戳
	metrics = append(metrics, client.Metric{
		Name:      "ros_exporter_last_collection_timestamp",
		Value:     float64(now.Unix()),
		Labels:    labels,
		Timestamp: now,
	})

	return metrics
}

// healthCheck 执行健康检查
func (e *Exporter) healthCheck(ctx context.Context) error {
	log.Printf("执行健康检查...")

	// 检查VictoriaMetrics连接
	if err := e.vmClient.HealthCheck(ctx); err != nil {
		return err
	}

	// 检查ROS系统（如果启用）
	if e.config.Collectors.ROS.Enabled {
		if err := e.rosCollector.HealthCheck(); err != nil {
			log.Printf("ROS健康检查警告: %v", err)
		}
	}

	log.Printf("健康检查完成")
	return nil
}

// GetStatus 获取Exporter状态
func (e *Exporter) GetStatus() ExporterStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return ExporterStatus{
		Running:  e.running,
		Instance: e.config.Exporter.Instance,
		Collectors: CollectorStatus{
			System: e.config.Collectors.System.Enabled,
			BMS:    e.config.Collectors.BMS.Enabled,
			ROS:    e.config.Collectors.ROS.Enabled,
			B2:     e.config.Collectors.B2.Enabled,
		},
		VictoriaMetrics: VMStatus{
			Endpoint: e.config.VictoriaMetrics.Endpoint,
		},
	}
}

// ExporterStatus Exporter状态
type ExporterStatus struct {
	Running         bool            `json:"running"`
	Instance        string          `json:"instance"`
	Collectors      CollectorStatus `json:"collectors"`
	VictoriaMetrics VMStatus        `json:"victoria_metrics"`
}

// CollectorStatus 收集器状态
type CollectorStatus struct {
	System bool `json:"system"`
	BMS    bool `json:"bms"`
	ROS    bool `json:"ros"`
	B2     bool `json:"b2"`
}

// VMStatus VictoriaMetrics状态
type VMStatus struct {
	Endpoint string `json:"endpoint"`
}

// initHTTPServer 初始化HTTP服务器
func (e *Exporter) initHTTPServer() {
	if !e.config.Exporter.HTTPServer.Enabled {
		return
	}

	mux := http.NewServeMux()

	// 注册enabled的endpoints
	for _, endpoint := range e.config.Exporter.HTTPServer.Endpoints {
		switch endpoint {
		case "health":
			mux.HandleFunc("/health", e.handleHealth)
		case "status":
			mux.HandleFunc("/status", e.handleStatus)
		case "metrics":
			mux.HandleFunc("/metrics", e.handleMetrics)
		}
	}

	// 根路径重定向到status
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/status", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	addr := fmt.Sprintf("%s:%d", e.config.Exporter.HTTPServer.Address, e.config.Exporter.HTTPServer.Port)
	e.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

// startHTTPServer 启动HTTP服务器
func (e *Exporter) startHTTPServer() {
	if e.httpServer == nil {
		return
	}

	go func() {
		log.Printf("HTTP服务器启动: http://%s", e.httpServer.Addr)
		if err := e.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器运行错误: %v", err)
		}
	}()
}

// stopHTTPServer 停止HTTP服务器
func (e *Exporter) stopHTTPServer(ctx context.Context) error {
	if e.httpServer == nil {
		return nil
	}

	log.Printf("正在停止HTTP服务器...")
	if err := e.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("停止HTTP服务器失败: %w", err)
	}
	log.Printf("HTTP服务器已停止")
	return nil
}

// handleHealth 健康检查handler
func (e *Exporter) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	e.mu.RLock()
	running := e.running
	e.mu.RUnlock()

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"running":   running,
	}

	// 简单的健康检查 - 检查VictoriaMetrics连接
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := e.vmClient.HealthCheck(ctx); err != nil {
		health["status"] = "unhealthy"
		health["error"] = fmt.Sprintf("VictoriaMetrics连接失败: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleStatus 状态查询handler
func (e *Exporter) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := e.GetStatus()

	// 添加额外的运行时信息
	response := map[string]interface{}{
		"exporter":       status,
		"version":        "1.0.0",
		"timestamp":      time.Now().Unix(),
		"uptime_seconds": time.Since(time.Now()).Seconds(), // 简化实现
		"endpoints":      e.config.Exporter.HTTPServer.Endpoints,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMetrics 指标预览handler
func (e *Exporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 收集当前指标快照
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var allMetrics []client.Metric

	// 收集系统指标
	if systemMetrics, err := e.systemCollector.Collect(ctx); err == nil {
		allMetrics = append(allMetrics, systemMetrics...)
	}

	// 收集BMS指标
	if bmsMetrics, err := e.bmsCollector.Collect(ctx); err == nil {
		allMetrics = append(allMetrics, bmsMetrics...)
	}

	// 收集ROS指标
	if rosMetrics, err := e.rosCollector.Collect(ctx); err == nil {
		allMetrics = append(allMetrics, rosMetrics...)
	}

	// 收集B2指标
	if b2Metrics, err := e.b2Collector.Collect(ctx); err == nil {
		allMetrics = append(allMetrics, b2Metrics...)
	}

	// 根据Accept header决定返回格式
	accept := r.Header.Get("Accept")

	if strings.Contains(accept, "text/plain") {
		// 返回Prometheus格式
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, e.vmClient.FormatPrometheusText(allMetrics))
	} else {
		// 返回JSON格式
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"metrics":   allMetrics,
			"count":     len(allMetrics),
			"timestamp": time.Now().Unix(),
		}
		json.NewEncoder(w).Encode(response)
	}
}

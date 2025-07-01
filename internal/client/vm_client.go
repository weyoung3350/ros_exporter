package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"ros_exporter/internal/config"
)

// Metric 表示一个指标
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

// VMClient VictoriaMetrics推送客户端
type VMClient struct {
	config     *config.VictoriaMetricsConfig
	httpClient *http.Client
}

// NewVMClient 创建新的VictoriaMetrics客户端
func NewVMClient(cfg *config.VictoriaMetricsConfig) *VMClient {
	return &VMClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Push 推送指标数据到VictoriaMetrics
func (c *VMClient) Push(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// 转换为Prometheus文本格式
	payload := c.FormatPrometheusText(metrics)

	// 执行推送，带重试机制
	return c.pushWithRetry(ctx, payload)
}

// pushWithRetry 带重试机制的推送
func (c *VMClient) pushWithRetry(ctx context.Context, payload string) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			// 计算退避延迟
			delay := time.Duration(float64(c.config.Retry.RetryDelay) * math.Pow(c.config.Retry.BackoffRate, float64(attempt-1)))
			if delay > c.config.Retry.MaxDelay {
				delay = c.config.Retry.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := c.doPush(ctx, payload); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("推送失败，已重试%d次: %w", c.config.Retry.MaxRetries, lastErr)
}

// doPush 执行单次推送
func (c *VMClient) doPush(ctx context.Context, payload string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Endpoint, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("User-Agent", "ros_exporter/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// FormatPrometheusText 将指标转换为Prometheus文本格式
func (c *VMClient) FormatPrometheusText(metrics []Metric) string {
	var buf bytes.Buffer

	for _, metric := range metrics {
		// 写入指标名称
		buf.WriteString(metric.Name)

		// 写入标签（包括额外标签）
		allLabels := make(map[string]string)

		// 添加配置中的额外标签
		for k, v := range c.config.ExtraLabels {
			allLabels[k] = v
		}

		// 添加指标自身的标签
		for k, v := range metric.Labels {
			allLabels[k] = v
		}

		if len(allLabels) > 0 {
			buf.WriteString("{")
			first := true
			for k, v := range allLabels {
				if !first {
					buf.WriteString(",")
				}
				buf.WriteString(fmt.Sprintf(`%s="%s"`, k, escapeLabel(v)))
				first = false
			}
			buf.WriteString("}")
		}

		// 写入值
		buf.WriteString(fmt.Sprintf(" %g", metric.Value))

		// 写入时间戳（毫秒）
		if !metric.Timestamp.IsZero() {
			buf.WriteString(fmt.Sprintf(" %d", metric.Timestamp.UnixMilli()))
		}

		buf.WriteString("\n")
	}

	return buf.String()
}

// escapeLabel 转义标签值中的特殊字符
func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\t", "\\t")
	return value
}

// HealthCheck 检查VictoriaMetrics连接健康状态
func (c *VMClient) HealthCheck(ctx context.Context) error {
	// 发送一个简单的测试指标
	testMetric := []Metric{
		{
			Name:      "ros_exporter_health_check",
			Value:     1,
			Labels:    map[string]string{"check": "connectivity"},
			Timestamp: time.Now(),
		},
	}

	return c.Push(ctx, testMetric)
}

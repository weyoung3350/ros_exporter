package ros

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Detector ROS版本检测器
type Detector struct {
	timeout time.Duration
}

// NewDetector 创建新的检测器
func NewDetector() *Detector {
	return &Detector{
		timeout: 5 * time.Second,
	}
}

// DetectResult ROS1检测结果
type DetectResult struct {
	IsROS1Available bool                  `json:"is_ros1_available"` // ROS1是否可用
	Distribution    string                `json:"distribution"`      // ROS1发行版
	Environment     map[string]string     `json:"environment"`       // 环境变量
	Paths           []string              `json:"paths"`             // ROS1相关路径
	Commands        map[string]bool       `json:"commands"`          // 可用命令
	Details         map[string]interface{} `json:"details"`          // 详细信息
}

// DetectROS1Environment 检测ROS1环境
func (d *Detector) DetectROS1Environment(ctx context.Context) (*DetectResult, error) {
	result := &DetectResult{
		IsROS1Available: false,
		Environment:     make(map[string]string),
		Paths:           []string{},
		Commands:        make(map[string]bool),
		Details:         make(map[string]interface{}),
	}

	// 使用带超时的上下文
	detectCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// 1. 检测环境变量
	d.detectEnvironmentVariables(result)

	// 2. 检测ROS1
	result.IsROS1Available = d.detectROS1(detectCtx, result)

	// 3. 获取发行版信息
	if result.IsROS1Available {
		result.Distribution = d.getROS1Distribution(detectCtx)
	}

	// 4. 收集详细信息
	d.collectDetailedInfo(detectCtx, result)

	return result, nil
}

// detectEnvironmentVariables 检测ROS1环境变量
func (d *Detector) detectEnvironmentVariables(result *DetectResult) {
	ros1EnvVars := []string{
		"ROS_VERSION", "ROS_DISTRO", "ROS_PACKAGE_PATH", "ROS_MASTER_URI",
		"CMAKE_PREFIX_PATH", "PYTHONPATH", "PATH",
	}

	for _, envVar := range ros1EnvVars {
		if value := os.Getenv(envVar); value != "" {
			result.Environment[envVar] = value
		}
	}
}

// detectROS1 检测ROS1环境
func (d *Detector) detectROS1(ctx context.Context, result *DetectResult) bool {
	indicators := []func(context.Context, *DetectResult) bool{
		d.checkROS1Commands,
		d.checkROS1Paths,
		d.checkROS1Environment,
		d.checkROSMaster,
	}

	detected := false
	ros1Details := make(map[string]interface{})

	for _, indicator := range indicators {
		if indicator(ctx, result) {
			detected = true
		}
	}

	if detected {
		result.Details["ros1"] = ros1Details
	}

	return detected
}

// checkROS1Commands 检查ROS1命令
func (d *Detector) checkROS1Commands(ctx context.Context, result *DetectResult) bool {
	commands := []string{"roscore", "rosnode", "rostopic", "rosparam", "roslaunch", "rosrun"}
	detected := false

	for _, cmd := range commands {
		if d.commandExists(ctx, cmd) {
			result.Commands[cmd] = true
			detected = true
		}
	}

	return detected
}



// checkROS1Paths 检查ROS1路径
func (d *Detector) checkROS1Paths(ctx context.Context, result *DetectResult) bool {
	paths := []string{
		"/opt/ros/*/setup.bash",
		"/opt/ros/melodic",
		"/opt/ros/noetic",
	}

	ros1Paths := []string{}
	for _, pattern := range paths {
		matches, _ := filepath.Glob(pattern)
		ros1Paths = append(ros1Paths, matches...)
	}

	if len(ros1Paths) > 0 {
		result.Paths = ros1Paths
		return true
	}

	return false
}

// checkROS1Environment 检查ROS1环境变量
func (d *Detector) checkROS1Environment(ctx context.Context, result *DetectResult) bool {
	// ROS_VERSION=1 表示ROS1
	if version := os.Getenv("ROS_VERSION"); version == "1" {
		return true
	}

	// 检查ROS_MASTER_URI
	if masterURI := os.Getenv("ROS_MASTER_URI"); masterURI != "" {
		return true
	}

	// 检查ROS_PACKAGE_PATH (ROS1特有)
	if packagePath := os.Getenv("ROS_PACKAGE_PATH"); packagePath != "" {
		return true
	}

	return false
}



// checkROSMaster 检查ROS Master是否运行
func (d *Detector) checkROSMaster(ctx context.Context, result *DetectResult) bool {
	cmd := exec.CommandContext(ctx, "rosnode", "list")
	if err := cmd.Run(); err == nil {
		result.Details["ros_master_running"] = true
		return true
	}
	return false
}



// commandExists 检查命令是否存在
func (d *Detector) commandExists(ctx context.Context, command string) bool {
	cmd := exec.CommandContext(ctx, "which", command)
	return cmd.Run() == nil
}





// getROS1Distribution 获取ROS1发行版
func (d *Detector) getROS1Distribution(ctx context.Context) string {
	// 1. 从环境变量获取
	if distro := os.Getenv("ROS_DISTRO"); distro != "" {
		return distro
	}

	// 2. 从命令行获取
	cmd := exec.CommandContext(ctx, "rosversion", "-d")
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

	// 3. 从路径推断
	paths := []string{
		"/opt/ros/melodic",
		"/opt/ros/noetic",
	}
	
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			if match := regexp.MustCompile(`/opt/ros/(\w+)`).FindStringSubmatch(path); len(match) > 1 {
				return match[1]
			}
		}
	}

	return ""
}



// collectDetailedInfo 收集详细信息
func (d *Detector) collectDetailedInfo(ctx context.Context, result *DetectResult) {
	details := make(map[string]interface{})

	// 收集Python版本
	if cmd := exec.CommandContext(ctx, "python", "--version"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			details["python_version"] = strings.TrimSpace(string(output))
		}
	}

	// 收集系统信息
	if cmd := exec.CommandContext(ctx, "uname", "-a"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			details["system_info"] = strings.TrimSpace(string(output))
		}
	}

	// 收集网络接口
	if cmd := exec.CommandContext(ctx, "ip", "link", "show"); cmd.Run() == nil {
		if output, err := cmd.Output(); err == nil {
			interfaces := d.parseNetworkInterfaces(string(output))
			details["network_interfaces"] = interfaces
		}
	}

	result.Details["system"] = details
}

// parseNetworkInterfaces 解析网络接口
func (d *Detector) parseNetworkInterfaces(output string) []string {
	var interfaces []string
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		if match := regexp.MustCompile(`^\d+:\s+(\w+):`).FindStringSubmatch(line); len(match) > 1 {
			interfaces = append(interfaces, match[1])
		}
	}
	
	return interfaces
}

// GetRecommendedConfiguration 获取ROS1推荐配置
func (d *Detector) GetRecommendedConfiguration(result *DetectResult) map[string]interface{} {
	config := make(map[string]interface{})

	if result.IsROS1Available {
		config["ros_version"] = "1"
		config["master_uri"] = result.Environment["ROS_MASTER_URI"]
		if config["master_uri"] == "" {
			config["master_uri"] = "http://localhost:11311"
		}
		config["distribution"] = result.Distribution
	}

	config["auto_detected"] = true
	config["detection_time"] = time.Now().Unix()

	return config
}

// ValidateROS1Environment 验证ROS1环境
func (d *Detector) ValidateROS1Environment(ctx context.Context) error {
	return d.validateROS1Environment(ctx)
}

// validateROS1Environment 验证ROS1环境
func (d *Detector) validateROS1Environment(ctx context.Context) error {
	// 检查roscore是否可用
	if !d.commandExists(ctx, "roscore") {
		return fmt.Errorf("roscore命令不可用")
	}

	// 检查ROS Master
	if cmd := exec.CommandContext(ctx, "rosnode", "list"); cmd.Run() != nil {
		return fmt.Errorf("无法连接到ROS Master，请确保roscore正在运行")
	}

	return nil
}

 
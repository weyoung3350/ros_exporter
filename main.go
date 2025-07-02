package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ros_exporter/internal/config"
	"ros_exporter/internal/exporter"
)

var (
	configFile = flag.String("config", "config.yaml", "配置文件路径")
	version    = flag.Bool("version", false, "显示版本信息")
	interfaces = flag.String("interfaces", "", "指定监控的网络接口，用逗号分隔 (例如: eth0,wlan0)")
)

const (
	AppName    = "ros_exporter"
	AppVersion = "1.0.0"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s %s\n", AppName, AppVersion)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 处理命令行指定的网络接口
	if *interfaces != "" {
		interfaceList := strings.Split(*interfaces, ",")
		for i, iface := range interfaceList {
			interfaceList[i] = strings.TrimSpace(iface)
		}
		cfg.Collectors.System.Network.Interfaces = interfaceList
		log.Printf("使用命令行指定的网络接口: %v", interfaceList)
	}

	// 创建Exporter
	exporterInstance, err := exporter.New(cfg)
	if err != nil {
		log.Fatalf("创建Exporter失败: %v", err)
	}

	// 创建上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动Exporter
	log.Printf("启动 %s %s", AppName, AppVersion)
	log.Printf("推送目标: %s", cfg.VictoriaMetrics.Endpoint)
	log.Printf("推送间隔: %v", cfg.Exporter.PushInterval)

	// 显示HTTP服务器配置
	if cfg.Exporter.HTTPServer.Enabled {
		log.Printf("HTTP服务器: http://%s:%d (endpoints: %v)",
			cfg.Exporter.HTTPServer.Address,
			cfg.Exporter.HTTPServer.Port,
			cfg.Exporter.HTTPServer.Endpoints)
	} else {
		log.Printf("HTTP服务器: 已禁用")
	}

	// 显示监控配置信息
	if cfg.Collectors.System.Temperature.Enabled {
		log.Printf("CPU温度监控: 启用 (来源: %s)", cfg.Collectors.System.Temperature.TempSource)
	}
	if cfg.Collectors.System.Network.BandwidthEnabled {
		if len(cfg.Collectors.System.Network.Interfaces) > 0 {
			log.Printf("网络带宽监控: 启用 (接口: %v)", cfg.Collectors.System.Network.Interfaces)
		} else {
			log.Printf("网络带宽监控: 启用 (所有接口)")
		}
	}

	go func() {
		if err := exporterInstance.Start(ctx); err != nil {
			log.Printf("Exporter运行错误: %v", err)
			cancel()
		}
	}()

	// 等待退出信号
	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，开始优雅退出...", sig)
	case <-ctx.Done():
		log.Printf("上下文取消，开始退出...")
	}

	// 优雅退出
	log.Printf("正在停止Exporter...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := exporterInstance.Stop(shutdownCtx); err != nil {
		log.Printf("停止Exporter时出错: %v", err)
	}

	log.Printf("%s 已退出", AppName)
}

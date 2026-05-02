# CLAUDE.md

> **最近一次更新**：2026-05-02

## 当前状态

### 主线：宇树 B2 工业级四足适配（已联调成功）

- **目标载体**：Jetson AGX Thor (ARM64 + Ubuntu 24.04) + 宇树 B2 编号 1401
- **数据流**：B2 主控 → unitree_sdk2 DDS → C wrapper（`internal/sdk/unitree/b2_sdk.cpp`）→ cgo → `internal/b2.B2DataSource` → `internal/collectors/B2Collector` → push VictoriaMetrics → Grafana
- **部署形态**：Thor 上 `docker compose` 跑三服务（VM + Grafana + ros_exporter 容器，挂载主机 binary 和共享库）
- **指标范围**：12 关节温度/扭矩/位置/速度/mode/lost、IMU 姿态/角速度/加速度、电池 SOC/cell voltage/NTC、足端力、运动状态、DDS 自检 4 路
- **Grafana 入口**：`http://<thor-ip>:3000` admin/admin → 文件夹 B2 → "B2 机器人总览"（18 panel）

### 关键设计文档（跨电脑接手必读）

- `docs/superpowers/specs/2026-05-02-b2-adaptation-design.md` —— 完整方案 v3，含 codex 评审反馈逐条落地
- `docs/CODEX_REVIEW_REQUEST.md` / `docs/CODEX_REVIEW_REPORT.md` —— 第三方独立技术评审
- `docs/对话记录.md` —— 里程碑追加日志

### 架构（B2 适配引入的新分层）

```
internal/
├── b2/                        ← B2 数据源抽象（DataSource interface + DDS/mock 实现）
├── g1/                        ← G1 同步重构（与 B2 对称，零行为变化约束）
├── sdk/unitree/
│   ├── b2_sdk.h / b2_sdk.cpp  ← unitree_sdk2 的 C ABI wrapper（含 DDS 自愈守护线程）
│   └── Makefile.b2            ← 编 libb2sdk.a 静态库
├── collectors/
│   ├── b2.go                  ← 调 b2.GetSnapshot() 输出 b2_* 指标
│   └── bms.go                 ← B2/G1 分支转发到对应 DataSource
└── config/
    └── config.go              ← B2CollectorConfig 含 DataSource/topic/staleness 字段
deploy/docker/                 ← docker-compose + grafana provisioning + B2 dashboard
```

### Build tag 矩阵

| 平台 | tag | B2 数据源 | G1 数据源 |
|------|-----|----------|-----------|
| macOS 默认 | `cgo,darwin` | dds_stub.go（明确报错）| sdk_stub.go（明确报错） |
| Linux + 无 SDK | `cgo,linux` | dds_stub.go | sdk_stub.go（要 `g1sdk` tag）|
| Thor + 装好 unitree_sdk2 | `cgo,linux` | dds_cgo.go ✓ | sdk_stub.go（默认） |
| Linux + libg1sdk + 显式启用 | `cgo,linux,g1sdk` | dds_cgo.go ✓ | sdk_cgo.go ✓ |

mock 数据源始终可用（任何 tag 组合）。

### 容器栈启停

```bash
cd ~/ros_exporter/deploy/docker
docker compose up -d        # 启动
docker compose down         # 停止（保留 vm-data / grafana-data 卷）
docker compose logs -f
docker compose restart ros_exporter   # 改 binary 后重启
```

### 已闭环参考

- B2 关节顺序确认：FR/FL/RR/RL × hip/thigh/calf（用 b2_stand_example.cpp 的 `_targetPos_4` 验证）
- BmsState IDL 字段实测：cell_vol[15] 但 B2 实际有效 14 节，第 15 节是预留槽（min/diff 计算跳过 0）
- 容器代理坑：Thor docker daemon 继承 sing-box 全局代理，所有容器需 NO_PROXY 防御
- ros_exporter 二进制依赖：libddscxx.so.0 + libddsc.so.0（在 /usr/local/lib，已加 ld.so.conf.d）

---

======设备介绍======
	ROSMASTER X3 标准版
	树莓派5-8GB
	Astra Pro Plus 深度相机
	思岚A1M8激光雷达
	电池组（12.6V，6000mAh）
	64GTF卡
	IP: 192.168.31.109  用户pi 密码yahboom	

======ROSMASTER X3 标准版物品清单======
	摇臂悬挂架
	防撞架
	电机底板
	车架主控固定板
	摇臂挂支架
	灯条固定板
	码盘底板
	ROS小车扩展板
	USB HUB扩展板
	电机4
	OLED屏扩展板
	联轴器
	LED灯条
	排线若干
	数据线
	螺丝刀
	游戏手柄+7号电池
	电池盒
	USB 3.0
	电池充电器
	零件包
	手机支架
	塑料轮6（含驱动轮4、从动轮2）
	麦克纳姆轮4

======语言交互包======
	语音交互模块
	Type-C数据线
	语音蜂鸣包
	喇叭

## Essential Development Commands

### Building and Testing
- `./build.sh build` - Build the application (default: just run `./build.sh`)
- `./build.sh test` - Run tests with logs saved to `tmp/test/`
- `./build.sh clean` - Clean build files and temporary files
- `./build.sh package` - Build multi-platform release packages
- `./build.sh docker` - Build Docker image

### Go Commands
- `go build -o ros_exporter main.go` - Direct build
- `go test -v ./...` - Run all tests
- `go mod tidy` - Clean up dependencies

### SDK Build (C++ component)
- `cd internal/sdk/unitree && make` - Build Unitree SDK library (mock mode)
- `cd internal/sdk/unitree && make real` - Build with real SDK dependencies
- `cd internal/sdk/unitree && make install` - Install library to Go project

## High-Level Architecture

### Core Structure
This is a **ROS metrics exporter** written in Go that collects system, ROS, and BMS (Battery Management System) metrics and pushes them to VictoriaMetrics. The architecture follows a modular collector pattern:

```
main.go → exporter → [system|ros|bms] collectors → victoria_metrics client
```

### Key Components

1. **Collectors** (`internal/collectors/`):
   - `system.go` - System metrics (CPU, memory, disk, network, temperature)
   - `ros.go` - ROS node/topic monitoring
   - `bms.go` - Battery management via multiple interfaces
   - `b2.go` - B2 robot specific metrics

2. **ROS Integration** (`internal/ros/`):
   - `detector.go` - Auto-detect ROS1/ROS2
   - `adapter_ros1.go` - ROS1 specific implementation
   - `factory.go` - ROS adapter factory pattern

3. **SDK Integration** (`internal/sdk/unitree/`):
   - C++ wrapper for Unitree G1 robot SDK
   - CGO bindings for Go integration
   - Supports both mock and real hardware modes

4. **Push-based Architecture**:
   - Direct push to VictoriaMetrics (not pull-based like Prometheus)
   - Handles dynamic IPs and network instability
   - Built-in retry mechanisms and error recovery

### Configuration System
- Uses single `config.yaml` file for all settings
- Supports environment-specific overrides (dev/test/production)
- Automatic default config generation on first run
- Detailed configuration guide in `docs/CONFIG_GUIDE.md`

### Data Flow
1. Collectors gather metrics independently
2. Exporter aggregates and timestamps all metrics
3. VictoriaMetrics client formats as Prometheus text
4. Push to VictoriaMetrics endpoint with retry logic

### Multi-interface Support
- **BMS interfaces**: Unitree SDK, Serial, CAN bus
- **ROS versions**: Auto-detection of ROS1/ROS2
- **Network monitoring**: Configurable interface filtering
- **Temperature**: Both `sensors` command and thermal_zone files

## Development Notes

### Project Naming
Recent commits show migration from "agent" to "exporter" naming - ensure new code uses "exporter" terminology consistently.

### Temporary Files
All temporary files go in `tmp/` directory:
- `tmp/build/` - Build artifacts  
- `tmp/test/` - Test outputs
- `tmp/logs/` - Runtime logs
- `tmp/cache/` - Cache files

Use `./scripts/quick-clean.sh` for cleanup.

### Multi-platform Support
The build system creates binaries for:
- linux/amd64, linux/arm64 (robots)
- darwin/amd64, darwin/arm64 (development)

### CGO Dependencies
The Unitree SDK requires CGO compilation. Mock mode available for development without hardware dependencies.
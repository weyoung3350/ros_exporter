# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
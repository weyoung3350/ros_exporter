# Codex 独立技术评审请求

> 请你以独立技术评审者身份，对方案文档做评审，找盲点和错误。

## 待评审方案

**方案文档路径**：`/Users/dna/.claude/plans/codex-fluffy-pillow.md`

**仓库路径**：`/Users/dna/ros_exporter`（已克隆，分支 main）

**参考资料**：`/Users/dna/Documents/Develop/claude_prj/AGX-Thor/AGX-Thor上狗集成方案.md`（B2 + Jetson Thor 现场部署上下文）

请先读这三个文件，再开始评审。

## 项目背景（30 秒上下文）

- 开源项目 `ros_exporter`（Go 写的 Prometheus/VictoriaMetrics exporter）
- 已支持 Unitree G1（CGO 真集成）、Go2/B2（stub）、ROSMaster-X3
- 当前任务：把 B2 适配做成真集成，部署到 Jetson AGX Thor (ARM64) + 宇树 B2 1401
- 用户已通过 4 轮 brainstorm 确定核心决策（DDS+CGO、interface 抽象、本机编译+systemd），plan 已写入文件

## 评审重点

A. **技术正确性**
- B2 在 unitree_sdk2 中的 topic 名（`rt/lowstate` / `rt/sportmodestate`）和字段是否准确
- 关节顺序假设（FR/FL/RR/RL，每腿 hip/thigh/calf）是否与 SDK 实际一致
- C wrapper 隔离 C++ unitree_sdk2 是否最优做法，有无更轻路径
- DDS 多播在 systemd `User=root` 下的权限/网络要求
- Jetson AGX Thor (ARM64 L4T) 上 unitree_sdk2 编译的已知坑（boost、glibc、cmake）

B. **架构合理性**
- `B2DataSource` interface 拆 5 个 GetXxx 方法 vs 单个 GetSnapshot 哪个更合理（DDS wrapper 内部缓存最新值）
- 关节单位改 rad（与 SDK 一致）会不会让现有 Grafana dashboard 翻车
- 配置删除 `MaxJointTemp` 等阈值移给 VM/Grafana 告警是否合理
- G1 同步重构的"零行为变化"硬约束是否可达成（特别是 build tag 切换 + interface 引入是否会改变 mock 数据序列）

C. **遗漏检查**
- DDS 断开后的重连/自愈逻辑（plan 没明说）
- exporter 自身健康指标是否齐全
- 工业级 B2 1401 的关键指标遗漏（足端力 FootForce、电机模式 mode、目标 vs 实际电流、CRC 错误计数？）
- 指标命名规范（Prometheus 推荐 `_seconds`、`_bytes`、`_total` 等单位后缀）

D. **更优方案**
- 直接订 unitree_ros2（已封装为 ROS2 topic）是否比 DDS 直连更简单（现场 robot_control 容器已经在跑这个）
- 多台 B2：单 exporter + 多 robot_id vs 每台独立 exporter

## 输出要求

按以下分级输出，每条 1-2 句给具体改法（不要写代码）：

- **Critical**（不改方案会出错）
- **Warning**（建议改但不阻塞）
- **Info**（学习价值或未来优化）

总报告 ≤ 800 字。命中要害比面面俱到重要——若某方面没问题，直接说"OK"即可。

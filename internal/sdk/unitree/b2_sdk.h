/*
 * b2_sdk.h — 宇树 B2 unitree_sdk2 的 C ABI wrapper
 *
 * 设计：
 *   - 内部用 C++ unitree_sdk2 订阅 rt/lowstate 和 rt/sportmodestate
 *   - 后台线程做 last_seen 检测和重建 subscriber（DDS 自愈，对应 codex C-3）
 *   - 对外只暴露 C ABI POD struct，方便 Go cgo 转换
 *
 * 调用顺序：
 *   1. b2_dds_init(iface, low_topic, sport_topic) → 初始化 ChannelFactory + 订阅
 *   2. b2_dds_wait_first_packet(timeout_ms) → 等首包，返回 0 表示成功
 *   3. b2_dds_get_snapshot(out) → 读最新缓存，可重复调
 *   4. b2_dds_get_health(out) → 读自检
 *   5. b2_dds_close() → 释放
 *
 * 字段对齐 unitree_sdk2 v0.10.x 的 unitree_go IDL（B2 复用 go2 IDL）。
 */

#ifndef ROS_EXPORTER_B2_SDK_H
#define ROS_EXPORTER_B2_SDK_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ===== POD 数据结构（与 unitree_sdk2 IDL 字段一一对应）===== */

/* 单个 motor 状态。我们只取 LowState.motor_state 数组前 12 个（B2 是 12 关节四足）。 */
typedef struct {
    float   q;           /* 关节位置，rad */
    float   dq;          /* 关节角速度，rad/s */
    float   tau_est;     /* 估计扭矩，N·m */
    int8_t  temperature; /* 关节温度，°C（IDL 是 int8） */
    uint8_t mode;        /* 电机模式 */
    uint32_t lost;       /* 通信丢失计数 */
    uint32_t reserve;    /* 保留字段（含 status 信息） */
} B2RawMotor;

/* IMU 状态 */
typedef struct {
    float quaternion[4];   /* w, x, y, z */
    float gyroscope[3];    /* rad/s, body frame */
    float accelerometer[3];/* m/s^2, body frame */
    float rpy[3];          /* roll, pitch, yaw, rad */
    int8_t temperature;    /* IMU 温度 */
} B2RawIMU;

/* BMS 状态（来自 BmsState_ IDL） */
typedef struct {
    uint8_t  version_high;
    uint8_t  version_low;
    uint8_t  status;
    uint8_t  soc;
    int32_t  current;     /* mA，带符号 */
    uint16_t cycle;
    uint8_t  bq_ntc[2];
    uint8_t  mcu_ntc[2];  /* IDL 字段名 mcu_ntc */
    uint16_t cell_vol[15];
} B2RawBms;

/* LowState 缓存 */
typedef struct {
    B2RawIMU   imu;
    B2RawMotor motors[12];
    B2RawBms   bms;
    int16_t    foot_force[4];      /* 索引 0=FL,1=FR,2=RL,3=RR */
    int16_t    foot_force_est[4];

    /* 收到此 LowState 的本机单调时钟（纳秒）。0 表示从未收到。 */
    int64_t    received_steady_ns;
} B2RawLowState;

/* SportState 缓存（rt/sportmodestate） */
typedef struct {
    float   velocity[3];   /* body frame: x_forward, y_left, z_up; m/s */
    uint8_t mode;          /* 运动模式原码 */
    uint8_t gait_type;     /* 步态原码 */

    int64_t received_steady_ns;
} B2RawSportState;

/* 一次 GetSnapshot 返回的全部缓存 */
typedef struct {
    int has_low_state;     /* 0/1：是否曾经收到至少一条 lowstate */
    int has_sport_state;   /* 0/1 */
    B2RawLowState   low_state;
    B2RawSportState sport_state;
} B2RawSnapshot;

/* DDS 自检状态 */
typedef struct {
    int      dds_connected;          /* 0/1：当前是否健康（last_seen 在阈值内） */
    int64_t  low_state_age_ns;       /* now - last_seen, 0=正常, INT64_MAX=从未收到 */
    int64_t  sport_state_age_ns;
    uint64_t reconnect_count;
    uint64_t error_count;
    char     last_error[256];
} B2RawHealth;

/* ===== API ===== */

/* 初始化 ChannelFactory + 创建 subscriber。可重入安全，多次调用只第一次生效。
 * iface             : 网卡名，如 "enP2p1s0"
 * low_state_topic   : 一般 "rt/lowstate"
 * sport_state_topic : 一般 "rt/sportmodestate"
 * stale_threshold_ms: 守护线程判定超龄的阈值（毫秒）
 * 返回 0 成功，非 0 失败（详细错误用 b2_dds_last_error 取）
 */
int b2_dds_init(const char* iface,
                const char* low_state_topic,
                const char* sport_state_topic,
                int stale_threshold_ms);

/* 阻塞等待至少一条 lowstate 到来，超时返回 -1。 */
int b2_dds_wait_first_packet(int timeout_ms);

/* 读最新 snapshot（不阻塞）。out 必须非 NULL。 */
int b2_dds_get_snapshot(B2RawSnapshot* out);

/* 读 DDS 自检状态。out 必须非 NULL。 */
int b2_dds_get_health(B2RawHealth* out);

/* 释放所有资源。多次调用安全。 */
void b2_dds_close(void);

/* 取最近一次错误的描述字符串。返回的指针由 wrapper 持有，不要 free。 */
const char* b2_dds_last_error(void);

#ifdef __cplusplus
}
#endif

#endif /* ROS_EXPORTER_B2_SDK_H */

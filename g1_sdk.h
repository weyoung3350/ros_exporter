#ifndef G1_SDK_H
#define G1_SDK_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>

// G1电池状态结构体
typedef struct {
    // 基础电池信息
    double voltage;                    // 总电压 (V)
    double current;                    // 电流 (A)
    double temperature;                // 平均温度 (°C)
    double capacity;                   // 剩余容量 (%)
    uint32_t cycle_count;              // 循环次数
    
    // 单体电压 (40节电池)
    double cell_voltages[40];          // 单体电压数组
    
    // 温度传感器 (12个)
    double temperatures[12];           // 温度传感器数组
    
    // 状态标志
    bool is_charging;                  // 充电状态
    bool is_discharging;              // 放电状态
    uint8_t health_status;            // 健康状态 (0-100)
    
    // 错误状态
    uint32_t error_code;              // 错误代码
    char error_message[256];          // 错误信息
    
    // 时间戳
    uint64_t timestamp;               // 数据时间戳
} G1BatteryStatus;

// SDK初始化和清理
int g1_sdk_init(const char* config_path);
void g1_sdk_cleanup(void);

// 连接管理
int g1_sdk_connect(void);
int g1_sdk_disconnect(void);
bool g1_sdk_is_connected(void);

// 数据获取
int g1_sdk_get_battery_status(G1BatteryStatus* status);
int g1_sdk_get_last_error(char* buffer, int buffer_size);

// 回调函数类型
typedef void (*BatteryStatusCallback)(const G1BatteryStatus* status);
int g1_sdk_set_battery_callback(BatteryStatusCallback callback);

#ifdef __cplusplus
}
#endif

#endif // G1_SDK_H 
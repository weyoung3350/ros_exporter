#include "g1_sdk.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <signal.h>

// 全局标志用于控制程序退出
static volatile bool running = true;

// 信号处理函数
void signalHandler(int signal) {
    std::cout << "\n收到信号 " << signal << "，正在退出..." << std::endl;
    running = false;
}

// 电池状态回调函数
void batteryCallback(const G1BatteryStatus* status) {
    std::cout << "=== 电池状态回调 ===" << std::endl;
    std::cout << "电压: " << status->voltage << "V" << std::endl;
    std::cout << "电流: " << status->current << "A" << std::endl;
    std::cout << "温度: " << status->temperature << "°C" << std::endl;
    std::cout << "电量: " << status->capacity << "%" << std::endl;
    std::cout << "健康度: " << (int)status->health_status << "%" << std::endl;
    std::cout << "充电状态: " << (status->is_charging ? "是" : "否") << std::endl;
    std::cout << "放电状态: " << (status->is_discharging ? "是" : "否") << std::endl;
    std::cout << "循环次数: " << status->cycle_count << std::endl;
    std::cout << "错误代码: " << status->error_code << std::endl;
    if (status->error_code != 0) {
        std::cout << "错误信息: " << status->error_message << std::endl;
    }
    std::cout << "时间戳: " << status->timestamp << std::endl;
    std::cout << "========================" << std::endl;
}

int main() {
    std::cout << "=== 宇树G1 SDK测试程序 ===" << std::endl;
    
    // 注册信号处理
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);
    
    // 初始化SDK
    std::cout << "初始化SDK..." << std::endl;
    if (g1_sdk_init(nullptr) != 0) {
        char errorBuffer[256];
        g1_sdk_get_last_error(errorBuffer, sizeof(errorBuffer));
        std::cerr << "SDK初始化失败: " << errorBuffer << std::endl;
        return 1;
    }
    std::cout << "SDK初始化成功" << std::endl;
    
    // 设置回调函数
    std::cout << "设置电池状态回调..." << std::endl;
    if (g1_sdk_set_battery_callback(batteryCallback) != 0) {
        std::cerr << "设置回调失败" << std::endl;
        g1_sdk_cleanup();
        return 1;
    }
    
    // 连接到机器人
    std::cout << "连接到G1机器人..." << std::endl;
    if (g1_sdk_connect() != 0) {
        char errorBuffer[256];
        g1_sdk_get_last_error(errorBuffer, sizeof(errorBuffer));
        std::cerr << "连接失败: " << errorBuffer << std::endl;
        g1_sdk_cleanup();
        return 1;
    }
    std::cout << "连接成功" << std::endl;
    
    // 主循环
    std::cout << "开始监控电池状态... (按Ctrl+C退出)" << std::endl;
    int counter = 0;
    
    while (running) {
        // 检查连接状态
        if (!g1_sdk_is_connected()) {
            std::cerr << "连接丢失，尝试重新连接..." << std::endl;
            if (g1_sdk_connect() != 0) {
                char errorBuffer[256];
                g1_sdk_get_last_error(errorBuffer, sizeof(errorBuffer));
                std::cerr << "重新连接失败: " << errorBuffer << std::endl;
                break;
            }
            std::cout << "重新连接成功" << std::endl;
        }
        
        // 每5秒主动获取一次电池状态
        if (counter % 50 == 0) {
            G1BatteryStatus status;
            if (g1_sdk_get_battery_status(&status) == 0) {
                std::cout << "\n=== 主动查询电池状态 ===" << std::endl;
                std::cout << "电压: " << status.voltage << "V, ";
                std::cout << "电流: " << status.current << "A, ";
                std::cout << "电量: " << status.capacity << "%, ";
                std::cout << "温度: " << status.temperature << "°C" << std::endl;
                
                // 显示单体电压统计
                double minVoltage = status.cell_voltages[0];
                double maxVoltage = status.cell_voltages[0];
                double sumVoltage = 0;
                
                for (int i = 0; i < 40; i++) {
                    double v = status.cell_voltages[i];
                    if (v < minVoltage) minVoltage = v;
                    if (v > maxVoltage) maxVoltage = v;
                    sumVoltage += v;
                }
                
                std::cout << "单体电压: 最小=" << minVoltage << "V, ";
                std::cout << "最大=" << maxVoltage << "V, ";
                std::cout << "平均=" << (sumVoltage / 40) << "V, ";
                std::cout << "差值=" << (maxVoltage - minVoltage) << "V" << std::endl;
                
                // 显示温度统计
                double minTemp = status.temperatures[0];
                double maxTemp = status.temperatures[0];
                double sumTemp = 0;
                
                for (int i = 0; i < 12; i++) {
                    double t = status.temperatures[i];
                    if (t < minTemp) minTemp = t;
                    if (t > maxTemp) maxTemp = t;
                    sumTemp += t;
                }
                
                std::cout << "温度传感器: 最小=" << minTemp << "°C, ";
                std::cout << "最大=" << maxTemp << "°C, ";
                std::cout << "平均=" << (sumTemp / 12) << "°C, ";
                std::cout << "差值=" << (maxTemp - minTemp) << "°C" << std::endl;
                std::cout << "========================\n" << std::endl;
            } else {
                char errorBuffer[256];
                g1_sdk_get_last_error(errorBuffer, sizeof(errorBuffer));
                std::cerr << "获取电池状态失败: " << errorBuffer << std::endl;
            }
        }
        
        // 等待100ms
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        counter++;
    }
    
    // 清理资源
    std::cout << "\n正在清理资源..." << std::endl;
    g1_sdk_disconnect();
    g1_sdk_cleanup();
    std::cout << "程序退出" << std::endl;
    
    return 0;
} 
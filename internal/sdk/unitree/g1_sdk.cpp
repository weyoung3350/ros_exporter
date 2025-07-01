#include "g1_sdk.h"
#include <iostream>
#include <memory>
#include <thread>
#include <chrono>
#include <cstring>
#include <mutex>
#include <atomic>

// DDS相关头文件 (这里使用模拟实现，实际项目中需要真实的DDS库)
#ifdef USE_REAL_DDS
#include "unitree/robot/g1/robot_state/robot_state.hpp"
#include "unitree/common/thread/thread.hpp"
#include "unitree/common/time/time_tool.hpp"
#include "unitree/idl/go2/BmsState_.hpp"
#else
// 模拟DDS消息结构
namespace unitree {
namespace robot {
namespace g1 {
    struct BmsState {
        double voltage = 25.2;
        double current = -2.5;
        double temperature = 35.0;
        double capacity = 85.0;
        uint32_t cycle_count = 150;
        std::vector<double> cell_voltages;
        std::vector<double> temperatures;
        bool is_charging = false;
        bool is_discharging = true;
        uint8_t health_status = 95;
        uint32_t error_code = 0;
        std::string error_message = "";
        uint64_t timestamp = 0;
        
        BmsState() {
            // 初始化40节电池电压 (典型值)
            cell_voltages.resize(40);
            for (int i = 0; i < 40; i++) {
                cell_voltages[i] = 3.7 + (rand() % 100) / 1000.0; // 3.7-3.8V
            }
            
            // 初始化12个温度传感器
            temperatures.resize(12);
            for (int i = 0; i < 12; i++) {
                temperatures[i] = 30.0 + (rand() % 100) / 10.0; // 30-40°C
            }
        }
    };
}}}
#endif

// 全局状态管理
class G1SDKManager {
private:
    std::atomic<bool> initialized_{false};
    std::atomic<bool> connected_{false};
    std::mutex data_mutex_;
    std::unique_ptr<std::thread> data_thread_;
    std::atomic<bool> running_{false};
    
    unitree::robot::g1::BmsState latest_bms_data_;
    BatteryStatusCallback callback_{nullptr};
    std::string last_error_;
    
public:
    static G1SDKManager& getInstance() {
        static G1SDKManager instance;
        return instance;
    }
    
    int initialize(const char* config_path) {
        if (initialized_) {
            return 0; // 已初始化
        }
        
        try {
            // 初始化DDS域
            std::cout << "[G1SDK] 初始化SDK，配置路径: " << (config_path ? config_path : "默认") << std::endl;
            
#ifdef USE_REAL_DDS
            // 真实DDS初始化代码
            // unitree::robot::ChannelFactory::Instance()->Init(0, "lo");
#else
            // 模拟初始化
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
#endif
            
            initialized_ = true;
            std::cout << "[G1SDK] SDK初始化成功" << std::endl;
            return 0;
        } catch (const std::exception& e) {
            last_error_ = "初始化失败: " + std::string(e.what());
            std::cerr << "[G1SDK] " << last_error_ << std::endl;
            return -1;
        }
    }
    
    void cleanup() {
        if (!initialized_) return;
        
        disconnect();
        initialized_ = false;
        std::cout << "[G1SDK] SDK清理完成" << std::endl;
    }
    
    int connect() {
        if (!initialized_) {
            last_error_ = "SDK未初始化";
            return -1;
        }
        
        if (connected_) {
            return 0; // 已连接
        }
        
        try {
#ifdef USE_REAL_DDS
            // 真实DDS连接代码
            // subscriber_ = std::make_unique<unitree::robot::ChannelSubscriber<unitree::robot::g1::BmsState>>("rt/bms_state");
            // subscriber_->InitChannel();
#else
            // 模拟连接
            std::this_thread::sleep_for(std::chrono::milliseconds(50));
#endif
            
            // 启动数据接收线程
            running_ = true;
            data_thread_ = std::make_unique<std::thread>(&G1SDKManager::dataReceiveLoop, this);
            
            connected_ = true;
            std::cout << "[G1SDK] 连接成功" << std::endl;
            return 0;
        } catch (const std::exception& e) {
            last_error_ = "连接失败: " + std::string(e.what());
            std::cerr << "[G1SDK] " << last_error_ << std::endl;
            return -1;
        }
    }
    
    int disconnect() {
        if (!connected_) return 0;
        
        // 停止数据线程
        running_ = false;
        if (data_thread_ && data_thread_->joinable()) {
            data_thread_->join();
        }
        data_thread_.reset();
        
        connected_ = false;
        std::cout << "[G1SDK] 断开连接" << std::endl;
        return 0;
    }
    
    bool isConnected() const {
        return connected_;
    }
    
    int getBatteryStatus(G1BatteryStatus* status) {
        if (!status) {
            last_error_ = "状态指针为空";
            return -1;
        }
        
        if (!connected_) {
            last_error_ = "未连接到机器人";
            return -1;
        }
        
        std::lock_guard<std::mutex> lock(data_mutex_);
        
        // 转换数据格式
        status->voltage = latest_bms_data_.voltage;
        status->current = latest_bms_data_.current;
        status->temperature = latest_bms_data_.temperature;
        status->capacity = latest_bms_data_.capacity;
        status->cycle_count = latest_bms_data_.cycle_count;
        status->is_charging = latest_bms_data_.is_charging;
        status->is_discharging = latest_bms_data_.is_discharging;
        status->health_status = latest_bms_data_.health_status;
        status->error_code = latest_bms_data_.error_code;
        status->timestamp = latest_bms_data_.timestamp;
        
        // 复制单体电压数据
        for (size_t i = 0; i < 40 && i < latest_bms_data_.cell_voltages.size(); i++) {
            status->cell_voltages[i] = latest_bms_data_.cell_voltages[i];
        }
        
        // 复制温度数据
        for (size_t i = 0; i < 12 && i < latest_bms_data_.temperatures.size(); i++) {
            status->temperatures[i] = latest_bms_data_.temperatures[i];
        }
        
        // 复制错误信息
        strncpy(status->error_message, latest_bms_data_.error_message.c_str(), 255);
        status->error_message[255] = '\0';
        
        return 0;
    }
    
    int getLastError(char* buffer, int buffer_size) {
        if (!buffer || buffer_size <= 0) return -1;
        
        strncpy(buffer, last_error_.c_str(), buffer_size - 1);
        buffer[buffer_size - 1] = '\0';
        return 0;
    }
    
    int setBatteryCallback(BatteryStatusCallback callback) {
        callback_ = callback;
        return 0;
    }
    
private:
    void dataReceiveLoop() {
        std::cout << "[G1SDK] 数据接收线程启动" << std::endl;
        
        while (running_) {
            try {
#ifdef USE_REAL_DDS
                // 真实DDS数据接收
                // unitree::robot::g1::BmsState msg;
                // if (subscriber_->Spin(msg, 100)) {
                //     updateBmsData(msg);
                // }
#else
                // 模拟数据更新
                updateSimulatedData();
#endif
                
                // 触发回调
                if (callback_) {
                    G1BatteryStatus status;
                    if (getBatteryStatus(&status) == 0) {
                        callback_(&status);
                    }
                }
                
                std::this_thread::sleep_for(std::chrono::milliseconds(100)); // 10Hz更新
            } catch (const std::exception& e) {
                last_error_ = "数据接收错误: " + std::string(e.what());
                std::cerr << "[G1SDK] " << last_error_ << std::endl;
            }
        }
        
        std::cout << "[G1SDK] 数据接收线程退出" << std::endl;
    }
    
    void updateSimulatedData() {
        std::lock_guard<std::mutex> lock(data_mutex_);
        
        // 模拟电池数据变化
        static double voltage_base = 25.2;
        static double current_base = -2.5;
        static int counter = 0;
        
        counter++;
        
        // 模拟电压轻微波动
        latest_bms_data_.voltage = voltage_base + 0.1 * sin(counter * 0.1);
        latest_bms_data_.current = current_base + 0.2 * sin(counter * 0.15);
        latest_bms_data_.temperature = 35.0 + 2.0 * sin(counter * 0.05);
        
        // 模拟容量缓慢下降
        if (counter % 100 == 0) {
            latest_bms_data_.capacity = std::max(0.0, latest_bms_data_.capacity - 0.1);
        }
        
        // 更新时间戳
        latest_bms_data_.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();
        
        // 更新单体电压
        for (size_t i = 0; i < latest_bms_data_.cell_voltages.size(); i++) {
            latest_bms_data_.cell_voltages[i] = 3.7 + 0.1 * sin(counter * 0.1 + i * 0.1);
        }
        
        // 更新温度传感器
        for (size_t i = 0; i < latest_bms_data_.temperatures.size(); i++) {
            latest_bms_data_.temperatures[i] = 30.0 + 5.0 * sin(counter * 0.05 + i * 0.2);
        }
    }
};

// C接口实现
extern "C" {

int g1_sdk_init(const char* config_path) {
    return G1SDKManager::getInstance().initialize(config_path);
}

void g1_sdk_cleanup(void) {
    G1SDKManager::getInstance().cleanup();
}

int g1_sdk_connect(void) {
    return G1SDKManager::getInstance().connect();
}

int g1_sdk_disconnect(void) {
    return G1SDKManager::getInstance().disconnect();
}

bool g1_sdk_is_connected(void) {
    return G1SDKManager::getInstance().isConnected();
}

int g1_sdk_get_battery_status(G1BatteryStatus* status) {
    return G1SDKManager::getInstance().getBatteryStatus(status);
}

int g1_sdk_get_last_error(char* buffer, int buffer_size) {
    return G1SDKManager::getInstance().getLastError(buffer, buffer_size);
}

int g1_sdk_set_battery_callback(BatteryStatusCallback callback) {
    return G1SDKManager::getInstance().setBatteryCallback(callback);
}

} 
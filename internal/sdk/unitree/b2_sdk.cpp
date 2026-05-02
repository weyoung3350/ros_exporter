/*
 * b2_sdk.cpp — B2 unitree_sdk2 C wrapper 实现
 *
 * 内部架构：
 *   - 全局单例 g_state（thread-safe，受 g_state_mu 保护）
 *   - 两个 ChannelSubscriber，回调线程（DDS 自带）写缓存 + 时间戳
 *   - 一个守护线程定期检查 last_seen，超龄重建对应 subscriber
 *
 * unitree_sdk2 不需要每次 GetSnapshot 都调 SDK——它把消息异步推到回调，
 * 我们在回调里把字段拷到 plain POD 缓存。Go 侧调 GetSnapshot 时只需加锁拷出缓存。
 */

#include "b2_sdk.h"

#include <atomic>
#include <chrono>
#include <cstring>
#include <mutex>
#include <string>
#include <thread>

#include <unitree/robot/channel/channel_factory.hpp>
#include <unitree/robot/channel/channel_subscriber.hpp>
#include <unitree/idl/go2/LowState_.hpp>
#include <unitree/idl/go2/SportModeState_.hpp>

using LowStateMsg   = unitree_go::msg::dds_::LowState_;
using SportStateMsg = unitree_go::msg::dds_::SportModeState_;

namespace {

using SteadyClock = std::chrono::steady_clock;

/* 把 SteadyClock::time_point 转成纳秒（相对单调时钟原点） */
int64_t steady_now_ns() {
    return std::chrono::duration_cast<std::chrono::nanoseconds>(
        SteadyClock::now().time_since_epoch()).count();
}

struct State {
    std::mutex mu;

    bool initialized = false;
    std::string iface;
    std::string low_topic;
    std::string sport_topic;
    int64_t stale_threshold_ns = 5LL * 1000 * 1000 * 1000; /* 5s */

    /* 缓存 */
    B2RawLowState   low_cache{};
    B2RawSportState sport_cache{};
    int has_low = 0;
    int has_sport = 0;

    /* 自检 */
    std::atomic<int>      dds_connected{0};
    std::atomic<uint64_t> reconnect_count{0};
    std::atomic<uint64_t> error_count{0};
    std::string           last_error;

    /* DDS subscriber 句柄（用智能指针由 ChannelFactory 管理） */
    unitree::robot::ChannelSubscriberPtr<LowStateMsg>   low_sub;
    unitree::robot::ChannelSubscriberPtr<SportStateMsg> sport_sub;

    /* 守护线程 */
    std::thread       watchdog;
    std::atomic<bool> running{false};
};

/* 全局单例。Unitree ChannelFactory 自身就是单例，多个 wrapper 实例没意义。 */
State g_state;

void set_last_error(const std::string& msg) {
    std::lock_guard<std::mutex> lk(g_state.mu);
    g_state.last_error = msg;
}

/* ----- 字段拷贝 ----- */

/* LowState 回调：DDS 收到一条 lowstate 时调用。拷字段到 g_state.low_cache。 */
void on_low_state(const void* raw) {
    const auto* m = static_cast<const LowStateMsg*>(raw);
    int64_t now = steady_now_ns();

    std::lock_guard<std::mutex> lk(g_state.mu);

    /* IMU */
    auto& imu = g_state.low_cache.imu;
    const auto& mi = m->imu_state();
    for (int i = 0; i < 4; ++i) imu.quaternion[i]    = mi.quaternion()[i];
    for (int i = 0; i < 3; ++i) imu.gyroscope[i]     = mi.gyroscope()[i];
    for (int i = 0; i < 3; ++i) imu.accelerometer[i] = mi.accelerometer()[i];
    for (int i = 0; i < 3; ++i) imu.rpy[i]           = mi.rpy()[i];
    imu.temperature = mi.temperature();

    /* 关节（取前 12 个 motor_state） */
    for (int i = 0; i < 12; ++i) {
        const auto& src = m->motor_state()[i];
        auto& dst = g_state.low_cache.motors[i];
        dst.q           = src.q();
        dst.dq          = src.dq();
        dst.tau_est     = src.tau_est();
        dst.temperature = src.temperature();
        dst.mode        = src.mode();
        dst.lost        = src.lost();
        dst.reserve     = src.reserve()[0]; /* reserve 是 array<uint32_t, 4>，取第一个 */
    }

    /* 足端力 */
    for (int i = 0; i < 4; ++i) {
        g_state.low_cache.foot_force[i]     = m->foot_force()[i];
        g_state.low_cache.foot_force_est[i] = m->foot_force_est()[i];
    }

    /* BMS */
    auto& bms = g_state.low_cache.bms;
    const auto& mb = m->bms_state();
    bms.version_high = mb.version_high();
    bms.version_low  = mb.version_low();
    bms.status       = mb.status();
    bms.soc          = mb.soc();
    bms.current      = mb.current();
    bms.cycle        = mb.cycle();
    for (int i = 0; i < 2; ++i) {
        bms.bq_ntc[i]  = mb.bq_ntc()[i];
        bms.mcu_ntc[i] = mb.mcu_ntc()[i];
    }
    for (int i = 0; i < 15; ++i) {
        bms.cell_vol[i] = mb.cell_vol()[i];
    }

    g_state.low_cache.received_steady_ns = now;
    g_state.has_low = 1;
    /* 收到 lowstate 即视为连接健康，无需等 watchdog 下一轮（watchdog 仍负责检测断流） */
    g_state.dds_connected.store(1);
}

/* SportState 回调 */
void on_sport_state(const void* raw) {
    const auto* m = static_cast<const SportStateMsg*>(raw);
    int64_t now = steady_now_ns();

    std::lock_guard<std::mutex> lk(g_state.mu);
    auto& sp = g_state.sport_cache;
    for (int i = 0; i < 3; ++i) sp.velocity[i] = m->velocity()[i];
    sp.mode      = m->mode();
    sp.gait_type = m->gait_type();
    sp.received_steady_ns = now;
    g_state.has_sport = 1;
}

/* ----- 守护线程：超龄重建 ----- */

bool create_subscribers() {
    try {
        g_state.low_sub.reset(new unitree::robot::ChannelSubscriber<LowStateMsg>(
            g_state.low_topic.c_str()));
        g_state.low_sub->InitChannel(on_low_state, 1);

        g_state.sport_sub.reset(new unitree::robot::ChannelSubscriber<SportStateMsg>(
            g_state.sport_topic.c_str()));
        g_state.sport_sub->InitChannel(on_sport_state, 1);
        return true;
    } catch (const std::exception& e) {
        set_last_error(std::string("create subscriber failed: ") + e.what());
        g_state.error_count++;
        return false;
    }
}

void watchdog_loop() {
    using namespace std::chrono_literals;
    while (g_state.running.load()) {
        std::this_thread::sleep_for(1s);
        int64_t now = steady_now_ns();
        int64_t threshold;
        int64_t last_low, last_sport;
        {
            std::lock_guard<std::mutex> lk(g_state.mu);
            threshold = g_state.stale_threshold_ns;
            last_low   = g_state.low_cache.received_steady_ns;
            last_sport = g_state.sport_cache.received_steady_ns;
        }
        bool low_stale   = (last_low   > 0 && now - last_low   > threshold);
        bool sport_stale = (last_sport > 0 && now - last_sport > threshold);
        bool low_never   = (last_low   == 0);

        /* connected = "lowstate 在阈值内"——sportmodestate 缺失不视为断流（B2 不一定订上） */
        g_state.dds_connected.store(last_low > 0 && !low_stale);

        if (low_stale || sport_stale) {
            set_last_error("DDS 超龄，重建 subscriber");
            g_state.reconnect_count++;
            /* unitree_sdk2 没有显式 unsubscribe；reset 智能指针即可。 */
            g_state.low_sub.reset();
            g_state.sport_sub.reset();
            create_subscribers();
        }
        (void)low_never; /* 暂未使用 */
    }
}

} /* anonymous namespace */

/* ===== 公共 API 实现 ===== */

extern "C" int b2_dds_init(const char* iface,
                           const char* low_state_topic,
                           const char* sport_state_topic,
                           int stale_threshold_ms) {
    {
        std::lock_guard<std::mutex> lk(g_state.mu);
        if (g_state.initialized) return 0;
        g_state.iface       = iface       ? iface       : "";
        g_state.low_topic   = low_state_topic   ? low_state_topic   : "rt/lowstate";
        g_state.sport_topic = sport_state_topic ? sport_state_topic : "rt/sportmodestate";
        if (stale_threshold_ms > 0) {
            g_state.stale_threshold_ns =
                int64_t(stale_threshold_ms) * 1000 * 1000;
        }
    }

    try {
        if (!g_state.iface.empty()) {
            unitree::robot::ChannelFactory::Instance()->Init(0, g_state.iface);
        } else {
            unitree::robot::ChannelFactory::Instance()->Init(0);
        }
    } catch (const std::exception& e) {
        set_last_error(std::string("ChannelFactory init failed: ") + e.what());
        g_state.error_count++;
        return 1;
    }

    if (!create_subscribers()) return 2;

    g_state.running.store(true);
    g_state.watchdog = std::thread(watchdog_loop);

    {
        std::lock_guard<std::mutex> lk(g_state.mu);
        g_state.initialized = true;
    }
    return 0;
}

extern "C" int b2_dds_wait_first_packet(int timeout_ms) {
    using namespace std::chrono;
    auto deadline = steady_clock::now() + milliseconds(timeout_ms);
    while (steady_clock::now() < deadline) {
        {
            std::lock_guard<std::mutex> lk(g_state.mu);
            if (g_state.has_low) return 0;
        }
        std::this_thread::sleep_for(milliseconds(50));
    }
    set_last_error("等待 lowstate 首包超时");
    return -1;
}

extern "C" int b2_dds_get_snapshot(B2RawSnapshot* out) {
    if (!out) return -1;
    std::lock_guard<std::mutex> lk(g_state.mu);
    out->has_low_state   = g_state.has_low;
    out->has_sport_state = g_state.has_sport;
    out->low_state       = g_state.low_cache;
    out->sport_state     = g_state.sport_cache;
    return 0;
}

extern "C" int b2_dds_get_health(B2RawHealth* out) {
    if (!out) return -1;
    int64_t now = steady_now_ns();
    std::lock_guard<std::mutex> lk(g_state.mu);
    out->dds_connected    = g_state.dds_connected.load();
    out->reconnect_count  = g_state.reconnect_count.load();
    out->error_count      = g_state.error_count.load();
    out->low_state_age_ns =
        g_state.low_cache.received_steady_ns > 0
            ? (now - g_state.low_cache.received_steady_ns)
            : INT64_MAX;
    out->sport_state_age_ns =
        g_state.sport_cache.received_steady_ns > 0
            ? (now - g_state.sport_cache.received_steady_ns)
            : INT64_MAX;
    std::strncpy(out->last_error,
                 g_state.last_error.c_str(),
                 sizeof(out->last_error) - 1);
    out->last_error[sizeof(out->last_error) - 1] = '\0';
    return 0;
}

extern "C" void b2_dds_close(void) {
    g_state.running.store(false);
    if (g_state.watchdog.joinable()) g_state.watchdog.join();

    std::lock_guard<std::mutex> lk(g_state.mu);
    g_state.low_sub.reset();
    g_state.sport_sub.reset();
    g_state.initialized = false;
}

extern "C" const char* b2_dds_last_error(void) {
    static thread_local std::string buf;
    std::lock_guard<std::mutex> lk(g_state.mu);
    buf = g_state.last_error;
    return buf.c_str();
}

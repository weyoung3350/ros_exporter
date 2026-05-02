/*
 * test_b2sdk.cpp — 仅验证 libb2sdk.a 链接通过的最小测试。
 * 不连真实 B2，仅 init + close 走一遍代码路径，不报段错误即通过。
 *
 * 在 Thor 上跑：
 *   make -f Makefile.b2 test
 *   ./test_b2sdk enP2p1s0
 */

#include <cstdio>
#include <cstdlib>
#include <unistd.h>

#include "b2_sdk.h"

int main(int argc, char** argv) {
    const char* iface = (argc > 1) ? argv[1] : "enP2p1s0";

    printf("[test_b2sdk] init iface=%s\n", iface);
    int rc = b2_dds_init(iface, "rt/lowstate", "rt/sportmodestate", 5000);
    if (rc != 0) {
        fprintf(stderr, "init failed: %s\n", b2_dds_last_error());
        return 1;
    }

    printf("[test_b2sdk] wait first packet (3s)...\n");
    rc = b2_dds_wait_first_packet(3000);
    if (rc == 0) {
        printf("[test_b2sdk] ✓ 收到 lowstate 首包\n");

        B2RawSnapshot snap{};
        b2_dds_get_snapshot(&snap);
        printf("  has_low_state=%d has_sport_state=%d\n",
               snap.has_low_state, snap.has_sport_state);
        if (snap.has_low_state) {
            printf("  joint[0] q=%.3f dq=%.3f tau=%.3f temp=%d°C mode=%d\n",
                   snap.low_state.motors[0].q,
                   snap.low_state.motors[0].dq,
                   snap.low_state.motors[0].tau_est,
                   snap.low_state.motors[0].temperature,
                   snap.low_state.motors[0].mode);
            printf("  IMU quat=(%.3f, %.3f, %.3f, %.3f)\n",
                   snap.low_state.imu.quaternion[0],
                   snap.low_state.imu.quaternion[1],
                   snap.low_state.imu.quaternion[2],
                   snap.low_state.imu.quaternion[3]);
            printf("  battery soc=%d%% current=%dmA cell_vol[0]=%dmV\n",
                   snap.low_state.bms.soc,
                   snap.low_state.bms.current,
                   snap.low_state.bms.cell_vol[0]);
            printf("  foot_force=[%d,%d,%d,%d]\n",
                   snap.low_state.foot_force[0],
                   snap.low_state.foot_force[1],
                   snap.low_state.foot_force[2],
                   snap.low_state.foot_force[3]);
        }
        if (snap.has_sport_state) {
            printf("  sport mode=%d gait=%d vel=(%.3f, %.3f, %.3f)\n",
                   snap.sport_state.mode,
                   snap.sport_state.gait_type,
                   snap.sport_state.velocity[0],
                   snap.sport_state.velocity[1],
                   snap.sport_state.velocity[2]);
        }
    } else {
        printf("[test_b2sdk] ! 3s 未收到首包: %s\n", b2_dds_last_error());
    }

    B2RawHealth h{};
    b2_dds_get_health(&h);
    printf("[test_b2sdk] health: dds_connected=%d reconnect=%llu error=%llu\n",
           h.dds_connected,
           (unsigned long long)h.reconnect_count,
           (unsigned long long)h.error_count);

    printf("[test_b2sdk] closing...\n");
    b2_dds_close();
    printf("[test_b2sdk] done\n");
    return 0;
}

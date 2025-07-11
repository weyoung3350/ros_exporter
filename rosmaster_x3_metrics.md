# ROSMaster-X3监控指标列表

## 电机指标
- `rosmaster_motor_temperature_celsius` - 电机温度 (°C)
- `rosmaster_motor_torque_nm` - 电机扭矩 (N·m)
- `rosmaster_motor_speed_rpm` - 电机转速 (RPM)

## 运动控制指标
- `rosmaster_wheel_odometry_x_meters` - X轴里程 (m)
- `rosmaster_wheel_odometry_y_meters` - Y轴里程 (m)  
- `rosmaster_velocity_linear_mps` - 线性速度 (m/s)
- `rosmaster_velocity_angular_rps` - 角速度 (rad/s)

## 激光雷达指标
- `rosmaster_lidar_scan_frequency_hz` - 扫描频率 (Hz)
- `rosmaster_lidar_point_count` - 点云数量
- `rosmaster_lidar_range_max_meters` - 最大测距 (m)

## IMU指标
- `rosmaster_imu_acceleration_x_mss` - X轴加速度 (m/s²)
- `rosmaster_imu_acceleration_y_mss` - Y轴加速度 (m/s²)
- `rosmaster_imu_acceleration_z_mss` - Z轴加速度 (m/s²)
- `rosmaster_imu_gyroscope_x_rps` - X轴角速度 (rad/s)
- `rosmaster_imu_gyroscope_y_rps` - Y轴角速度 (rad/s)
- `rosmaster_imu_gyroscope_z_rps` - Z轴角速度 (rad/s)

## 电池指标
- `rosmaster_battery_voltage_volts` - 电池电压 (V)
- `rosmaster_battery_current_amperes` - 电池电流 (A)
- `rosmaster_battery_soc_percent` - 电量百分比 (%)
- `rosmaster_battery_temperature_celsius` - 电池温度 (°C)

## 导航指标
- `rosmaster_pose_x_meters` - 当前位置X坐标 (m)
- `rosmaster_pose_y_meters` - 当前位置Y坐标 (m)
- `rosmaster_path_length_meters` - 路径长度 (m)

## 状态指标
- `rosmaster_emergency_stop_status` - 急停状态 (0/1)
- `rosmaster_error_count` - 错误计数
- `rosmaster_warning_count` - 警告计数
- `rosmaster_safety_score` - 安全评分 (0-1)

总计: 25个专门针对ROSMaster-X3的监控指标

#!/bin/bash

# 设置SSH连接到ROSMaster-X3的脚本

ROBOT_IP="192.168.31.109"
ROBOT_USER="pi"
ROBOT_PASSWORD="yahboom"

echo "=== 设置SSH免密登录到ROSMaster-X3 ==="
echo "机器人IP: $ROBOT_IP"
echo "用户: $ROBOT_USER"
echo ""

# 使用expect自动输入密码
expect << EOF
spawn ssh-copy-id -i ~/.ssh/rosmaster_x3_key.pub $ROBOT_USER@$ROBOT_IP
expect {
    "password:" {
        send "$ROBOT_PASSWORD\r"
        exp_continue
    }
    "continue connecting" {
        send "yes\r"
        exp_continue
    }
    eof
}
EOF

echo ""
echo "测试SSH连接..."
ssh $ROBOT_USER@$ROBOT_IP "echo 'SSH连接成功！'; hostname; uname -a"

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ SSH免密登录设置成功！"
else
    echo ""
    echo "✗ SSH连接失败，请检查网络和密码"
    exit 1
fi
#!/bin/bash

# ros_exporter GitHub推送脚本
# 使用方法: ./push_to_github.sh <github_repo_url>

set -e

if [ $# -eq 0 ]; then
    echo "使用方法: $0 <github_repo_url>"
    echo ""
    echo "示例:"
    echo "  $0 https://github.com/YOUR_USERNAME/ros_exporter.git"
    echo "  $0 git@github.com:YOUR_USERNAME/ros_exporter.git"
    echo ""
    echo "请先在GitHub上创建名为 'ros_exporter' 的新仓库"
    exit 1
fi

REPO_URL="$1"

echo "=== ros_exporter GitHub推送 ==="
echo "目标仓库: $REPO_URL"
echo ""

# 检查Git状态
if [ ! -d ".git" ]; then
    echo "错误: 当前目录不是Git仓库"
    exit 1
fi

# 检查是否有提交
if ! git rev-parse HEAD >/dev/null 2>&1; then
    echo "错误: 没有找到Git提交"
    exit 1
fi

# 添加远程仓库
echo "添加远程仓库..."
if git remote get-url origin >/dev/null 2>&1; then
    echo "警告: origin远程仓库已存在，将覆盖"
    git remote set-url origin "$REPO_URL"
else
    git remote add origin "$REPO_URL"
fi

# 设置主分支
echo "设置主分支为main..."
git branch -M main

# 推送到GitHub
echo "推送到GitHub..."
git push -u origin main

echo ""
echo "=== 推送完成 ==="
echo "仓库地址: $REPO_URL"
echo "提交信息: $(git log --oneline -1)"
echo "文件数量: $(git ls-files | wc -l)"
echo ""
echo "您现在可以在GitHub上查看代码了！" 
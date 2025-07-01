#!/bin/bash

# ros_exporter - 快速清理脚本
# 一键清理项目中的常见临时文件

# 清理编译产物
echo "清理编译产物..."
rm -f ros_exporter ros_exporter-* test_simple_cgo *.exe

# 清理系统文件
echo "清理系统文件..."
find . -name ".DS_Store" -delete 2>/dev/null || true
find . -name "._*" -delete 2>/dev/null || true
find . -name "*.swp" -delete 2>/dev/null || true
find . -name "*.swo" -delete 2>/dev/null || true
find . -name "*~" -delete 2>/dev/null || true

# 清理临时文件
echo "清理临时文件..."
rm -f *.log *.tmp *.temp

# 清理tmp目录内容(保留目录结构)
if [ -d "tmp" ]; then
    echo "清理tmp目录..."
    find tmp -type f -delete 2>/dev/null || true
    find tmp -type d -empty -delete 2>/dev/null || true
    # 重新创建基础目录结构
    mkdir -p tmp/{build,cache,logs,test}
fi

echo "快速清理完成！"

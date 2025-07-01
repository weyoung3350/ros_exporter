# Makefile for G1 SDK Library
# 用于编译宇树G1 SDK的C++库

# 编译器设置
CXX = g++
CXXFLAGS = -std=c++17 -fPIC -Wall -O2
LDFLAGS = -shared

# 目标架构 (可通过命令行覆盖)
ARCH ?= $(shell uname -m)

# 目录设置
SRC_DIR = .
BUILD_DIR = build
LIB_DIR = lib

# 源文件
SOURCES = g1_sdk.cpp
OBJECTS = $(SOURCES:.cpp=.o)
TARGET_LIB = libg1sdk.so

# 宇树SDK路径 (需要根据实际安装路径调整)
UNITREE_SDK_PATH ?= /opt/unitree_sdk
UNITREE_INCLUDE = -I$(UNITREE_SDK_PATH)/include
UNITREE_LIBS = -L$(UNITREE_SDK_PATH)/lib -lunitree_robot_sdk -lunitree_common

# DDS库路径 (Fast-DDS)
DDS_PATH ?= /usr/local
DDS_INCLUDE = -I$(DDS_PATH)/include
DDS_LIBS = -L$(DDS_PATH)/lib -lfastrtps -lfastcdr

# 包含路径
INCLUDES = $(UNITREE_INCLUDE) $(DDS_INCLUDE) -I.

# 链接库
LIBS = $(UNITREE_LIBS) $(DDS_LIBS) -lpthread -lm

# 编译标志
ifdef USE_REAL_DDS
CXXFLAGS += -DUSE_REAL_DDS
endif

# 默认目标
all: $(TARGET_LIB)

# 创建目录
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(LIB_DIR):
	mkdir -p $(LIB_DIR)

# 编译目标文件
%.o: %.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -c $< -o $@

# 链接共享库
$(TARGET_LIB): $(OBJECTS) | $(LIB_DIR)
	$(CXX) $(LDFLAGS) -Wl,-install_name,@rpath/$(TARGET_LIB) -o $(LIB_DIR)/$(TARGET_LIB) $(OBJECTS) $(LIBS)

# 模拟版本 (不依赖真实SDK)
mock: CXXFLAGS += -DMOCK_MODE
mock: LIBS = -lpthread -lm
mock: $(TARGET_LIB)

# 真实SDK版本
real: CXXFLAGS += -DUSE_REAL_DDS
real: $(TARGET_LIB)

# 交叉编译 ARM64
arm64: CXX = aarch64-linux-gnu-g++
arm64: ARCH = aarch64
arm64: $(TARGET_LIB)

# 清理
clean:
	rm -f *.o
	rm -rf $(BUILD_DIR)
	rm -rf $(LIB_DIR)

# 安装 (复制到Go项目的CGO路径)
install: 
	cp $(LIB_DIR)/$(TARGET_LIB) ../../../
	cp g1_sdk.h ../../../

# 测试编译
test: $(TARGET_LIB)
	$(CXX) -std=c++17 -I. -L$(LIB_DIR) test_sdk.cpp -lg1sdk -o test_g1sdk

# 显示配置信息
info:
	@echo "编译器: $(CXX)"
	@echo "架构: $(ARCH)"
	@echo "编译标志: $(CXXFLAGS)"
	@echo "包含路径: $(INCLUDES)"
	@echo "链接库: $(LIBS)"
	@echo "目标库: $(TARGET_LIB)"

# 帮助信息
help:
	@echo "可用目标:"
	@echo "  all      - 编译默认版本 (模拟模式)"
	@echo "  mock     - 编译模拟版本"
	@echo "  real     - 编译真实SDK版本"
	@echo "  arm64    - 交叉编译ARM64版本"
	@echo "  clean    - 清理编译文件"
	@echo "  install  - 安装到Go项目"
	@echo "  test     - 编译测试程序"
	@echo "  info     - 显示编译配置"
	@echo ""
	@echo "环境变量:"
	@echo "  UNITREE_SDK_PATH - 宇树SDK安装路径"
	@echo "  DDS_PATH         - DDS库安装路径"
	@echo "  USE_REAL_DDS     - 启用真实DDS支持"
	@echo "  ARCH             - 目标架构"

.PHONY: all mock real arm64 clean install test info help 
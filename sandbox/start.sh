#!/bin/bash

# 启动沙箱服务脚本

set -e

echo "=============================="
echo "🚀 启动沙箱服务"
echo "=============================="

# 检查 Python 环境
if ! command -v python3 &> /dev/null; then
    echo "❌ 未找到 python3"
    exit 1
fi

# 检查配置文件
if [ ! -f config.yaml ]; then
    echo "⚠️  未找到 config.yaml，从示例文件复制..."
    if [ -f config.example.yaml ]; then
        cp config.example.yaml config.yaml
        echo "✅ 已创建 config.yaml，请根据需要修改配置"
    else
        echo "❌ 未找到 config.example.yaml"
        exit 1
    fi
fi

# 检查依赖
echo "📦 检查 Python 依赖..."
if ! python3 -c "import flask, docker, psycopg2, yaml" 2>/dev/null; then
    echo "⚠️ 缺少依赖，正在安装..."
    pip install -r requirements.txt
fi

# 检查 Docker
echo "🐳 检查 Docker..."
if ! command -v docker &> /dev/null; then
    echo "❌ 未找到 docker"
    echo "   请先安装 Docker: https://docs.docker.com/get-docker/"
    exit 1
fi

# 检查 Docker 是否运行
if ! docker info &> /dev/null; then
    echo "❌ Docker 未运行"
    echo "   请启动 Docker 服务"
    exit 1
fi

# 设置环境（默认为 development）
export SANDBOX_ENV=${SANDBOX_ENV:-development}

# 加载环境变量（如果存在 .env 文件）
if [ -f .env ]; then
    echo "📝 加载 .env 环境变量..."
    export $(cat .env | grep -v '^#' | xargs)
fi

# 打印配置信息
echo ""
echo "⚙️ 配置信息:"
echo "   环境: $SANDBOX_ENV"
echo "   配置文件: config.yaml"
echo "   环境变量优先级: .env > config.yaml"
echo ""

# 启动服务
echo "🚀 启动沙箱服务..."
python3 main.py

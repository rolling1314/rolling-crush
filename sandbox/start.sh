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

# 检查依赖
echo "📦 检查 Python 依赖..."
if ! python3 -c "import flask, docker, psycopg2" 2>/dev/null; then
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

# 加载环境变量（如果存在）
if [ -f .env ]; then
    echo "📝 加载环境变量..."
    export $(cat .env | grep -v '^#' | xargs)
fi

# 打印配置信息
echo ""
echo "⚙️ 配置信息:"
echo "   数据库: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}/${POSTGRES_DB:-crush}"
echo "   用户: ${POSTGRES_USER:-crush}"
echo "   服务地址: ${SANDBOX_HOST:-0.0.0.0}:${SANDBOX_PORT:-8888}"
echo ""

# 启动服务
echo "🚀 启动沙箱服务..."
python3 main.py

#!/bin/bash

# Crush 开发环境资源启动脚本
# 根据 config.yaml 中的开发环境配置启动所需的容器资源

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置变量（来自 config.yaml development 环境）
# PostgreSQL
PG_HOST="localhost"
PG_PORT="5432"
PG_USER="crush"
PG_PASSWORD="123456"
PG_DATABASE="crush"
PG_CONTAINER_NAME="crush-postgres"

# Redis
REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_PASSWORD="123456"
REDIS_CONTAINER_NAME="crush-redis"

# MinIO
MINIO_PORT="9000"
MINIO_CONSOLE_PORT="9001"
MINIO_ACCESS_KEY="minioadmin"
MINIO_SECRET_KEY="minioadmin123"
MINIO_BUCKET="crush-images"
MINIO_CONTAINER_NAME="crush-minio"

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Docker 是否安装并运行
check_docker() {
    print_info "检查 Docker 状态..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        print_error "Docker 未运行，请先启动 Docker"
        exit 1
    fi
    print_success "Docker 运行正常"
}

# 拉取 Docker 镜像
pull_images() {
    print_info "拉取 Docker 镜像..."
    
    print_info "拉取 PostgreSQL 镜像..."
    docker pull postgres:15-alpine
    
    print_info "拉取 Redis 镜像..."
    docker pull redis:7-alpine
    
    print_info "拉取 MinIO 镜像..."
    docker pull minio/minio:latest
    
    print_success "所有镜像拉取完成"
}

# 停止并删除已存在的容器
cleanup_container() {
    local container_name=$1
    if docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
        print_warning "停止并删除已存在的容器: ${container_name}"
        docker stop "${container_name}" 2>/dev/null || true
        docker rm "${container_name}" 2>/dev/null || true
    fi
}

# 启动 PostgreSQL
start_postgres() {
    print_info "启动 PostgreSQL 容器..."
    
    cleanup_container "${PG_CONTAINER_NAME}"
    
    docker run -d \
        --name "${PG_CONTAINER_NAME}" \
        -e POSTGRES_USER="${PG_USER}" \
        -e POSTGRES_PASSWORD="${PG_PASSWORD}" \
        -e POSTGRES_DB="${PG_DATABASE}" \
        -p "${PG_PORT}:5432" \
        -v crush-postgres-data:/var/lib/postgresql/data \
        --restart unless-stopped \
        postgres:15-alpine
    
    print_info "等待 PostgreSQL 启动..."
    sleep 5
    
    # 等待 PostgreSQL 就绪
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if docker exec "${PG_CONTAINER_NAME}" pg_isready -U "${PG_USER}" &>/dev/null; then
            print_success "PostgreSQL 已启动"
            print_info "  - 主机: ${PG_HOST}"
            print_info "  - 端口: ${PG_PORT}"
            print_info "  - 用户: ${PG_USER}"
            print_info "  - 密码: ${PG_PASSWORD}"
            print_info "  - 数据库: ${PG_DATABASE}"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    print_error "PostgreSQL 启动超时"
    return 1
}

# 启动 Redis
start_redis() {
    print_info "启动 Redis 容器..."
    
    cleanup_container "${REDIS_CONTAINER_NAME}"
    
    docker run -d \
        --name "${REDIS_CONTAINER_NAME}" \
        -p "${REDIS_PORT}:6379" \
        -v crush-redis-data:/data \
        --restart unless-stopped \
        redis:7-alpine redis-server --requirepass "${REDIS_PASSWORD}" --appendonly yes
    
    print_info "等待 Redis 启动..."
    sleep 3
    
    # 验证 Redis 连接
    if docker exec "${REDIS_CONTAINER_NAME}" redis-cli -a "${REDIS_PASSWORD}" ping 2>/dev/null | grep -q "PONG"; then
        print_success "Redis 已启动"
        print_info "  - 主机: ${REDIS_HOST}"
        print_info "  - 端口: ${REDIS_PORT}"
        print_info "  - 密码: ${REDIS_PASSWORD}"
    else
        print_error "Redis 启动失败"
        return 1
    fi
}

# 启动 MinIO
start_minio() {
    print_info "启动 MinIO 容器..."
    
    cleanup_container "${MINIO_CONTAINER_NAME}"
    
    docker run -d \
        --name "${MINIO_CONTAINER_NAME}" \
        -e MINIO_ROOT_USER="${MINIO_ACCESS_KEY}" \
        -e MINIO_ROOT_PASSWORD="${MINIO_SECRET_KEY}" \
        -p "${MINIO_PORT}:9000" \
        -p "${MINIO_CONSOLE_PORT}:9001" \
        -v crush-minio-data:/data \
        --restart unless-stopped \
        minio/minio:latest server /data --console-address ":9001"
    
    print_info "等待 MinIO 启动..."
    sleep 5
    
    # 创建 bucket（使用 mc 客户端或 curl）
    print_info "创建 MinIO bucket: ${MINIO_BUCKET}"
    
    # 等待 MinIO 就绪
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "http://localhost:${MINIO_PORT}/minio/health/live" &>/dev/null; then
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    # 使用 docker 内置的 mc 创建 bucket
    docker exec "${MINIO_CONTAINER_NAME}" mc alias set local http://localhost:9000 "${MINIO_ACCESS_KEY}" "${MINIO_SECRET_KEY}" 2>/dev/null || true
    docker exec "${MINIO_CONTAINER_NAME}" mc mb local/"${MINIO_BUCKET}" 2>/dev/null || print_warning "Bucket 可能已存在"
    docker exec "${MINIO_CONTAINER_NAME}" mc anonymous set public local/"${MINIO_BUCKET}" 2>/dev/null || true
    
    print_success "MinIO 已启动"
    print_info "  - API 端口: ${MINIO_PORT}"
    print_info "  - Console 端口: ${MINIO_CONSOLE_PORT}"
    print_info "  - Access Key: ${MINIO_ACCESS_KEY}"
    print_info "  - Secret Key: ${MINIO_SECRET_KEY}"
    print_info "  - Bucket: ${MINIO_BUCKET}"
    print_info "  - Console URL: http://localhost:${MINIO_CONSOLE_PORT}"
}

# 显示所有服务状态
show_status() {
    echo ""
    print_info "========== 服务状态 =========="
    echo ""
    
    echo "容器状态:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "(NAMES|crush-)"
    
    echo ""
    print_info "========== 连接信息 =========="
    echo ""
    echo "PostgreSQL:"
    echo "  连接字符串: postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${PG_DATABASE}?sslmode=disable"
    echo ""
    echo "Redis:"
    echo "  连接字符串: redis://:${REDIS_PASSWORD}@${REDIS_HOST}:${REDIS_PORT}/0"
    echo ""
    echo "MinIO:"
    echo "  API Endpoint: http://localhost:${MINIO_PORT}"
    echo "  Console: http://localhost:${MINIO_CONSOLE_PORT}"
    echo ""
}

# 停止所有服务
stop_all() {
    print_info "停止所有 Crush 开发环境容器..."
    
    for container in "${PG_CONTAINER_NAME}" "${REDIS_CONTAINER_NAME}" "${MINIO_CONTAINER_NAME}"; do
        if docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
            print_info "停止 ${container}..."
            docker stop "${container}"
        fi
    done
    
    print_success "所有容器已停止"
}

# 删除所有服务（包括数据卷）
destroy_all() {
    print_warning "这将删除所有容器和数据卷，数据将丢失！"
    read -p "确定要继续吗？(y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        stop_all
        
        for container in "${PG_CONTAINER_NAME}" "${REDIS_CONTAINER_NAME}" "${MINIO_CONTAINER_NAME}"; do
            if docker ps -a --format '{{.Names}}' | grep -q "^${container}$"; then
                print_info "删除容器 ${container}..."
                docker rm "${container}"
            fi
        done
        
        print_info "删除数据卷..."
        docker volume rm crush-postgres-data crush-redis-data crush-minio-data 2>/dev/null || true
        
        print_success "所有资源已清理"
    else
        print_info "操作已取消"
    fi
}

# 显示帮助信息
show_help() {
    echo "Crush 开发环境资源管理脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  start     启动所有服务（默认）"
    echo "  stop      停止所有服务"
    echo "  restart   重启所有服务"
    echo "  status    显示服务状态"
    echo "  destroy   删除所有服务和数据"
    echo "  pull      仅拉取镜像"
    echo "  help      显示此帮助信息"
    echo ""
}

# 主函数
main() {
    local command="${1:-start}"
    
    case "${command}" in
        start)
            check_docker
            pull_images
            start_postgres
            start_redis
            start_minio
            show_status
            ;;
        stop)
            stop_all
            ;;
        restart)
            stop_all
            sleep 2
            check_docker
            start_postgres
            start_redis
            start_minio
            show_status
            ;;
        status)
            show_status
            ;;
        destroy)
            destroy_all
            ;;
        pull)
            check_docker
            pull_images
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: ${command}"
            show_help
            exit 1
            ;;
    esac
}

main "$@"

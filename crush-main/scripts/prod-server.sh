#!/bin/bash

# Crush 生产环境服务启动脚本
# 启动 HTTP Server 和 WebSocket Server

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# 配置
APP_ENV="production"
HTTP_SERVER_NAME="crush-http-server"
WS_SERVER_NAME="crush-ws-server"

# 目录配置
BIN_DIR="${PROJECT_ROOT}/bin"
LOG_DIR="${PROJECT_ROOT}/logs"
PID_DIR="${PROJECT_ROOT}/pids"

# 二进制文件路径
HTTP_SERVER_BIN="${BIN_DIR}/${HTTP_SERVER_NAME}"
WS_SERVER_BIN="${BIN_DIR}/${WS_SERVER_NAME}"

# PID 文件路径
HTTP_SERVER_PID="${PID_DIR}/${HTTP_SERVER_NAME}.pid"
WS_SERVER_PID="${PID_DIR}/${WS_SERVER_NAME}.pid"

# 日志文件路径
HTTP_SERVER_LOG="${LOG_DIR}/${HTTP_SERVER_NAME}.log"
WS_SERVER_LOG="${LOG_DIR}/${WS_SERVER_NAME}.log"

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

# 创建必要的目录
ensure_dirs() {
    mkdir -p "${BIN_DIR}"
    mkdir -p "${LOG_DIR}"
    mkdir -p "${PID_DIR}"
}

# 检查 Go 是否安装
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    print_info "Go 版本: $(go version)"
}

# 构建服务
build_servers() {
    print_info "构建服务..."
    
    cd "${PROJECT_ROOT}"
    
    print_info "构建 HTTP Server..."
    go build -o "${HTTP_SERVER_BIN}" ./cmd/http-server
    
    print_info "构建 WebSocket Server..."
    go build -o "${WS_SERVER_BIN}" ./cmd/ws-server
    
    print_success "构建完成"
}

# 检查进程是否运行
is_running() {
    local pid_file=$1
    if [ -f "${pid_file}" ]; then
        local pid=$(cat "${pid_file}")
        if ps -p "${pid}" > /dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

# 获取进程 PID
get_pid() {
    local pid_file=$1
    if [ -f "${pid_file}" ]; then
        cat "${pid_file}"
    fi
}

# 启动 HTTP Server
start_http_server() {
    if is_running "${HTTP_SERVER_PID}"; then
        print_warning "HTTP Server 已在运行 (PID: $(get_pid ${HTTP_SERVER_PID}))"
        return 0
    fi
    
    print_info "启动 HTTP Server..."
    print_info "配置文件: ${PROJECT_ROOT}/config.yaml"
    
    # 从 PROJECT_ROOT 启动，加载 crush-main/config.yaml
    cd "${PROJECT_ROOT}"
    
    APP_ENV="${APP_ENV}" nohup "${HTTP_SERVER_BIN}" > "${HTTP_SERVER_LOG}" 2>&1 &
    local pid=$!
    echo "${pid}" > "${HTTP_SERVER_PID}"
    
    sleep 2
    
    if is_running "${HTTP_SERVER_PID}"; then
        print_success "HTTP Server 已启动 (PID: ${pid})"
        print_info "  - 端口: 8001"
        print_info "  - 日志: ${HTTP_SERVER_LOG}"
    else
        print_error "HTTP Server 启动失败，请查看日志: ${HTTP_SERVER_LOG}"
        return 1
    fi
}

# 启动 WebSocket Server
start_ws_server() {
    if is_running "${WS_SERVER_PID}"; then
        print_warning "WebSocket Server 已在运行 (PID: $(get_pid ${WS_SERVER_PID}))"
        return 0
    fi
    
    print_info "启动 WebSocket Server..."
    print_info "配置文件: ${PROJECT_ROOT}/config.yaml"
    
    # 从 PROJECT_ROOT 启动，加载 crush-main/config.yaml
    cd "${PROJECT_ROOT}"
    
    APP_ENV="${APP_ENV}" nohup "${WS_SERVER_BIN}" > "${WS_SERVER_LOG}" 2>&1 &
    local pid=$!
    echo "${pid}" > "${WS_SERVER_PID}"
    
    sleep 2
    
    if is_running "${WS_SERVER_PID}"; then
        print_success "WebSocket Server 已启动 (PID: ${pid})"
        print_info "  - 端口: 8002"
        print_info "  - 日志: ${WS_SERVER_LOG}"
    else
        print_error "WebSocket Server 启动失败，请查看日志: ${WS_SERVER_LOG}"
        return 1
    fi
}

# 停止 HTTP Server
stop_http_server() {
    if ! is_running "${HTTP_SERVER_PID}"; then
        print_warning "HTTP Server 未在运行"
        rm -f "${HTTP_SERVER_PID}"
        return 0
    fi
    
    local pid=$(get_pid "${HTTP_SERVER_PID}")
    print_info "停止 HTTP Server (PID: ${pid})..."
    
    kill "${pid}" 2>/dev/null || true
    
    # 等待进程退出
    local count=0
    while is_running "${HTTP_SERVER_PID}" && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done
    
    # 如果还在运行，强制杀死
    if is_running "${HTTP_SERVER_PID}"; then
        print_warning "进程未响应，强制终止..."
        kill -9 "${pid}" 2>/dev/null || true
    fi
    
    rm -f "${HTTP_SERVER_PID}"
    print_success "HTTP Server 已停止"
}

# 停止 WebSocket Server
stop_ws_server() {
    if ! is_running "${WS_SERVER_PID}"; then
        print_warning "WebSocket Server 未在运行"
        rm -f "${WS_SERVER_PID}"
        return 0
    fi
    
    local pid=$(get_pid "${WS_SERVER_PID}")
    print_info "停止 WebSocket Server (PID: ${pid})..."
    
    kill "${pid}" 2>/dev/null || true
    
    # 等待进程退出
    local count=0
    while is_running "${WS_SERVER_PID}" && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done
    
    # 如果还在运行，强制杀死
    if is_running "${WS_SERVER_PID}"; then
        print_warning "进程未响应，强制终止..."
        kill -9 "${pid}" 2>/dev/null || true
    fi
    
    rm -f "${WS_SERVER_PID}"
    print_success "WebSocket Server 已停止"
}

# 启动所有服务
start_all() {
    ensure_dirs
    check_go
    build_servers
    start_http_server
    start_ws_server
    echo ""
    show_status
}

# 停止所有服务
stop_all() {
    stop_http_server
    stop_ws_server
}

# 重启所有服务
restart_all() {
    stop_all
    sleep 2
    start_all
}

# 显示状态
show_status() {
    print_info "========== 服务状态 =========="
    echo ""
    
    echo -n "HTTP Server:      "
    if is_running "${HTTP_SERVER_PID}"; then
        echo -e "${GREEN}运行中${NC} (PID: $(get_pid ${HTTP_SERVER_PID}))"
    else
        echo -e "${RED}已停止${NC}"
    fi
    
    echo -n "WebSocket Server: "
    if is_running "${WS_SERVER_PID}"; then
        echo -e "${GREEN}运行中${NC} (PID: $(get_pid ${WS_SERVER_PID}))"
    else
        echo -e "${RED}已停止${NC}"
    fi
    
    echo ""
    print_info "========== 服务信息 =========="
    echo ""
    echo "环境: ${APP_ENV}"
    echo "配置文件: ${PROJECT_ROOT}/config.yaml"
    echo ""
    echo "HTTP Server:"
    echo "  - URL: http://localhost:8001"
    echo "  - 日志: ${HTTP_SERVER_LOG}"
    echo ""
    echo "WebSocket Server:"
    echo "  - URL: ws://localhost:8002/ws"
    echo "  - 日志: ${WS_SERVER_LOG}"
    echo ""
}

# 查看日志
view_logs() {
    local server=$1
    case "${server}" in
        http)
            if [ -f "${HTTP_SERVER_LOG}" ]; then
                tail -f "${HTTP_SERVER_LOG}"
            else
                print_error "HTTP Server 日志文件不存在"
            fi
            ;;
        ws)
            if [ -f "${WS_SERVER_LOG}" ]; then
                tail -f "${WS_SERVER_LOG}"
            else
                print_error "WebSocket Server 日志文件不存在"
            fi
            ;;
        *)
            print_error "请指定服务: http 或 ws"
            echo "用法: $0 logs [http|ws]"
            ;;
    esac
}

# 显示帮助信息
show_help() {
    echo "Crush 生产环境服务管理脚本"
    echo ""
    echo "用法: $0 [命令] [参数]"
    echo ""
    echo "命令:"
    echo "  start         启动所有服务（构建并启动）"
    echo "  stop          停止所有服务"
    echo "  restart       重启所有服务"
    echo "  status        显示服务状态"
    echo "  build         仅构建服务"
    echo "  logs [http|ws] 查看服务日志"
    echo "  help          显示此帮助信息"
    echo ""
    echo "单独控制:"
    echo "  start-http    仅启动 HTTP Server"
    echo "  start-ws      仅启动 WebSocket Server"
    echo "  stop-http     仅停止 HTTP Server"
    echo "  stop-ws       仅停止 WebSocket Server"
    echo ""
    echo "示例:"
    echo "  $0 start          # 启动所有服务"
    echo "  $0 logs http      # 查看 HTTP Server 日志"
    echo "  $0 restart        # 重启所有服务"
    echo ""
}

# 主函数
main() {
    local command="${1:-help}"
    
    case "${command}" in
        start)
            start_all
            ;;
        stop)
            stop_all
            ;;
        restart)
            restart_all
            ;;
        status)
            show_status
            ;;
        build)
            ensure_dirs
            check_go
            build_servers
            ;;
        logs)
            view_logs "$2"
            ;;
        start-http)
            ensure_dirs
            check_go
            build_servers
            start_http_server
            ;;
        start-ws)
            ensure_dirs
            check_go
            build_servers
            start_ws_server
            ;;
        stop-http)
            stop_http_server
            ;;
        stop-ws)
            stop_ws_server
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

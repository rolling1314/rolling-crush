# 选择最新稳定 Go Alpine 镜像
FROM golang:1.23.9-alpine

# 安装系统依赖 + node + pnpm + python
RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    npm \
    python3 \
    py3-pip \
 && npm install -g npm@latest \
 && npm install -g pnpm@latest \
 && ln -sf python3 /usr/bin/python

# 设置工作目录
WORKDIR /workspace

# =====================
# 初始化 backend (Go)
# =====================
RUN mkdir -p backend \
 && cd backend \
 && go mod init backend \
 && cat << 'EOF' > main.go
package main

import "fmt"

func main() {
    fmt.Println("backend service running")
}
EOF

# =====================
# 初始化 frontend (Vite)
# =====================
RUN mkdir -p frontend \
 && cd frontend \
 && pnpm create vite@latest . --template react-ts --force \
 && pnpm install

# 暴露端口
EXPOSE 5173 8888

# 启动脚本
RUN cat << 'EOF' > /workspace/start.sh
#!/bin/sh
set -e
cd /workspace/frontend
echo "Starting Vite development server..."
exec pnpm dev --host 0.0.0.0
EOF

RUN chmod +x /workspace/start.sh

# 容器启动时运行前端项目
CMD ["sh", "/workspace/start.sh"]
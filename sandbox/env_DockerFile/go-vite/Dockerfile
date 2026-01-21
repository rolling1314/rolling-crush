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
 && echo 'package main' > main.go \ 
 && echo 'import "fmt"' >> main.go \ 
 && echo 'func main() {' >> main.go \ 
 && echo '    fmt.Println("backend service running")' >> main.go \ 
 && echo '}' >> main.go 
 
# ===================== 
# 初始化 frontend (Vite) 
# ===================== 
RUN mkdir -p frontend \ 
 && cd frontend \ 
 && pnpm create vite@latest . --template react-ts --force \ 
 && pnpm install 
 
# 暴露端口 
EXPOSE 5173 8888 
 
# 创建启动脚本 - 使用 echo 逐行写入 
RUN echo '#!/bin/sh' > /workspace/start.sh \ 
 && echo 'set -e' >> /workspace/start.sh \ 
 && echo 'echo "Starting container..."' >> /workspace/start.sh \ 
 && echo 'cd /workspace/frontend' >> /workspace/start.sh \ 
 && echo 'echo "Current directory: $(pwd)"' >> /workspace/start.sh \ 
 && echo 'ls -la' >> /workspace/start.sh \ 
 && echo 'echo "Starting Vite development server..."' >> /workspace/start.sh \ 
 && echo 'exec pnpm dev --host 0.0.0.0 --port 5173' >> /workspace/start.sh \ 
 && chmod +x /workspace/start.sh 
 
# 容器启动时运行前端项目 
CMD ["sh", "/workspace/start.sh"]
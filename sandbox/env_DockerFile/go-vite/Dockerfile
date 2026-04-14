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
 && printf 'package main\n\nimport (\n\t"fmt"\n\t"net/http"\n)\n\nfunc main() {\n\thttp.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintln(w, "backend service running")\n\t})\n\tfmt.Println("Backend listening on :8888")\n\tif err := http.ListenAndServe(":8888", nil); err != nil {\n\t\tfmt.Printf("Server error: %%v\\n", err)\n\t}\n}\n' > main.go

# ===================== 
# 初始化 frontend (Vite) 
# ===================== 
RUN mkdir -p frontend \ 
 && cd frontend \ 
 && pnpm create vite@latest . --template react-ts --force \ 
 && pnpm install 
 
# 暴露端口 
EXPOSE 5173 8888 
 
# 创建启动脚本
RUN echo '#!/bin/sh' > /workspace/start.sh \
 && echo 'set -e' >> /workspace/start.sh \
 && echo '' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo '# 清理占用端口' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo 'kill $(lsof -t -i:8888) 2>/dev/null || true' >> /workspace/start.sh \
 && echo 'kill $(lsof -t -i:5173) 2>/dev/null || true' >> /workspace/start.sh \
 && echo '' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo '# 配置 Go 代理环境' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo 'export GOPATH=/go' >> /workspace/start.sh \
 && echo 'export GOROOT=/usr/local/go' >> /workspace/start.sh \
 && echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> /workspace/start.sh \
 && echo 'export GOPROXY=https://goproxy.cn,https://goproxy.io,direct' >> /workspace/start.sh \
 && echo 'export GONOSUMCHECK=*' >> /workspace/start.sh \
 && echo 'export GONOSUMDB=*' >> /workspace/start.sh \
 && echo '' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo '# 1. 先后台启动 Vite 前端' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo 'echo "==> Starting Vite dev server in background..."' >> /workspace/start.sh \
 && echo 'cd /workspace/frontend' >> /workspace/start.sh \
 && echo 'pnpm dev --host 0.0.0.0 --port 5173 > /workspace/frontend/frontend.log 2>&1 &' >> /workspace/start.sh \
 && echo 'FRONTEND_PID=$!' >> /workspace/start.sh \
 && echo 'echo "Frontend started with PID: $FRONTEND_PID"' >> /workspace/start.sh \
 && echo '' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo '# 2. 再前台启动 Go 后端' >> /workspace/start.sh \
 && echo '# =====================' >> /workspace/start.sh \
 && echo 'echo "==> Running go mod tidy for backend..."' >> /workspace/start.sh \
 && echo 'cd /workspace/backend' >> /workspace/start.sh \
 && echo 'go mod tidy' >> /workspace/start.sh \
 && echo 'echo "==> Building backend..."' >> /workspace/start.sh \
 && echo 'go build -o /workspace/backend/server .' >> /workspace/start.sh \
 && echo 'echo "==> Starting backend in foreground..."' >> /workspace/start.sh \
 && echo 'exec /workspace/backend/server' >> /workspace/start.sh \
 && chmod +x /workspace/start.sh

# 容器启动时运行脚本
CMD ["sh", "/workspace/start.sh"]
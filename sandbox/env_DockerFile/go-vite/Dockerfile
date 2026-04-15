# 选择最新稳定 Go Alpine 镜像
FROM golang:1.25-alpine

# 安装系统依赖 + node + pnpm + python
RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    npm \
    python3 \
    py3-pip \
    lsof \
    sqlite \
    sqlite-dev \
 && npm install -g npm@latest \
 && npm install -g pnpm@latest \
 && ln -sf python3 /usr/bin/python

# 设置工作目录
WORKDIR /workspace

# =====================
# 初始化 backend (Go + SQLite)
# =====================
RUN mkdir -p backend/db \
 && cd backend \
 && go mod init backend \
 && printf 'package db\n\nimport (\n\t"database/sql"\n\t"fmt"\n\t"os"\n\t"path/filepath"\n\n\t_ "modernc.org/sqlite"\n)\n\nfunc Init() error {\n\tif err := os.MkdirAll("/workspace/backend/db", 0o755); err != nil {\n\t\treturn err\n\t}\n\n\tdbPath := filepath.Join("/workspace/backend/db", "app.db")\n\tdb, err := sql.Open("sqlite", dbPath)\n\tif err != nil {\n\t\treturn err\n\t}\n\tdefer db.Close()\n\n\tif _, err := db.Exec("CREATE TABLE IF NOT EXISTS health_check (id INTEGER PRIMARY KEY, status TEXT NOT NULL)"); err != nil {\n\t\treturn err\n\t}\n\n\tfmt.Printf("SQLite ready: %%s\\n", dbPath)\n\treturn nil\n}\n' > db/sqlite.go \
 && printf 'package main\n\nimport (\n\t"fmt"\n\t"net/http"\n\n\t"backend/db"\n)\n\nfunc main() {\n\tif err := db.Init(); err != nil {\n\t\tpanic(err)\n\t}\n\n\thttp.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintln(w, "backend service running")\n\t})\n\n\tfmt.Println("Backend listening on :8888")\n\tif err := http.ListenAndServe(":8888", nil); err != nil {\n\t\tfmt.Printf("Server error: %%v\\n", err)\n\t}\n}\n' > main.go

# =====================
# 初始化 frontend (Vite)
# =====================
RUN mkdir -p frontend \
 && cd frontend \
 && pnpm create vite@latest . --template react-ts --force \
 && pnpm install

# 暴露端口
EXPOSE 5173 8888

# 创建 /start.sh（根目录）容器启动脚本
RUN echo '#!/bin/sh' > /start.sh \
 && echo 'set -e' >> /start.sh \
 && echo '' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo '# 清理占用端口' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo 'kill $(lsof -t -i:8888) 2>/dev/null || true' >> /start.sh \
 && echo 'kill $(lsof -t -i:5173) 2>/dev/null || true' >> /start.sh \
 && echo '' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo '# 配置 Go 代理环境' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo 'export GOPATH=/go' >> /start.sh \
 && echo 'export GOROOT=/usr/local/go' >> /start.sh \
 && echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> /start.sh \
 && echo 'export GOPROXY=https://goproxy.cn,https://goproxy.io,direct' >> /start.sh \
 && echo 'export GONOSUMCHECK=*' >> /start.sh \
 && echo 'export GONOSUMDB=*' >> /start.sh \
 && echo '' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo '# 后台启动 Vite 前端' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo 'echo "==> Starting Vite dev server in background..."' >> /start.sh \
 && echo 'cd /workspace/frontend' >> /start.sh \
 && echo 'pnpm dev --host 0.0.0.0 --port 5173 > /workspace/frontend/frontend.log 2>&1 &' >> /start.sh \
 && echo 'FRONTEND_PID=$!' >> /start.sh \
 && echo 'echo "Frontend started with PID: $FRONTEND_PID"' >> /start.sh \
 && echo '' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo '# 后台构建并启动 Go 后端' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo 'echo "==> Running go mod tidy for backend..."' >> /start.sh \
 && echo 'cd /workspace/backend' >> /start.sh \
 && echo 'go mod tidy' >> /start.sh \
 && echo 'echo "==> Building backend..."' >> /start.sh \
 && echo 'go build -o /workspace/backend/server .' >> /start.sh \
 && echo 'echo "==> Starting backend in background..."' >> /start.sh \
 && echo '/workspace/backend/server > /workspace/backend/backend.log 2>&1 &' >> /start.sh \
 && echo 'BACKEND_PID=$!' >> /start.sh \
 && echo 'echo "Backend started with PID: $BACKEND_PID"' >> /start.sh \
 && echo '' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo '# wait 作为 PID 1 保活' >> /start.sh \
 && echo '# =====================' >> /start.sh \
 && echo 'echo "==> All services started, container is alive."' >> /start.sh \
 && echo 'wait' >> /start.sh \
 && chmod +x /start.sh

# 创建 /restart.sh 手动重启脚本
RUN echo '#!/bin/sh' > /restart.sh \
 && echo 'set -e' >> /restart.sh \
 && echo '' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo '# 按进程名精准杀掉服务' >> /restart.sh \
 && echo '# 不用 lsof 避免误杀 PID 1' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo 'echo "==> Stopping backend..."' >> /restart.sh \
 && echo 'pkill -f /workspace/backend/server 2>/dev/null || true' >> /restart.sh \
 && echo 'echo "==> Stopping frontend..."' >> /restart.sh \
 && echo 'pkill -f "pnpm dev" 2>/dev/null || true' >> /restart.sh \
 && echo '' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo '# 配置 Go 代理环境' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo 'export GOPATH=/go' >> /restart.sh \
 && echo 'export GOROOT=/usr/local/go' >> /restart.sh \
 && echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> /restart.sh \
 && echo 'export GOPROXY=https://goproxy.cn,https://goproxy.io,direct' >> /restart.sh \
 && echo 'export GONOSUMCHECK=*' >> /restart.sh \
 && echo 'export GONOSUMDB=*' >> /restart.sh \
 && echo '' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo '# 重启 Go 后端' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo 'echo "==> Rebuilding and restarting backend..."' >> /restart.sh \
 && echo 'cd /workspace/backend' >> /restart.sh \
 && echo 'go mod tidy' >> /restart.sh \
 && echo 'go build -o /workspace/backend/server .' >> /restart.sh \
 && echo '/workspace/backend/server > /workspace/backend/backend.log 2>&1 &' >> /restart.sh \
 && echo 'echo "Backend restarted with PID: $!"' >> /restart.sh \
 && echo '' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo '# 重启 Vite 前端' >> /restart.sh \
 && echo '# =====================' >> /restart.sh \
 && echo 'echo "==> Restarting Vite dev server..."' >> /restart.sh \
 && echo 'cd /workspace/frontend' >> /restart.sh \
 && echo 'pnpm dev --host 0.0.0.0 --port 5173 > /workspace/frontend/frontend.log 2>&1 &' >> /restart.sh \
 && echo 'echo "Frontend restarted with PID: $!"' >> /restart.sh \
 && echo 'echo "==> All services restarted."' >> /restart.sh \
 && chmod +x /restart.sh

# 容器启动时运行脚本
CMD ["sh", "/start.sh"]

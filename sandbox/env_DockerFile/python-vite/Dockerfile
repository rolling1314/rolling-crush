# 选择最新稳定 Python Alpine 镜像
FROM python:3.12-alpine

# 安装系统依赖 + node + pnpm
RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    npm \
 && npm install -g npm@latest \
 && npm install -g pnpm@latest

# 设置工作目录
WORKDIR /workspace

# =====================
# 初始化 backend (Python/Flask)
# =====================
RUN mkdir -p backend \
 && cd backend \
 && python -m venv venv \
 && echo 'flask' > requirements.txt \
 && echo 'from flask import Flask' > app.py \
 && echo '' >> app.py \
 && echo 'app = Flask(__name__)' >> app.py \
 && echo '' >> app.py \
 && echo '@app.route("/")' >> app.py \
 && echo 'def hello():' >> app.py \
 && echo '    return {"message": "Backend service running"}' >> app.py \
 && echo '' >> app.py \
 && echo 'if __name__ == "__main__":' >> app.py \
 && echo '    app.run(host="0.0.0.0", port=8000, debug=True)' >> app.py \
 && source venv/bin/activate \
 && pip install --no-cache-dir -r requirements.txt

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
 && echo 'echo "Starting container..."' >> /workspace/start.sh \
 && echo 'cd /workspace/frontend' >> /workspace/start.sh \
 && echo 'echo "Current directory: $(pwd)"' >> /workspace/start.sh \
 && echo 'ls -la' >> /workspace/start.sh \
 && echo 'echo "Starting Vite development server..."' >> /workspace/start.sh \
 && echo 'exec pnpm dev --host 0.0.0.0 --port 5173' >> /workspace/start.sh \
 && chmod +x /workspace/start.sh

# 容器启动时运行前端项目
CMD ["sh", "/workspace/start.sh"]
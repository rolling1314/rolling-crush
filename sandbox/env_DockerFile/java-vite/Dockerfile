# 选择 OpenJDK Alpine 镜像
FROM eclipse-temurin:21-jdk-alpine

# 安装系统依赖 + node + pnpm + maven + python
RUN apk add --no-cache \
    bash \
    curl \
    git \
    nodejs \
    npm \
    maven \
    python3 \
    py3-pip \
 && npm install -g npm@latest \
 && npm install -g pnpm@latest \
 && ln -sf python3 /usr/bin/python

# 设置工作目录
WORKDIR /workspace

# =====================
# 初始化 backend (Java/Spring Boot)
# =====================
RUN mkdir -p backend/src/main/java/com/example/demo \
 && cd backend \
 && echo '<?xml version="1.0" encoding="UTF-8"?>' > pom.xml \
 && echo '<project xmlns="http://maven.apache.org/POM/4.0.0"' >> pom.xml \
 && echo '         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"' >> pom.xml \
 && echo '         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0' >> pom.xml \
 && echo '         https://maven.apache.org/xsd/maven-4.0.0.xsd">' >> pom.xml \
 && echo '    <modelVersion>4.0.0</modelVersion>' >> pom.xml \
 && echo '    <groupId>com.example</groupId>' >> pom.xml \
 && echo '    <artifactId>demo</artifactId>' >> pom.xml \
 && echo '    <version>0.0.1-SNAPSHOT</version>' >> pom.xml \
 && echo '    <properties>' >> pom.xml \
 && echo '        <maven.compiler.source>21</maven.compiler.source>' >> pom.xml \
 && echo '        <maven.compiler.target>21</maven.compiler.target>' >> pom.xml \
 && echo '    </properties>' >> pom.xml \
 && echo '</project>' >> pom.xml \
 && echo 'package com.example.demo;' > src/main/java/com/example/demo/Main.java \
 && echo 'public class Main {' >> src/main/java/com/example/demo/Main.java \
 && echo '    public static void main(String[] args) {' >> src/main/java/com/example/demo/Main.java \
 && echo '        System.out.println("Backend service running");' >> src/main/java/com/example/demo/Main.java \
 && echo '    }' >> src/main/java/com/example/demo/Main.java \
 && echo '}' >> src/main/java/com/example/demo/Main.java

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
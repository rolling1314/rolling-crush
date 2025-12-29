# Crush JWT 认证系统 - 快速启动指南

## 🚀 快速开始

### 前置要求
- Go 1.25.0 或更高版本
- Node.js 18+ 和 pnpm
- 端口 8080, 8081, 5173 可用

### 1️⃣ 启动后端服务器

```bash
# 进入后端目录
cd crush-main

# 安装依赖（首次运行）
go mod download

# 运行服务器
go run main.go
```

**预期输出：**
```
Crush servers are running
HTTP Server: http://localhost:8081
WebSocket Server: ws://localhost:8080
Press Ctrl+C to stop.
```

### 2️⃣ 启动前端应用

新开一个终端窗口：

```bash
# 进入前端目录
cd crush-fe

# 安装依赖（首次运行）
pnpm install

# 启动开发服务器
pnpm dev
```

**预期输出：**
```
VITE v5.x.x  ready in xxx ms

➜  Local:   http://localhost:5173/
➜  Network: use --host to expose
```

### 3️⃣ 访问应用

1. 打开浏览器访问: **http://localhost:5173**
2. 你会看到登录页面
3. 使用以下测试账号登录：

   **管理员账号：**
   - 用户名: `admin`
   - 密码: `admin123`

   **普通用户账号：**
   - 用户名: `user`
   - 密码: `password123`

4. 登录成功后，你会看到聊天界面
5. 现在可以与 AI 助手交互了！

## 🔍 验证安装

### 检查后端服务

```bash
# 检查 HTTP 服务器健康状态
curl http://localhost:8081/health

# 预期响应
{"status":"healthy"}

# 测试登录接口
curl -X POST http://localhost:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 预期响应
{
  "success": true,
  "token": "eyJhbGc...",
  "message": "Login successful",
  "user": {
    "id": "...",
    "username": "admin"
  }
}
```

### 检查 WebSocket 连接

在浏览器的开发者工具（F12）中：
1. 打开 Console 标签
2. 登录后应该看到: `Connected to WebSocket`
3. 检查 Network 标签的 WS 部分，应该看到一个活跃的 WebSocket 连接

## 📁 项目结构

```
crush/
├── crush-main/          # 后端 Go 代码
│   ├── internal/
│   │   ├── auth/       # JWT 认证模块
│   │   ├── httpserver/ # HTTP API 服务器
│   │   ├── server/     # WebSocket 服务器
│   │   └── ...
│   └── main.go
│
├── crush-fe/           # 前端 React 代码
│   ├── src/
│   │   ├── components/
│   │   │   ├── LoginPage.tsx
│   │   │   ├── ChatPanel.tsx
│   │   │   └── ...
│   │   ├── App.tsx
│   │   └── main.tsx
│   └── package.json
│
└── JWT_AUTH_IMPLEMENTATION.md  # 详细文档
```

## 🔧 常见问题

### 问题 1: 后端无法启动
**错误**: `go: updates to go.mod needed`

**解决**:
```bash
cd crush-main
go mod tidy
go run main.go
```

### 问题 2: 前端无法连接到后端
**错误**: 登录时显示 "Unable to connect to server"

**检查**:
1. 后端是否正在运行？
   ```bash
   curl http://localhost:8081/health
   ```
2. 是否有防火墙阻止 8081 端口？
3. 浏览器控制台是否有 CORS 错误？

### 问题 3: WebSocket 连接失败
**错误**: WebSocket 连接立即断开

**检查**:
1. JWT token 是否有效？检查 localStorage
   ```javascript
   // 在浏览器控制台执行
   console.log(localStorage.getItem('jwt_token'))
   ```
2. 后端日志是否显示认证错误？
3. 是否使用了正确的 WebSocket URL？

### 问题 4: 编译错误
**错误**: Package 找不到

**解决**:
```bash
# 清理缓存
go clean -modcache

# 重新下载依赖
go mod download

# 重新编译
go build main.go
```

### 问题 5: 前端依赖安装失败
**解决**:
```bash
# 清理 node_modules
rm -rf node_modules pnpm-lock.yaml

# 重新安装
pnpm install
```

## 🎯 测试流程

### 1. 登录流程测试
1. 打开 http://localhost:5173
2. 输入错误的用户名/密码 → 应该显示错误信息
3. 输入正确的凭证 → 应该进入聊天界面
4. 刷新页面 → 应该保持登录状态（不需要重新登录）

### 2. WebSocket 通信测试
1. 登录后，在聊天框输入消息
2. 检查浏览器控制台，应该看到 `WS Message:` 日志
3. 应该收到 AI 的回复

### 3. 权限测试
1. 退出登录
2. 尝试直接访问聊天界面 → 应该自动跳转到登录页
3. 手动删除 localStorage 的 token → WebSocket 应该断开

## 🛡️ 安全说明

**当前实现适用于开发环境**，以下特性需要在生产环境中加强：

### 必须改进的部分：

1. **JWT Secret**
   - 当前: 随机生成或使用默认值
   - 生产: 从环境变量加载固定的强密钥

2. **密码哈希**
   - 当前: SHA-256
   - 生产: 使用 bcrypt 或 argon2

3. **用户存储**
   - 当前: 内存 map
   - 生产: PostgreSQL/MySQL 数据库

4. **CORS 配置**
   - 当前: 允许所有来源 (`*`)
   - 生产: 限制到特定域名

5. **HTTPS/WSS**
   - 当前: HTTP 和 WS
   - 生产: HTTPS 和 WSS（TLS 加密）

6. **Token 刷新**
   - 当前: 仅 access token，24 小时过期
   - 生产: 实现 refresh token 机制

## 📊 服务端口

| 服务 | 端口 | 用途 |
|------|------|------|
| HTTP Server | 8081 | 登录、API 请求 |
| WebSocket Server | 8080 | 实时聊天通信 |
| Frontend Dev Server | 5173 | 前端开发服务器 |

## 🔗 相关链接

- **详细文档**: [JWT_AUTH_IMPLEMENTATION.md](./JWT_AUTH_IMPLEMENTATION.md)
- **后端代码**: `crush-main/`
- **前端代码**: `crush-fe/`

## 💡 提示

### 开发技巧

1. **查看后端日志**
   ```bash
   # 运行后端时启用详细日志
   CRUSH_DEBUG=true go run main.go
   ```

2. **查看前端日志**
   - 打开浏览器开发者工具（F12）
   - 查看 Console 标签

3. **测试 API**
   ```bash
   # 使用 curl 测试
   curl -X POST http://localhost:8081/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"admin123"}'
   ```

4. **WebSocket 调试**
   - 在浏览器 Network 标签中查看 WS 连接
   - 查看发送和接收的消息

### 性能优化

1. **构建生产版本**
   ```bash
   # 后端
   cd crush-main
   go build -o crush main.go
   ./crush

   # 前端
   cd crush-fe
   pnpm build
   pnpm preview
   ```

2. **使用环境变量**
   ```bash
   # 创建 .env 文件
   echo "JWT_SECRET=your-secret-key-here" > .env
   ```

## 🎉 成功！

如果你看到了聊天界面并能够发送消息，恭喜！系统已经正常运行了。

接下来你可以：
- 探索代码结构
- 自定义用户界面
- 添加新功能
- 集成到生产环境

祝你使用愉快！🚀


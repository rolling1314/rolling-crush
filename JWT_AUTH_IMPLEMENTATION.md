# JWT 认证实现总结

## 架构概述

本项目实现了完整的前后端 JWT 认证系统，包括：

### 后端架构 (Go)

#### 1. 双服务器架构
- **HTTP 服务器** (端口 8081): 处理认证和 API 请求
- **WebSocket 服务器** (端口 8080): 处理实时聊天通信

#### 2. 核心模块

##### a. 认证模块 (`internal/auth/`)
- **jwt.go**: JWT token 生成和验证
  - `GenerateToken(userID, username)`: 生成有效期 24 小时的 JWT
  - `ValidateToken(tokenString)`: 验证并解析 JWT token
  - 使用 HS256 签名算法

- **user.go**: 用户管理
  - 内存存储用户数据（生产环境应使用数据库）
  - 预置测试用户：
    - `admin / admin123`
    - `user / password123`
  - SHA-256 密码哈希（建议生产环境使用 bcrypt）

- **middleware.go**: HTTP 认证中间件
  - 从 `Authorization: Bearer <token>` 提取 token
  - 验证 token 有效性

##### b. HTTP 服务器 (`internal/httpserver/`)
- **server.go**: RESTful API 服务器
  - `POST /api/auth/login`: 用户登录，返回 JWT token
  - `GET /api/auth/verify`: 验证 token 有效性（需要认证）
  - `GET /health`: 健康检查
  - CORS 支持（开发环境允许所有来源）

##### c. WebSocket 服务器 (`internal/server/`)
- **server.go**: WebSocket 实时通信服务器
  - JWT 认证在连接建立前完成
  - 支持两种 token 传递方式：
    1. Query parameter: `/ws?token=<jwt>`
    2. Authorization header: `Bearer <token>`
  - 广播消息给所有已连接客户端
  - 自动处理连接断开

#### 3. 启动流程
```
main.go
  └─> cmd.Execute()
      └─> root.go RunE()
          ├─> app.HTTPServer.Start() (goroutine, port 8081)
          └─> app.WSServer.Start("8080") (goroutine, port 8080)
```

### 前端架构 (React + TypeScript)

#### 1. 核心组件

##### a. LoginPage 组件 (`src/components/LoginPage.tsx`)
- 美观的现代化登录界面
- 表单验证和错误处理
- 连接到后端 `http://localhost:8081/api/auth/login`
- 登录成功后：
  - 将 JWT token 存储到 `localStorage`
  - 将用户名存储到 `localStorage`
  - 调用 `onLoginSuccess` 回调

##### b. App 组件 (`src/App.tsx`)
- **状态管理**:
  - `isAuthenticated`: 认证状态
  - `jwtToken`: JWT token
  - `currentUsername`: 当前用户名
  
- **认证流程**:
  1. 应用启动时检查 localStorage 中的 token
  2. 如果未认证，显示登录页面
  3. 登录成功后，建立 WebSocket 连接
  
- **WebSocket 连接**:
  ```javascript
  const wsUrl = `ws://localhost:8080/ws?token=${encodeURIComponent(jwtToken)}`;
  const ws = new WebSocket(wsUrl);
  ```
  
- **退出登录**:
  - 清除 localStorage
  - 关闭 WebSocket 连接
  - 返回登录页面

#### 2. 安全特性
- Token 自动从 localStorage 加载
- WebSocket 连接失败时的错误处理
- Token 过期时自动断开连接

## 数据流

### 登录流程
```
1. 用户输入用户名和密码
   └─> LoginPage

2. POST /api/auth/login
   └─> HTTP Server (8081)
       └─> auth.UserStore.Authenticate()
           └─> auth.GenerateToken()

3. 返回 { success: true, token: "...", user: {...} }
   └─> 前端存储 token 到 localStorage
       └─> 触发 onLoginSuccess
           └─> 设置 isAuthenticated = true
```

### WebSocket 连接流程
```
1. 前端发起 WebSocket 连接 (带 JWT token)
   └─> ws://localhost:8080/ws?token=xxx

2. WebSocket Server 验证 token
   └─> extractToken(r)
       └─> auth.ValidateToken(token)
           ├─> 成功: 升级连接
           └─> 失败: 返回 401 Unauthorized

3. 连接建立后
   ├─> 前端发送消息 -> app.HandleClientMessage()
   └─> 后端广播消息 -> 所有客户端
```

## 配置和运行

### 后端启动

```bash
cd crush-main
go run main.go
```

输出信息：
```
HTTP Server: http://localhost:8081
WebSocket Server: ws://localhost:8080
```

### 前端启动

```bash
cd crush-fe
pnpm install
pnpm dev
```

访问: `http://localhost:5173`

### 测试账号
- 用户名: `admin`, 密码: `admin123`
- 用户名: `user`, 密码: `password123`

## 安全建议（生产环境）

### 后端
1. **JWT Secret**: 
   - 当前: 随机生成或使用默认值
   - 建议: 从环境变量加载 `JWT_SECRET`
   ```go
   jwtSecret = []byte(os.Getenv("JWT_SECRET"))
   ```

2. **密码哈希**:
   - 当前: SHA-256
   - 建议: 使用 bcrypt 或 argon2
   ```go
   import "golang.org/x/crypto/bcrypt"
   hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
   ```

3. **用户存储**:
   - 当前: 内存 map
   - 建议: 使用数据库（PostgreSQL, MySQL 等）

4. **CORS 配置**:
   - 当前: 允许所有来源
   - 建议: 限制到特定域名
   ```go
   w.Header().Set("Access-Control-Allow-Origin", "https://yourdomain.com")
   ```

5. **Token 过期时间**:
   - 当前: 24 小时
   - 建议: 实现 refresh token 机制

6. **HTTPS**:
   - 使用 TLS/SSL 证书
   - WebSocket 使用 WSS 协议

### 前端
1. **Token 存储**:
   - 当前: localStorage
   - 考虑: httpOnly cookies（更安全）

2. **XSS 防护**:
   - 使用 React 的自动转义
   - 不使用 `dangerouslySetInnerHTML`

3. **Token 刷新**:
   - 实现 token 自动刷新机制
   - 检测 token 即将过期并刷新

## API 文档

### POST /api/auth/login
**请求**:
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**响应** (成功):
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "message": "Login successful",
  "user": {
    "id": "507c7f79bcf86cd7994f6c0e",
    "username": "admin"
  }
}
```

**响应** (失败):
```json
{
  "success": false,
  "message": "Invalid username or password"
}
```

### GET /api/auth/verify
**请求头**:
```
Authorization: Bearer <jwt_token>
```

**响应** (成功):
```json
{
  "valid": true,
  "message": "Token is valid"
}
```

### WebSocket /ws?token=<jwt_token>
**连接**: 
```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=' + jwtToken);
```

**发送消息**:
```json
{
  "content": "Hello, AI!"
}
```

**接收消息**:
```json
{
  "ID": "msg_123",
  "Role": "assistant",
  "Parts": [
    {
      "text": "Hello! How can I help you?"
    }
  ],
  "CreatedAt": 1234567890
}
```

## 故障排查

### 前端无法连接到后端
1. 检查后端是否运行：`curl http://localhost:8081/health`
2. 检查防火墙设置
3. 检查浏览器控制台的 CORS 错误

### WebSocket 连接被拒绝
1. 检查 token 是否有效
2. 检查后端日志中的 JWT 验证错误
3. 确认 token 正确编码：`encodeURIComponent(token)`

### 登录失败
1. 确认用户名和密码正确
2. 检查后端日志
3. 验证 HTTP 服务器是否在 8081 端口运行

## 代码文件清单

### 后端
- `internal/auth/jwt.go` - JWT 生成和验证
- `internal/auth/user.go` - 用户管理
- `internal/auth/middleware.go` - HTTP 认证中间件
- `internal/httpserver/server.go` - HTTP API 服务器
- `internal/server/server.go` - WebSocket 服务器
- `internal/app/app.go` - 应用程序主类
- `internal/cmd/root.go` - 命令行入口
- `main.go` - 程序入口

### 前端
- `src/App.tsx` - 主应用组件
- `src/components/LoginPage.tsx` - 登录页面
- `src/components/ChatPanel.tsx` - 聊天面板
- `src/components/FileTree.tsx` - 文件树
- `src/components/CodeEditor.tsx` - 代码编辑器
- `src/types.ts` - TypeScript 类型定义

## 下一步改进

1. [ ] 实现用户注册功能
2. [ ] 添加密码重置功能
3. [ ] 实现 refresh token 机制
4. [ ] 将用户数据迁移到数据库
5. [ ] 添加用户权限和角色管理
6. [ ] 实现会话管理（踢出用户、查看在线用户）
7. [ ] 添加日志审计功能
8. [ ] 实现 rate limiting（速率限制）
9. [ ] 添加 2FA（双因素认证）
10. [ ] 性能监控和指标收集


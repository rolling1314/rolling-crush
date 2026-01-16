# Crush 服务架构文档

本文档描述了 Crush 项目的两个核心服务：HTTP API 服务和 WebSocket 实时通信服务。

## 目录

- [概述](#概述)
- [HTTP Server](#http-server)
  - [目录结构](#http-server-目录结构)
  - [功能特性](#http-server-功能特性)
  - [API 路由](#api-路由)
  - [启动与配置](#http-server-启动与配置)
- [WebSocket Server](#websocket-server)
  - [目录结构](#websocket-server-目录结构)
  - [功能特性](#websocket-server-功能特性)
  - [消息处理](#websocket-消息处理)
  - [启动与配置](#websocket-server-启动与配置)
- [共享组件](#共享组件)
- [部署说明](#部署说明)

---

## 概述

Crush 采用微服务架构，将功能拆分为两个独立服务：

1. **HTTP Server** (`cmd/http-server`): 提供 RESTful API，处理标准的 HTTP 请求
2. **WebSocket Server** (`cmd/ws-server`): 提供 WebSocket 连接，处理实时通信和 Agent 交互

两个服务共享配置文件和数据库连接，但可以独立部署和扩展。

---

## HTTP Server

HTTP Server 是 Crush 的 RESTful API 服务，负责处理所有标准 HTTP 请求。

### HTTP Server 目录结构

```
cmd/http-server/
├── main.go              # 服务入口点
├── app/
│   └── app.go          # HTTP 应用初始化和配置
└── handler/
    ├── server.go       # HTTP 服务器主逻辑和路由定义
    ├── middleware.go   # 中间件（CORS 等）
    ├── helpers.go      # 辅助函数
    ├── types.go        # 类型定义
    ├── handler_auth.go      # 认证相关处理器
    ├── handler_project.go   # 项目管理处理器
    ├── handler_session.go   # 会话管理处理器
    ├── handler_toolcall.go  # 工具调用处理器
    ├── handler_file.go      # 文件操作处理器
    ├── handler_provider.go  # 模型提供商处理器
    └── handler_health.go    # 健康检查处理器
```

### HTTP Server 功能特性

- **认证与授权**
  - 用户注册和登录
  - JWT Token 认证
  - GitHub OAuth 集成
  - Token 验证

- **项目管理**
  - 创建、查询、更新、删除项目
  - 项目会话列表查询

- **会话管理**
  - 创建和管理会话
  - 查询会话消息
  - 会话配置管理
  - 工具调用查询

- **文件操作**
  - 文件列表查询
  - 图片上传

- **模型提供商**
  - 提供商列表查询
  - 模型列表查询
  - 提供商连接测试
  - 提供商配置

### API 路由

#### 健康检查
- `GET /health` - 服务健康检查

#### 认证路由 (`/api/auth`)
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/login` - 用户登录
- `GET /api/auth/verify` - Token 验证 (需要认证)
- `GET /api/auth/github` - GitHub OAuth 登录
- `GET /api/auth/github/callback` - GitHub OAuth 回调
- `GET /auth/github/callback` - GitHub OAuth 回调（根路径，用于匹配 GitHub OAuth 应用配置）

#### 项目管理路由 (`/api/projects`) - 需要认证
- `POST /api/projects` - 创建项目
- `GET /api/projects` - 获取项目列表
- `GET /api/projects/:id` - 获取项目详情
- `PUT /api/projects/:id` - 更新项目
- `DELETE /api/projects/:id` - 删除项目
- `GET /api/projects/:id/sessions` - 获取项目的会话列表

#### 会话管理路由 (`/api/sessions`) - 需要认证
- `POST /api/sessions` - 创建会话
- `GET /api/sessions/:id/messages` - 获取会话消息列表
- `GET /api/sessions/:id/config` - 获取会话配置
- `PUT /api/sessions/:id/config` - 更新会话配置
- `DELETE /api/sessions/:id` - 删除会话
- `GET /api/sessions/:id/tool-calls` - 获取会话的工具调用列表
- `GET /api/sessions/:id/tool-calls/pending` - 获取待处理的工具调用
- `GET /api/sessions/:id/tool-calls/:toolCallId` - 获取特定工具调用详情

#### 模型提供商路由 (`/api/providers`) - 需要认证
- `GET /api/providers` - 获取提供商列表
- `GET /api/providers/:provider/models` - 获取特定提供商的模型列表
- `POST /api/providers/test-connection` - 测试提供商连接
- `POST /api/providers/configure` - 配置提供商

#### 其他路由 - 需要认证
- `GET /api/auto-model` - 获取自动模型配置
- `GET /api/files` - 获取文件列表
- `POST /api/upload` - 上传图片

### HTTP Server 启动与配置

#### 启动方式

```bash
cd crush-main
go run cmd/http-server/main.go
```

#### 环境变量

- `CRUSH_CWD`: 工作目录（可选）
- `CRUSH_DATA_DIR`: 数据目录（可选）
- `CRUSH_PROFILE`: 启用 pprof 性能分析（端口 6060）

#### 配置说明

服务从 `config.yaml` 读取配置：
- HTTP 服务端口：`server.http_port`
- 数据库连接配置
- 存储服务配置（MinIO）
- 沙箱服务配置

#### 服务特性

- **CORS 支持**: 自动处理跨域请求
- **JWT 认证**: 使用 Bearer Token 进行身份验证
- **pprof 集成**: 支持性能分析（通过环境变量启用）
- **优雅关闭**: 支持信号处理和资源清理

---

## WebSocket Server

WebSocket Server 是 Crush 的实时通信服务，处理 WebSocket 连接和 Agent 交互。

### WebSocket Server 目录结构

```
cmd/ws-server/
├── main.go              # 服务入口点
├── app/
│   ├── app.go          # WebSocket 应用初始化和配置
│   ├── client.go       # 客户端消息处理
│   ├── agent.go        # Agent 协调器初始化
│   ├── events.go       # 事件处理
│   ├── lsp.go          # LSP 客户端管理
│   ├── lsp_events.go   # LSP 事件处理
│   ├── session_config.go # 会话配置管理
│   └── noninteractive.go # 非交互模式处理
└── handler/
    └── server.go       # WebSocket 服务器实现
```

### WebSocket Server 功能特性

- **实时通信**
  - WebSocket 连接管理
  - 消息广播和单播
  - 会话级别的消息路由

- **Agent 协调**
  - Agent 协调器初始化和管理
  - 多会话 Agent 支持
  - Agent 任务调度

- **LSP 集成**
  - 多语言 LSP 客户端管理
  - LSP 事件处理
  - 代码补全和诊断

- **事件系统**
  - 发布/订阅事件处理
  - Redis 流服务集成
  - 消息缓冲（连接断开时）

- **权限管理**
  - 工具使用权限控制
  - 操作权限验证

### WebSocket 消息处理

#### 连接流程

1. **连接建立**
   - 客户端通过 `ws://host:port/ws?token=<jwt_token>&session_id=<session_id>` 建立连接
   - 服务器验证 JWT Token
   - 服务器验证会话 ID
   - 连接成功，建立映射关系

2. **消息接收**
   - 客户端发送消息到服务器
   - 服务器通过注册的 `MessageHandler` 处理消息
   - 消息经过 Agent 协调器处理

3. **消息发送**
   - 服务器可以通过 `Broadcast()` 广播消息到所有客户端
   - 服务器可以通过 `SendToSession()` 发送消息到特定会话的客户端
   - 支持 JSON 格式消息

4. **连接断开**
   - 服务器检测到连接断开
   - 调用 `DisconnectHandler` 清理资源
   - 清理 Agent 状态和 LSP 客户端

#### 认证机制

WebSocket 连接需要 JWT Token 认证：
- Token 可以从 `Authorization` 请求头获取（Bearer Token）
- Token 也可以从查询参数 `token` 获取（URL 编码）

#### 会话管理

- 每个 WebSocket 连接关联一个 `session_id`
- 服务器支持更新客户端的会话 ID
- 消息可以按会话 ID 路由到特定客户端

### WebSocket Server 启动与配置

#### 启动方式

```bash
cd crush-main
go run cmd/ws-server/main.go
```

#### 环境变量

- `CRUSH_CWD`: 工作目录（可选）
- `CRUSH_DATA_DIR`: 数据目录（可选）
- `CRUSH_PROFILE`: 启用 pprof 性能分析（端口 6061）
- `CRUSH_YOLO`: 跳过权限请求（设置为 "true"）

#### 配置说明

服务从 `config.yaml` 读取配置：
- WebSocket 服务端口：`server.ws_port`
- 数据库连接配置
- Redis 配置（用于消息缓冲）
- Agent 配置
- LSP 配置
- 存储服务配置（MinIO）
- 沙箱服务配置

#### 服务特性

- **多客户端支持**: 支持多个并发 WebSocket 连接
- **会话隔离**: 每个会话的消息独立路由
- **消息缓冲**: 使用 Redis Stream 在连接断开时缓冲消息
- **Agent 集成**: 完整的 Agent 协调器支持
- **LSP 支持**: 多语言 LSP 客户端管理
- **优雅关闭**: 支持信号处理、资源清理和 Agent 取消

---

## 共享组件

两个服务共享以下组件：

### 数据库层

- **PostgreSQL**: 使用 `infra/postgres` 包进行数据库操作
- **SQLC**: 类型安全的 SQL 代码生成
- **迁移**: 使用数据库迁移管理 schema

### 领域服务

- `domain/user`: 用户服务
- `domain/project`: 项目服务
- `domain/session`: 会话服务
- `domain/message`: 消息服务
- `domain/toolcall`: 工具调用服务
- `domain/permission`: 权限服务
- `domain/history`: 历史记录服务

### 基础设施

- `infra/storage`: 对象存储（MinIO）客户端
- `infra/sandbox`: 沙箱服务客户端
- `infra/redis`: Redis 客户端和流服务
- `internal/shared`: 共享初始化逻辑

### 配置管理

- `pkg/config`: 统一配置管理
- `config.yaml`: 服务配置文件

---

## 部署说明

### 独立部署

两个服务可以独立部署，提高可扩展性和容错性：

```bash
# 启动 HTTP Server
./crush http-server

# 启动 WebSocket Server
./crush ws-server
```

### 配置要求

两个服务需要访问相同的：
- PostgreSQL 数据库
- Redis（WebSocket Server 必需，HTTP Server 可选）
- 配置文件 (`config.yaml`)

### 端口配置

在 `config.yaml` 中配置：

```yaml
server:
  http_port: "8080"  # HTTP Server 端口
  ws_port: "8081"    # WebSocket Server 端口
```

### 监控和调试

- **pprof**: 通过环境变量 `CRUSH_PROFILE=1` 启用
  - HTTP Server: `http://localhost:6060/debug/pprof/`
  - WebSocket Server: `http://localhost:6061/debug/pprof/`

- **日志**: 使用 `slog` 进行结构化日志记录

### 生产环境建议

1. **负载均衡**: 为 HTTP Server 配置负载均衡器
2. **WebSocket 代理**: 使用 Nginx 或其他代理处理 WebSocket 升级
3. **健康检查**: 使用 `/health` 端点进行健康检查
4. **资源限制**: 合理配置连接数和内存限制
5. **监控**: 集成监控系统（Prometheus、Grafana 等）

---

## 总结

Crush 的架构通过分离 HTTP 和 WebSocket 服务实现了：

- **关注点分离**: RESTful API 和实时通信分离
- **独立扩展**: 可以根据负载独立扩展服务
- **高可用性**: 一个服务的故障不影响另一个服务
- **清晰的职责**: 每个服务职责明确，易于维护

这种架构设计使得系统更加灵活、可扩展，并且便于开发和运维。
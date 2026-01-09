# Sandbox 数据库集成说明

## 功能概述

沙箱服务现已集成 PostgreSQL 数据库，实现以下功能：

1. **自动查询项目信息**：根据会话 ID 查询对应的项目信息
2. **智能容器管理**：
   - 如果项目已有 `container_name`，自动连接到现有容器
   - 否则创建新容器
3. **与主服务一致**：使用与 `crush-main` 相同的数据库连接配置

## 数据库连接配置

### 环境变量（与 Go 代码一致）

```bash
export POSTGRES_HOST="localhost"       # 默认: localhost
export POSTGRES_PORT="5432"            # 默认: 5432
export POSTGRES_USER="crush"           # 默认: crush
export POSTGRES_PASSWORD="123456"      # 默认: 123456
export POSTGRES_DB="crush"             # 默认: crush
export POSTGRES_SSLMODE="disable"      # 默认: disable
```

### 安装依赖

```bash
cd sandbox
pip install -r requirements.txt
```

依赖包括：
- `Flask` - Web 框架
- `docker` - Docker 客户端
- `psycopg2-binary` - PostgreSQL 驱动

## 工作流程

### 1. 有数据库连接时

```
客户端请求 (session_id)
    ↓
查询数据库 (sessions → projects)
    ↓
检查 container_name
    ↓
├─ 有容器名称 → 连接到现有容器
└─ 无容器名称 → 创建新容器
```

### 2. 无数据库连接时

```
客户端请求 (session_id)
    ↓
创建新容器（独立模式）
```

## 数据库表结构

### sessions 表

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    title TEXT,
    -- 其他字段...
);
```

### projects 表

```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT,
    container_name TEXT,      -- 容器名称（新增）
    workdir_path TEXT,        -- 工作目录路径（新增）
    workspace_path TEXT,
    -- 其他字段...
);
```

## 使用示例

### 启动服务

```bash
# 方式1: 使用环境变量
export POSTGRES_HOST="localhost"
export POSTGRES_USER="crush"
export POSTGRES_PASSWORD="123456"
python main.py server

# 方式2: Docker Compose（推荐）
docker-compose up sandbox
```

### 测试连接

```bash
curl http://localhost:8888/health
```

响应：
```json
{
  "status": "ok",
  "active_sessions": 0
}
```

### 执行代码（带数据库查询）

```bash
curl -X POST http://localhost:8888/execute \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "your-session-id",
    "command": "ls -la",
    "language": "bash"
  }'
```

流程：
1. 沙箱服务收到请求
2. 查询数据库获取会话对应的项目
3. 如果项目有 `container_name`，连接到该容器
4. 在容器中执行命令
5. 返回结果

## 日志输出示例

### 成功连接数据库

```
✅ 数据库连接成功: crush@localhost:5432/crush
🚀 沙箱服务启动在 http://0.0.0.0:8888
📊 数据库: 已连接 (crush@localhost:5432/crush)
   智能模式: 自动查询项目容器信息
```

### 连接到现有容器

```
📨 [/execute] 收到请求
   会话ID: abc123
🔗 连接到现有容器: my-project-container (会话: abc123)
   工作目录: /workspace
✅ 已连接到容器: my-project-container
   状态: running
   工作目录: /workspace
```

### 创建新容器

```
📨 [/execute] 收到请求
   会话ID: xyz789
🆕 创建新沙箱容器 (会话: xyz789)
   项目: My Project
🚀 正在启动沙箱 (镜像: python:3.11-slim)...
✅ 沙箱已启动 (容器ID: d3a8b9c4)
```

## 故障处理

### 数据库连接失败

服务会自动降级为独立模式：

```
⚠️ 数据库连接失败: connection refused
   将以独立模式运行（不连接数据库）
📊 数据库: 未连接，运行在独立模式
```

### 容器不存在

如果数据库中指定的容器不存在：

```
❌ 容器 my-container 不存在，将创建新容器
🚀 正在启动沙箱 (镜像: python:3.11-slim)...
```

## 优势

1. **性能提升**：复用现有容器，避免重复创建
2. **状态保持**：会话之间共享项目容器，保持文件和环境
3. **资源优化**：减少容器数量，降低系统负载
4. **灵活性**：支持独立模式和集成模式

## 注意事项

1. 确保数据库与主服务连接到同一个 PostgreSQL 实例
2. 容器名称必须唯一，避免冲突
3. 数据库连接失败不会影响服务启动，会降级为独立模式
4. 建议在生产环境中启用 SSL 连接（`POSTGRES_SSLMODE=require`）

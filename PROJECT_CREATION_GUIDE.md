# 项目创建功能使用指南

## 功能概述

现在可以通过前端界面快速创建项目，系统会自动：
1. 在沙箱服务中启动 Docker 容器
2. 动态分配可用端口
3. 将容器信息和端口保存到数据库

## 前端界面更新

创建项目表单已简化，只需填写：

1. **项目名称** (必填)
   - 输入项目的名称，如 "My Awesome App"

2. **后端语言** (可选)
   - None: 纯前端项目（只有 Vite）
   - Go: 前端 + Go 后端
   - Java: 前端 + Java 后端
   - Python: 前端 + Python 后端

3. **需要数据库** (可选)
   - 仅在选择了后端语言时显示
   - 勾选后会配置 PostgreSQL 数据库连接

## 后端实现

### 1. 沙箱服务 API (`sandbox/main.py`)

新增接口: `POST /projects/create`

**请求参数:**
```json
{
  "project_name": "My Project",
  "backend_language": "go",  // "", "go", "java", "python"
  "need_database": true
}
```

**响应:**
```json
{
  "status": "ok",
  "container_id": "abc123...",
  "container_name": "my-project-1234567890",
  "frontend_port": 8123,
  "backend_port": 8567,
  "image": "go-vite-dev",
  "message": "Project container created successfully"
}
```

**功能:**
- 根据语言选择对应的 Docker 镜像：
  - 纯前端: `vite-dev`
  - Go: `go-vite-dev`
  - Java: `java-vite-dev`
  - Python: `python-vite-dev`
- 自动查找可用端口（主机端口范围: 8000-9000）
- 容器内固定端口: 5173 (前端), 8888 (后端)
- 启动容器并返回容器ID和分配的端口

### 2. 沙箱客户端 (`crush-main/internal/sandbox/client.go`)

新增方法: `CreateProject`

**类型定义:**
```go
type CreateProjectRequest struct {
    ProjectName     string `json:"project_name"`
    BackendLanguage string `json:"backend_language,omitempty"`
    NeedDatabase    bool   `json:"need_database"`
}

type CreateProjectResponse struct {
    Status        string `json:"status"`
    ContainerID   string `json:"container_id"`
    ContainerName string `json:"container_name"`
    FrontendPort  int32  `json:"frontend_port"`
    BackendPort   *int32 `json:"backend_port,omitempty"`
    Image         string `json:"image"`
    Message       string `json:"message"`
    Error         string `json:"error,omitempty"`
}
```

### 3. HTTP 服务器 (`crush-main/internal/httpserver/server.go`)

更新 `handleCreateProject` 方法：
1. 接收前端请求（项目名称、后端语言、是否需要数据库）
2. 调用沙箱服务创建容器
3. 创建项目记录
4. 将容器信息（container_name, container_id, 端口等）保存到数据库
5. 如果需要数据库，配置数据库连接信息

## 数据库字段

项目表 (`projects`) 中保存的信息：
- `container_name`: 容器名称
- `container_id`: 容器ID（暂存在 workdir_path 字段，后续可添加专门字段）
- `frontend_port`: 前端端口（主机端口）
- `backend_port`: 后端端口（主机端口，如果有）
- `backend_language`: 后端语言
- `frontend_language`: "vite" (固定)
- `db_host`, `db_port`, `db_user`, `db_password`, `db_name`: 数据库配置（如果需要）

## 测试步骤

### 前置条件

1. 确保沙箱服务运行:
```bash
cd /Users/apple/rolling-crush/sandbox
python main.py
```

2. 确保后端服务运行:
```bash
cd /Users/apple/rolling-crush/crush-main
go run cmd/server/main.go
```

3. 确保前端服务运行:
```bash
cd /Users/apple/rolling-crush/crush-fe
npm run dev
```

4. 准备 Docker 镜像:
```bash
# 根据需要构建对应的镜像
docker build -t vite-dev ./docker/vite
docker build -t go-vite-dev ./docker/go-vite
docker build -t python-vite-dev ./docker/python-vite
docker build -t java-vite-dev ./docker/java-vite
```

### 测试流程

1. 打开前端界面: http://localhost:5173
2. 登录账号
3. 点击 "New Project" 按钮
4. 填写项目名称，如 "Test Project"
5. 选择后端语言（可选）
6. 如果选择了后端语言，可勾选 "Need Database"
7. 点击 "Create Project"
8. 等待创建完成（容器启动可能需要几秒）

### 验证结果

1. **检查前端**:
   - 项目列表中应该出现新创建的项目
   
2. **检查数据库**:
```sql
SELECT id, name, container_name, frontend_port, backend_port, backend_language 
FROM projects 
ORDER BY created_at DESC 
LIMIT 1;
```

3. **检查 Docker 容器**:
```bash
docker ps | grep <project-name>
```

4. **检查端口**:
```bash
# 前端端口
curl http://localhost:<frontend_port>

# 后端端口（如果有）
curl http://localhost:<backend_port>
```

## 常见问题

### 1. 镜像不存在错误
```
Docker image 'xxx-vite-dev' not found. Please build it first.
```
**解决**: 构建对应的 Docker 镜像

### 2. 端口被占用
沙箱服务会自动查找可用端口，如果 8000-9000 范围内端口都被占用，会返回错误。

### 3. 容器启动失败
检查 Docker 服务是否运行:
```bash
docker info
```

## 后续优化建议

1. 添加项目模板选择（不同的前端框架和后端框架）
2. 支持自定义端口范围
3. 添加容器健康检查
4. 支持容器日志查看
5. 添加容器资源限制配置
6. 支持容器的启动/停止/重启操作
7. 添加项目配置的可视化编辑

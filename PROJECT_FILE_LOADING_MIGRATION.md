# 项目文件加载迁移：从Session ID到Project ID

## 概述

本次修改将项目代码文件的加载从基于 `session_id` 改为基于 `project_id`，使得项目代码加载与项目本身绑定，而不是与特定的会话绑定。这样更符合直觉，也更便于管理。

## 修改的文件

### 1. Sandbox服务（Python）

#### `sandbox/database.py`
- **新增方法**：`get_project_by_id(project_id)`
  - 直接通过项目ID查询项目信息
  - 返回项目的容器名称、工作目录等信息

#### `sandbox/session_manager.py`
- **新增方法**：`get_or_create_by_project(project_id)`
  - 通过项目ID获取或连接到项目容器
  - 使用缓存key `"project:{project_id}"` 来缓存项目级别的容器连接
  - 与原有的 `get_or_create(session_id)` 方法并行，保持向后兼容

#### `sandbox/routes/file_ops.py`
- **修改端点**：`GET /file/tree`
  - 现在同时支持 `session_id` 和 `project_id` 参数
  - 优先使用 `project_id`（新方式）
  - 保持对 `session_id` 的向后兼容

### 2. 后端服务（Go）

#### `crush-main/sandbox/client.go`
- **修改结构体**：`FileTreeRequest`
  - 添加 `ProjectID` 字段（可选）
  - 保留 `SessionID` 字段（可选，向后兼容）
- **修改方法**：`GetFileTree()`
  - 优先使用 `ProjectID` 构建请求URL
  - 如果没有 `ProjectID`，则使用 `SessionID`（向后兼容）
  - 两者都不存在时返回错误

#### `crush-main/api/http/handler_file.go`
- **修改处理函数**：`handleGetFiles()`
  - 同时接受 `session_id` 和 `project_id` 查询参数
  - 优先使用 `project_id`
  - 传递给 sandbox client

### 3. 前端服务（TypeScript/React）

#### `crush-fe/src/pages/WorkspacePage.tsx`
- **修改函数**：`loadFiles()`
  - 参数改为 `projectIdOrSessionId`（更清晰的命名）
  - 使用 `project_id` 参数调用后端API
  - 更新日志输出
- **修改useEffect**：
  - 依赖从 `currentSessionId` 改为 `projectId`
  - 确保在项目加载时就能加载文件树
- **修改WebSocket消息处理**：
  - 工具调用结果后刷新文件树时使用 `project.id` 而不是 `session_id`

## 工作流程

### 之前的流程（基于Session ID）
```
前端 → 后端API → Sandbox Client → Sandbox服务
      (session_id) → (session_id) → (session_id)
                                    ↓
                      查询: session → project → container
```

### 现在的流程（基于Project ID）
```
前端 → 后端API → Sandbox Client → Sandbox服务
      (project_id) → (project_id) → (project_id)
                                    ↓
                      查询: project → container
```

## 优势

1. **更直观**：项目的文件应该与项目绑定，而不是与会话绑定
2. **简化逻辑**：减少一层间接查询（不需要先查session再查project）
3. **更高效**：直接使用项目ID查询，减少数据库查询
4. **向后兼容**：保留了对 `session_id` 的支持，不会破坏现有功能

## 向后兼容性

所有修改都保持了向后兼容：
- Sandbox服务的 `/file/tree` 端点同时接受 `session_id` 和 `project_id`
- Go的 `FileTreeRequest` 结构体同时包含两个字段
- 后端API处理函数同时处理两种参数

这意味着：
- 旧的代码仍然可以使用 `session_id` 正常工作
- 新的代码使用 `project_id`（推荐）
- 渐进式迁移，无需一次性修改所有代码

## 测试建议

1. **测试新建项目**：
   - 创建一个新项目
   - 进入项目页面
   - 验证文件树正确加载

2. **测试文件刷新**：
   - 在会话中执行工具调用（如编辑文件）
   - 验证文件树自动刷新

3. **测试多会话**：
   - 在同一项目下创建多个会话
   - 切换会话时验证文件树保持一致（因为都是同一个项目）

4. **测试向后兼容**：
   - 如果有使用 `session_id` 的旧代码，验证其仍然正常工作

## 部署说明

部署顺序（自下而上）：
1. 首先部署 Sandbox 服务（Python）
2. 然后部署后端服务（Go）
3. 最后部署前端服务（React）

这个顺序确保了向后兼容性，即使部署过程中有短暂的版本不一致，系统仍能正常工作。

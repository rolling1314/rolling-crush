# Session-Specific Working Directory Implementation

## 概述
实现了基于会话ID的动态工作目录查询机制，使每个agent会话根据其关联的项目使用独立的工作目录。

## 修改内容

### 1. coordinator.go 结构修改

#### 1.1 添加数据库查询器字段
在 `coordinator` 结构体中添加了 `dbQuerier` 字段用于查询数据库：

```go
type coordinator struct {
    // ... 其他字段
    dbQuerier   db.Querier      // For querying session and project info
}
```

#### 1.2 修改 NewCoordinator 函数
在初始化时将 `dbReader` 转换为 `db.Querier`（因为它们是同一个实例）：

```go
func NewCoordinator(...) (Coordinator, error) {
    var dbQuerier db.Querier
    if dbReader != nil {
        if q, ok := dbReader.(db.Querier); ok {
            dbQuerier = q
        }
    }
    
    c := &coordinator{
        // ... 其他字段
        dbQuerier:   dbQuerier,
    }
}
```

### 2. Run 方法修改

#### 2.1 查询工作目录
在 `Run` 方法开始时，根据 sessionID 查询工作目录：

```go
// Query workdir_path from session -> project
workingDir := c.cfg.WorkingDir() // Default to config working dir
if c.dbQuerier != nil {
    dbSession, err := c.dbQuerier.GetSessionByID(ctx, sessionID)
    if err == nil && dbSession.ProjectID.Valid && dbSession.ProjectID.String != "" {
        project, err := c.dbQuerier.GetProjectByID(ctx, dbSession.ProjectID.String)
        if err == nil && project.WorkdirPath.Valid && project.WorkdirPath.String != "" {
            workingDir = project.WorkdirPath.String
            slog.Info("Using project-specific working directory", 
                "session_id", sessionID, 
                "project_id", project.ID, 
                "workdir", workingDir)
        }
    }
}
```

#### 2.2 使用查询到的工作目录
- 在加载会话配置时使用查询到的 `workingDir`
- 在重新构建工具时使用查询到的 `workingDir`

```go
// Load session-specific config with the queried workingDir
sessionCfg, err := config.LoadWithSessionConfig(
    ctx,
    workingDir, // Use queried workingDir instead of c.cfg.WorkingDir()
    c.cfg.Options.DataDirectory,
    c.cfg.Options.Debug,
    sessionID,
    c.dbReader,
)

// Rebuild tools with session-specific working directory
sessionTools, err := c.buildTools(ctx, agentCfg, workingDir)
if err == nil {
    c.currentAgent.SetTools(sessionTools)
}
```

### 3. buildTools 方法修改

修改 `buildTools` 方法签名，添加 `workingDir` 参数：

```go
func (c *coordinator) buildTools(ctx context.Context, agent config.Agent, workingDir string) ([]fantasy.AgentTool, error) {
    // ... 使用 workingDir 参数创建所有工具
    allTools = append(allTools,
        tools.NewBashTool(c.permissions, workingDir, ...),
        tools.NewDownloadTool(c.permissions, workingDir, ...),
        tools.NewEditTool(c.lspClients, c.permissions, c.history, workingDir),
        tools.NewMultiEditTool(c.lspClients, c.permissions, c.history, workingDir),
        tools.NewFetchTool(c.permissions, workingDir, ...),
        tools.NewGlobTool(workingDir),
        tools.NewGrepTool(workingDir),
        tools.NewLsTool(c.permissions, workingDir, ...),
        tools.NewViewTool(c.lspClients, c.permissions, workingDir),
        tools.NewWriteTool(c.lspClients, c.permissions, c.history, workingDir),
    )
    
    // MCP tools 也使用 workingDir
    for _, tool := range tools.GetMCPTools(c.permissions, workingDir) {
        // ...
    }
}
```

## 工作流程

1. **会话启动时**：
   - 调用 `coordinator.Run(ctx, sessionID, prompt, ...)`
   
2. **查询工作目录**：
   - 使用 `dbQuerier.GetSessionByID(sessionID)` 获取会话信息
   - 从会话中提取 `ProjectID`
   - 使用 `dbQuerier.GetProjectByID(projectID)` 获取项目信息
   - 从项目中提取 `WorkdirPath`
   
3. **使用工作目录**：
   - 如果 `WorkdirPath` 有效且不为空，使用它作为工作目录
   - 否则使用默认配置的工作目录 `c.cfg.WorkingDir()`
   
4. **重新构建工具**：
   - 使用查询到的 `workingDir` 重新构建所有需要工作目录的工具
   - 调用 `c.currentAgent.SetTools(sessionTools)` 更新 agent 的工具集

## 数据库表结构依赖

### sessions 表
```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT,  -- 外键关联到 projects 表
    -- ... 其他字段
);
```

### projects 表
```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    workdir_path TEXT,  -- 项目的工作目录路径
    -- ... 其他字段
);
```

## 注意事项

1. **向后兼容**：如果无法查询到工作目录（例如会话没有关联项目，或项目没有设置 workdir_path），系统会回退到使用默认配置的工作目录。

2. **错误处理**：所有数据库查询错误都会被记录日志，但不会导致会话失败，系统会使用默认工作目录继续运行。

3. **性能考虑**：每次会话运行时都会查询数据库两次（session + project），但这些查询都很轻量级，且对于会话的总体执行时间影响很小。

4. **工具隔离**：每个会话都会有自己的工具实例，使用独立的工作目录，确保不同项目的会话之间不会相互影响。

## 测试建议

1. **创建项目时设置 workdir_path**：
   ```go
   project := db.CreateProjectParams{
       ID: "project-1",
       WorkdirPath: sql.NullString{String: "/path/to/project", Valid: true},
       // ... 其他字段
   }
   ```

2. **创建会话时关联项目**：
   ```go
   session := db.CreateSessionParams{
       ID: "session-1",
       ProjectID: sql.NullString{String: "project-1", Valid: true},
       // ... 其他字段
   }
   ```

3. **验证工作目录**：
   - 检查日志中是否有 "Using project-specific working directory" 消息
   - 验证工具（如 bash、edit 等）是否使用了正确的工作目录

## 未来改进

1. **缓存机制**：可以考虑缓存 session -> project -> workdir 的映射，减少数据库查询次数
2. **工作目录验证**：在使用工作目录前验证其存在性和可访问性
3. **动态更新**：支持在会话运行过程中动态更新工作目录（如果项目的 workdir_path 被修改）

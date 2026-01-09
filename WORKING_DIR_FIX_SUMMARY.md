# Agent Working Directory 修复总结

## 问题描述

当前 agent 运行时的 working directory 使用的是主机运行时的目录,而不是基于会话 ID 查询到的项目工作目录(`project.workdir_path`)。

## 解决方案

通过 Context 传递项目特定的 working directory,让工具在执行时动态获取正确的工作目录。

## 主要修改

### 1. 添加 Context Key (`internal/agent/tools/tools.go`)

- 添加了 `WorkingDirContextKey` 常量
- 添加了 `GetWorkingDirFromContext(ctx)` 函数,用于从 context 获取工作目录

```go
const WorkingDirContextKey workingDirContextKey = "working_dir"

func GetWorkingDirFromContext(ctx context.Context) string {
	workingDir := ctx.Value(WorkingDirContextKey)
	if workingDir == nil {
		return ""
	}
	wd, ok := workingDir.(string)
	if !ok {
		return ""
	}
	return wd
}
```

### 2. Agent 层面查询并存入 Context (`internal/agent/agent.go`)

- 在 `sessionAgent` 结构体中添加了 `dbQuerier db.Querier` 字段
- 在 `SessionAgentOptions` 中添加了 `DBQuerier` 字段
- 在 `Run` 方法中,基于 session ID 查询关联的 project,获取 `workdir_path`,并存入 context:

```go
// Query and add working directory from project to the context
if a.dbQuerier != nil {
	dbSession, err := a.dbQuerier.GetSessionByID(ctx, call.SessionID)
	if err != nil {
		slog.Warn("Failed to get session for workdir lookup", "session_id", call.SessionID, "error", err)
	} else if dbSession.ProjectID.Valid && dbSession.ProjectID.String != "" {
		project, err := a.dbQuerier.GetProjectByID(ctx, dbSession.ProjectID.String)
		if err != nil {
			slog.Warn("Failed to get project for workdir lookup", "project_id", dbSession.ProjectID.String, "error", err)
		} else if project.WorkdirPath.Valid && project.WorkdirPath.String != "" {
			ctx = context.WithValue(ctx, tools.WorkingDirContextKey, project.WorkdirPath.String)
			slog.Info("Using project-specific working directory", "session_id", call.SessionID, "project_id", project.ID, "workdir", project.WorkdirPath.String)
		}
	}
}
```

### 3. Coordinator 传递 DBQuerier (`internal/agent/coordinator.go`)

- 在 `buildAgent` 方法中,将 `c.dbQuerier` 传递给 `NewSessionAgent`:

```go
result := NewSessionAgent(SessionAgentOptions{
	LargeModel:           large,
	SmallModel:           small,
	SystemPromptPrefix:   systemPromptPrefix,
	SystemPrompt:         systemPrompt,
	DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
	IsYolo:               c.permissions.SkipRequests(),
	Sessions:             c.sessions,
	Messages:             c.messages,
	Tools:                nil,
	DBQuerier:            c.dbQuerier, // 新增
})
```

### 4. 修改所有工具从 Context 读取 Working Directory

修改了以下工具,让它们优先从 context 读取 working directory:

- `bash.go`
- `edit.go`
- `multiedit.go`
- `write.go`
- `ls.go`
- `glob.go`
- `grep.go`
- `download.go`
- `fetch.go`
- `view.go`
- `web_fetch.go`

每个工具的修改模式:

```go
// 从 context 获取 working directory
contextWorkingDir := GetWorkingDirFromContext(ctx)
// 优先使用 context 中的值,如果没有则使用默认值
effectiveWorkingDir := cmp.Or(contextWorkingDir, workingDir)
// 在后续代码中使用 effectiveWorkingDir
```

### 5. 清理不必要的代码

删除了 `coordinator.go` 中尝试重新构建工具的代码,因为:
- 全局 agent 实例是共享的,为单个 session 设置工具会影响其他 session
- 现在工具通过 context 动态获取 working directory,不需要重新构建

## 优势

1. **线程安全**: 每个请求通过自己的 context 获取 working directory,不会相互影响
2. **灵活性**: 工具保持不变,只需在 context 中传递不同的 working directory
3. **向后兼容**: 如果 context 中没有 working directory,工具会使用默认值
4. **正确性**: 每个 session 都使用其关联项目的正确工作目录

## 测试

代码已通过完整编译验证:

```bash
cd /Users/apple/rolling-crush/crush-main
go build ./...
```

编译成功,没有错误。

## 额外修复

在测试过程中,还修复了一些与 session API 变更相关的编译错误:

1. `internal/tui/page/chat/chat.go`: 修复 `Sessions.Create` 调用,添加 `projectID` 参数
2. `internal/tui/tui.go`: 修复 `Sessions.List` 调用,添加 `projectID` 参数

## 建议

在使用前,建议:
1. 确保 `projects` 表的 `workdir_path` 字段已正确填充
2. 测试不同项目的 session,验证工具是否使用了正确的工作目录
3. 检查日志中的 "Using project-specific working directory" 消息,确认功能正常运行

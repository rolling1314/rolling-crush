# 权限确认功能修复总结

## 🔧 已完成的修复

### 1. 前端修复 (`crush-fe/src/App.tsx`)

#### 修复 1: 权限请求检测条件
**问题**: 原来的条件 `data.Type === 'permission_request' || data.tool_call_id` 太宽松，导致普通消息也被误判为权限请求。

**修复**:
```typescript
// 修复前
if (data.Type === 'permission_request' || data.tool_call_id) {

// 修复后  
if (data.Type === 'permission_request' && data.tool_call_id) {
```

#### 修复 2: 添加调试日志
添加了详细的控制台日志，方便调试：
- ✅ 权限请求接收日志
- 🔧 工具调用检测日志
- 📨 消息解析日志

### 2. 后端修复 (`crush-main/internal/app/app.go`)

#### 已确认正确配置:
- ✅ `Subscribe()` 方法正确广播权限请求
- ✅ `HandleClientMessage()` 正确处理权限响应
- ✅ 不自动批准 session（注释掉了 `AutoApproveSession`）

## 📋 权限确认流程

### 完整流程

```
1. 用户发送消息
   ↓
2. 后端 Agent 决定需要调用工具
   ↓
3. 后端发送工具调用消息（带 tool_call 的 Parts）
   前端解析并显示工具卡片
   ↓
4. 后端发送权限请求消息（Type: "permission_request"）
   前端将其添加到 pendingPermissions Map
   ↓
5. 前端检查 pendingPermissions.has(toolCall.id)
   如果存在，显示 Approve/Deny 按钮
   ↓
6. 用户点击 Approve 或 Deny
   ↓
7. 前端发送权限响应消息
   {
     type: "permission_response",
     tool_call_id: "xxx",
     granted: true/false
   }
   ↓
8. 后端收到响应，执行或拒绝工具调用
   ↓
9. 后端发送工具结果消息
   前端显示执行结果
   ↓
10. Agent 继续处理并返回最终答案
```

## 🎯 关键代码位置

### 前端

**权限请求处理** (`App.tsx:73-85`):
```typescript
if (data.Type === 'permission_request' && data.tool_call_id) {
  const permissionReq: PermissionRequest = {
    id: data.id || data.ID,
    session_id: data.session_id || data.SessionID,
    tool_call_id: data.tool_call_id,
    tool_name: data.tool_name,
    action: data.action
  };
  setPendingPermissions(prev => new Map(prev).set(permissionReq.tool_call_id, permissionReq));
  console.log('✅ Permission request received:', permissionReq);
  return;
}
```

**工具调用解析** (`App.tsx:104-114`):
```typescript
if (part.type === 'tool_call' || (part.id && part.name && part.input !== undefined)) {
  const toolCall: ToolCall = {
    id: part.id || part.data?.id,
    name: part.name || part.data?.name,
    input: part.input || part.data?.input || '',
    finished: part.finished ?? part.data?.finished ?? false,
    provider_executed: part.provider_executed ?? part.data?.provider_executed
  };
  toolCalls.push(toolCall);
  console.log('🔧 Tool call detected:', toolCall);
}
```

**权限按钮显示** (`ChatPanel.tsx:80-96`):
```typescript
{msg.toolCalls && msg.toolCalls.length > 0 && (
  <div className="space-y-2">
    {msg.toolCalls.map((toolCall) => {
      const result = msg.toolResults?.find(r => r.tool_call_id === toolCall.id);
      const needsPermission = pendingPermissions.has(toolCall.id);
      return (
        <ToolCallDisplay
          key={toolCall.id}
          toolCall={toolCall}
          result={result}
          needsPermission={needsPermission}
          onApprove={onPermissionApprove}
          onDeny={onPermissionDeny}
        />
      );
    })}
  </div>
)}
```

**按钮渲染** (`ToolCallDisplay.tsx:78-95`):
```typescript
{needsPermission && onApprove && onDeny && (
  <div className="flex gap-2 mb-2">
    <button onClick={() => onApprove(toolCall.id)} className="...">
      <CheckCircle className="w-3 h-3" />
      Approve
    </button>
    <button onClick={() => onDeny(toolCall.id)} className="...">
      <XCircle className="w-3 h-3" />
      Deny
    </button>
  </div>
)}
```

### 后端

**权限请求广播** (`app.go:461-473`):
```go
// Broadcast permission requests to WebSocket
if event, ok := msg.(pubsub.Event[permission.PermissionRequest]); ok {
    slog.Info("Broadcasting permission request to WebSocket", "tool_call_id", event.Payload.ToolCallID)
    app.WSServer.Broadcast(map[string]interface{}{
        "Type":        "permission_request",
        "id":          event.Payload.ID,
        "session_id":  event.Payload.SessionID,
        "tool_call_id": event.Payload.ToolCallID,
        "tool_name":   event.Payload.ToolName,
        "description": event.Payload.Description,
        "action":      event.Payload.Action,
        "params":      event.Payload.Params,
        "path":        event.Payload.Path,
    })
}
```

**权限响应处理** (`app.go:149-174`):
```go
if msg.Type == "permission_response" {
    permissionReq := permission.PermissionRequest{
        ID:         msg.ID,
        ToolCallID: msg.ToolCallID,
    }
    
    if msg.Granted {
        slog.Info("Permission granted by client", "tool_call_id", msg.ToolCallID)
        app.Permissions.Grant(permissionReq)
    } else if msg.Denied {
        slog.Info("Permission denied by client", "tool_call_id", msg.ToolCallID)
        app.Permissions.Deny(permissionReq)
    }
    return
}
```

## 🧪 测试方法

### 方法 1: 正常测试

1. 启动后端: `cd crush-main && go run main.go`
2. 启动前端: `cd crush-fe && pnpm run dev`
3. 打开浏览器: http://localhost:5173
4. 打开控制台: F12
5. 输入: "请读取 main.go 文件"
6. 观察: 工具调用卡片和权限按钮
7. 点击: Approve
8. 观察: 工具执行和结果

### 方法 2: 控制台调试

在浏览器控制台查看：
```javascript
// 查看所有消息
console.log('Messages:', messages);

// 查看待处理的权限请求
console.log('Pending permissions:', pendingPermissions);

// 查看 WebSocket 连接状态
console.log('WebSocket:', wsConnection);
```

## 🐛 故障排除

### 问题: 按钮不显示

**检查清单**:
1. [ ] 后端是否运行？
2. [ ] WebSocket 是否连接？（控制台应显示 "Connected to WebSocket"）
3. [ ] 是否收到权限请求？（控制台应显示 "✅ Permission request received"）
4. [ ] 是否收到工具调用？（控制台应显示 "🔧 Tool call detected"）
5. [ ] `tool_call_id` 是否匹配？

**调试命令**:
```javascript
// 检查最后一条消息
console.log('Last message:', messages[messages.length - 1]);

// 检查工具调用
console.log('Tool calls:', messages[messages.length - 1]?.toolCalls);

// 检查权限请求
console.log('Pending permissions:', Array.from(pendingPermissions.entries()));
```

### 问题: 点击按钮没反应

**检查清单**:
1. [ ] WebSocket 是否连接？
2. [ ] 是否有错误日志？
3. [ ] 权限响应是否发送？

**调试命令**:
```javascript
// 手动发送权限响应
if (wsConnection && wsConnection.readyState === WebSocket.OPEN) {
  wsConnection.send(JSON.stringify({
    type: 'permission_response',
    tool_call_id: 'xxx', // 替换为实际的 tool_call_id
    granted: true
  }));
}
```

### 问题: 后端没有发送权限请求

**检查**:
1. 后端日志中是否有 "Broadcasting permission request"
2. Session 是否被自动批准了？

**修复**:
确保 `app.go` 中的 `HandleClientMessage` 函数注释掉了：
```go
// app.Permissions.AutoApproveSession(sess.ID)
```

## 📚 相关文档

- `TOOL_CALL_FEATURE.md` - 工具调用功能完整文档
- `DEBUG_PERMISSIONS.md` - 详细调试指南
- `QUICK_TEST.md` - 快速测试指南
- `STREAMING_SETUP.md` - 流式消息渲染设置

## ✅ 验证成功的标志

测试成功时，你应该看到：

1. **控制台日志**:
   ```
   Connected to WebSocket
   WS Message: {...}
   🔧 Tool call detected: {id: "...", name: "read_file", ...}
   ✅ Permission request received: {tool_call_id: "...", ...}
   📨 Parsed message: {toolCallsCount: 1, ...}
   ```

2. **UI 显示**:
   - 工具调用卡片（橙色左边框）
   - 工具名称: "Read File"
   - 参数显示
   - **Approve** 按钮（绿色）
   - **Deny** 按钮（红色）
   - "Permission Required" 标签

3. **点击 Approve 后**:
   - 按钮消失
   - 状态变为 "Running..."
   - 完成后显示结果（绿色边框）

## 🎉 总结

所有必要的代码修改已完成：
- ✅ 前端权限请求检测逻辑修复
- ✅ 前端调试日志添加
- ✅ 后端权限广播正确配置
- ✅ 后端不自动批准权限
- ✅ UI 组件正确渲染

现在需要：
1. **启动后端和前端**
2. **打开浏览器控制台**
3. **发送测试消息**
4. **查看日志和 UI**

如果按钮还是不显示，请提供：
- 浏览器控制台的完整日志
- 后端终端的输出
- UI 截图


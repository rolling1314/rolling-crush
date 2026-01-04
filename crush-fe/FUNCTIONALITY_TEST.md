# 前端权限功能测试指南

## 当前实现状态

### ✅ 已实现的功能

1. **实时权限请求处理**
   - WebSocket接收 `permission_request` 消息
   - 添加到 `pendingPermissions` Map
   - 显示approve/deny按钮
   - 点击按钮发送 `permission_response`

2. **历史消息加载**
   - 使用统一的 `convertBackendMessageToFrontend` 函数
   - 解析 `text` - 文本内容
   - 解析 `thinking` - 推理过程（Thinking Process）
   - 解析 `tool_call` - 工具调用（id, name, input）
   - 解析 `tool_result` - 工具结果（content, is_error）

3. **UI渲染**
   - Thinking Process显示在紫色框中
   - 工具调用显示在卡片中
   - 工具结果可折叠显示
   - 权限按钮只在需要时显示

## 测试步骤

### 测试1: 实时权限按钮

1. 刷新浏览器 http://localhost:5173
2. 发送消息: "请帮我创建文件 /Users/apple/test.txt，内容是 hello"
3. 预期结果:
   - ✅ 显示工具调用卡片（橙色边框）
   - ✅ 显示 "Permission Required" 文本
   - ✅ 显示绿色 Approve 和红色 Deny 按钮
   - ✅ 点击按钮后按钮消失
   - ✅ 工具继续执行或停止

### 测试2: 历史消息中的Thinking Process

1. 切换到有历史消息的会话
2. 查看assistant消息
3. 预期结果:
   - ✅ 紫色 "💭 Thinking Process" 框显示
   - ✅ 显示thinking内容

### 测试3: 历史消息中的工具调用

1. 切换到有工具调用的会话
2. 查看工具调用消息
3. 预期结果:
   - ✅ 显示工具名称（如 "Write"）
   - ✅ Parameters 可以展开
   - ✅ Result 显示结果内容
   - ✅ 不显示 approve/deny 按钮（因为是历史）

## 控制台调试日志

打开浏览器控制台(F12)，可以看到：

```
Converting message: <msg-id> Parts: <count>
Processing part: [array of keys]
Found text, length: <n>
Found thinking, length: <n>
Found tool call: <id> <name>
Found tool result for: <id>
Converted message result: { hasText, hasReasoning, toolCallsCount, ... }
```

## 如果功能不正常

### Thinking Process 不显示
- 检查控制台: 是否显示 "Found thinking"
- 检查后端数据: Parts中是否有 `thinking` 字段

### 工具调用不显示
- 检查控制台: 是否显示 "Found tool call"
- 检查后端数据: Parts中是否有 `id`, `name`, `input` 字段

### Approve/Deny按钮不显示
- 检查控制台: 是否显示 "Permission request received"
- 检查 needsPermission 值
- 确认操作需要权限（如文件编辑）

## 代码位置

- 权限状态: `WorkspacePage.tsx` line 67
- WebSocket处理: `WorkspacePage.tsx` line 146-195
- 消息转换: `WorkspacePage.tsx` line 198-267
- 历史加载: `WorkspacePage.tsx` line 310-325
- 按钮渲染: `ToolCallDisplay.tsx` line 88-106
- Thinking显示: `ChatPanel.tsx` line 67-77


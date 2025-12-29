# 工具调用卡片优化

## ✨ 优化内容

### 1. 限制卡片宽度
- **最大宽度**: 95% 的父容器宽度
- 卡片不会再占据整个聊天框宽度
- 更紧凑、更美观的显示

### 2. Parameters 折叠功能
- 默认**折叠**，不显示参数内容
- 点击 "Parameters" 按钮展开/折叠
- 图标指示当前状态：
  - ▼ (ChevronDown) = 已折叠
  - ▲ (ChevronUp) = 已展开

### 3. Result 折叠功能
- 默认**展开**，显示执行结果
- 点击 "Result" 或 "Error" 按钮展开/折叠
- 支持独立控制（Parameters 和 Result 各自独立折叠）

### 4. 内容限制
- Parameters 最大高度: 200px（超出可滚动）
- Result 最大高度: 200px（超出可滚动）
- 文本自动换行（break-words）

## 🎯 使用效果

### 默认状态（折叠）
```
┌─────────────────────────────────────┐
│ 🔧 Bash                Permission  │
│ [Approve] [Deny]                    │
│ ▼ Parameters                        │
└─────────────────────────────────────┘
```

### 展开 Parameters
```
┌─────────────────────────────────────┐
│ 🔧 Bash                Permission  │
│ [Approve] [Deny]                    │
│ ▲ Parameters                        │
│ ┌─────────────────────────────────┐ │
│ │ {                               │ │
│ │   "command": "cd /Users/...",   │ │
│ │   "description": "..."          │ │
│ │ }                               │ │
│ └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

### 执行完成后（展开 Result）
```
┌─────────────────────────────────────┐
│ ✓ Bash                              │
│ ▼ Parameters                        │
│ ▲ Result                            │
│ ┌─────────────────────────────────┐ │
│ │ Directory created successfully  │ │
│ │ /Users/apple/code-oj-platform   │ │
│ └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

## 📝 代码修改详情

### 1. 添加状态管理

```typescript
const [isParamsExpanded, setIsParamsExpanded] = useState(false); // 默认折叠
const [isResultExpanded, setIsResultExpanded] = useState(true);  // 默认展开
```

### 2. 限制卡片宽度

```typescript
<div className={cn(
  "my-2 rounded-lg border-l-4 p-3 bg-gray-800/50 max-w-[95%]", // 添加 max-w-[95%]
  // ... 其他样式
)}>
```

### 3. Parameters 折叠按钮

```typescript
<button
  onClick={() => setIsParamsExpanded(!isParamsExpanded)}
  className="flex items-center gap-1 text-gray-400 hover:text-gray-300 mb-1 transition-colors"
>
  {isParamsExpanded ? (
    <ChevronUp className="w-3 h-3" />
  ) : (
    <ChevronDown className="w-3 h-3" />
  )}
  <span>Parameters</span>
</button>
{isParamsExpanded && (
  <div className="bg-gray-900/50 rounded p-2 overflow-x-auto max-h-[200px] overflow-y-auto">
    {/* 参数内容 */}
  </div>
)}
```

### 4. Result 折叠按钮

```typescript
<button
  onClick={() => setIsResultExpanded(!isResultExpanded)}
  className={cn(
    "flex items-center gap-1 mb-1 transition-colors",
    result.is_error 
      ? "text-red-400 hover:text-red-300" 
      : "text-green-400 hover:text-green-300"
  )}
>
  {isResultExpanded ? (
    <ChevronUp className="w-3 h-3" />
  ) : (
    <ChevronDown className="w-3 h-3" />
  )}
  <span>{result.is_error ? 'Error' : 'Result'}</span>
</button>
{isResultExpanded && (
  <div className="...">
    {/* 结果内容 */}
  </div>
)}
```

## 🎨 视觉优化

### 交互反馈
- ✅ 悬停时按钮文字颜色变亮
- ✅ 图标指示展开/折叠状态
- ✅ 颜色过渡动画（transition-colors）

### 空间利用
- ✅ 卡片宽度不超过 95%
- ✅ 默认折叠 Parameters，节省空间
- ✅ 长内容可滚动，不撑爆布局

### 颜色区分
- 🟢 **Result 成功**: 绿色按钮 + 深色背景
- 🔴 **Result 错误**: 红色按钮 + 红色背景
- ⚪ **Parameters**: 灰色按钮 + 深色背景

## 📊 对比效果

### 优化前
```
┌───────────────────────────────────────────────────────────┐
│ 🔧 Bash                           Permission Required     │
│ [Approve] [Deny]                                          │
│ Parameters:                                               │
│ ┌───────────────────────────────────────────────────────┐ │
│ │ {                                                     │ │
│ │   "command": "cd /Users/apple/code-oj-platform &&... │ │
│ │   "description": "Create OJ platform directory..."   │ │
│ │ }                                                     │ │
│ └───────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
```
**问题**:
- ❌ 卡片太宽，占据整个聊天框
- ❌ Parameters 始终展开，占用大量空间
- ❌ 长命令显示不友好

### 优化后
```
┌────────────────────────────────────┐ (宽度缩小至 95%)
│ 🔧 Bash         Permission Required│
│ [Approve] [Deny]                   │
│ ▼ Parameters (点击展开)             │
└────────────────────────────────────┘
```
**优势**:
- ✅ 卡片更紧凑，不占据整个宽度
- ✅ Parameters 默认折叠，界面更清爽
- ✅ 需要时再展开查看详情
- ✅ 独立控制 Parameters 和 Result

## 🚀 测试步骤

### 1. 启动应用

```bash
cd /Users/apple/crush/crush-fe
pnpm run dev
```

### 2. 测试折叠功能

1. 在聊天框输入需要工具调用的命令
2. 观察工具卡片显示
3. 点击 "▼ Parameters" 按钮
4. 观察参数展开
5. 再次点击 "▲ Parameters" 按钮
6. 观察参数折叠

### 3. 测试 Result 折叠

1. 等待工具执行完成
2. 观察 "▲ Result" 按钮（默认展开）
3. 点击按钮折叠结果
4. 再次点击展开结果

### 4. 测试卡片宽度

1. 观察卡片宽度是否比聊天框窄
2. 调整浏览器窗口大小
3. 确认卡片宽度响应式调整

## 💡 使用场景

### 场景 1: 快速浏览
- Parameters 折叠，只看工具名称
- 快速判断是否需要批准
- 不被长参数干扰

### 场景 2: 详细检查
- 展开 Parameters 查看具体参数
- 展开 Result 查看执行结果
- 做出准确的批准/拒绝决定

### 场景 3: 节省空间
- 多个工具调用时，折叠不必要的内容
- 保持界面整洁
- 更容易找到需要的信息

## 🎯 设计原则

### 1. 默认最小化
- Parameters 默认折叠（通常不需要立即看到）
- Result 默认展开（执行结果很重要）

### 2. 用户可控
- 完全由用户决定展开/折叠
- 每个卡片的状态独立
- 状态在卡片生命周期内保持

### 3. 清晰反馈
- 图标明确指示状态
- 悬停效果提示可点击
- 颜色区分不同类型内容

## 📏 尺寸规格

| 元素 | 尺寸 | 说明 |
|------|------|------|
| 卡片最大宽度 | 95% | 相对于父容器 |
| Parameters 最大高度 | 200px | 超出可滚动 |
| Result 最大高度 | 200px | 超出可滚动 |
| 折叠按钮图标 | 12px (w-3 h-3) | 小巧不突兀 |
| 字体大小 | text-xs | 紧凑显示 |

## ✅ 功能验收清单

- [x] 卡片宽度限制在 95%
- [x] Parameters 默认折叠
- [x] Parameters 可点击展开/折叠
- [x] Result 默认展开
- [x] Result 可点击展开/折叠
- [x] 图标正确指示展开/折叠状态
- [x] 悬停时有视觉反馈
- [x] 长内容可滚动
- [x] 文本自动换行
- [x] 样式保持一致性

## 🎉 总结

通过这次优化，工具调用卡片变得：
- ✅ **更紧凑** - 不占据整个聊天框宽度
- ✅ **更灵活** - 可以控制内容的展开/折叠
- ✅ **更清晰** - 默认只显示最重要的信息
- ✅ **更友好** - 需要时再展开查看详情

用户体验显著提升！🚀


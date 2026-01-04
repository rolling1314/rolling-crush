# TUI界面初次启动时模型配置的调用链路

本文档描述了TUI界面在初次启动时进行模型配置填写的完整调用链路。

## 调用链路概览

```
main.go:main()
  └─> cmd.Execute()
      └─> rootCmd.RunE() (如果启动TUI模式)
          └─> setupAppWithProgressBar()
              └─> setupApp()
                  └─> config.Init()
                      └─> config.Load()
                          └─> 加载配置，检查是否已配置
                  └─> app.New()
                      └─> 创建App实例
          └─> 启动TUI (tea.Program)
              └─> tui.New(app)
                  └─> chat.New(app) 创建ChatPage
                      └─> chatPage.Init()
                          └─> config.HasInitialDataConfig()
                              ├─> 如果返回false (未配置)
                              │   └─> splash.SetOnboarding(true)
                              │       └─> isOnboarding = true
                              │       └─> splashFullScreen = true
                              │
                              └─> 如果返回true (已配置)
                                  └─> 正常进入编辑器界面
```

## 详细调用流程

### 1. 应用启动入口

**文件**: `internal/cmd/root.go`

```go
rootCmd.RunE: func(cmd *cobra.Command, args []string) error {
    app, err := setupAppWithProgressBar(cmd)
    // ... 启动TUI或服务器
}
```

### 2. 配置初始化

**文件**: `internal/cmd/root.go` -> `setupApp()`

```go
cfg, err := config.Init(cwd, dataDir, debug)
```

**文件**: `internal/config/init.go`

```go
func Init(workingDir, dataDir string, debug bool) (*Config, error) {
    cfg, err := Load(workingDir, dataDir, debug)
    instance.Store(cfg)
    return instance.Load(), nil
}
```

### 3. 检查初始配置

**文件**: `internal/tui/page/chat/chat.go` -> `Init()`

```go
func (p *chatPage) Init() tea.Cmd {
    // ...
    
    // Set splash state based on config
    if !config.HasInitialDataConfig() {
        // First-time setup: show model selection
        p.splash.SetOnboarding(true)
        p.isOnboarding = true
        p.splashFullScreen = true
    } else if b, _ := config.ProjectNeedsInitialization(); b {
        // Project needs context initialization
        p.splash.SetProjectInit(true)
        p.isProjectInit = true
        p.splashFullScreen = true
    } else {
        // Ready to chat: focus editor, splash in background
        p.focusedPane = PanelTypeEditor
        p.splashFullScreen = false
    }
    
    return tea.Batch(
        p.header.Init(),
        p.sidebar.Init(),
        p.chat.Init(),
        p.editor.Init(),
        p.splash.Init(),  // 初始化splash组件
    )
}
```

**关键函数**: `config.HasInitialDataConfig()`

**文件**: `internal/config/init.go`

```go
func HasInitialDataConfig() bool {
    cfgPath := GlobalConfigData()  // 获取全局配置数据目录路径
    if _, err := os.Stat(cfgPath); err != nil {
        return false  // 配置文件不存在
    }
    return Get().IsConfigured()  // 检查是否至少有一个provider配置
}
```

### 4. Splash组件初始化

**文件**: `internal/tui/components/chat/splash/splash.go`

```go
func New() Splash {
    modelList := models.NewModelListComponent(listKeyMap, "Find your fave", false)
    apiKeyInput := models.NewAPIKeyInput()
    
    return &splashCmp{
        modelList:   modelList,
        apiKeyInput: apiKeyInput,
        // ...
    }
}

func (s *splashCmp) SetOnboarding(onboarding bool) {
    s.isOnboarding = onboarding
}

func (s *splashCmp) Init() tea.Cmd {
    return tea.Batch(
        s.modelList.Init(),      // 初始化模型列表组件
        s.apiKeyInput.Init(),    // 初始化API Key输入组件
        s.claudeAuthMethodChooser.Init(),
        s.claudeOAuth2.Init(),
    )
}
```

### 5. 模型列表组件初始化

**文件**: `internal/tui/components/dialogs/models/list.go`

```go
func (m *ModelListComponent) Init() tea.Cmd {
    var cmds []tea.Cmd
    if len(m.providers) == 0 {
        cfg := config.Get()
        providers, err := config.Providers(cfg)  // 获取所有可用的providers
        // 过滤出支持环境变量的providers
        filteredProviders := []catwalk.Provider{}
        for _, p := range providers {
            hasAPIKeyEnv := strings.HasPrefix(p.APIKey, "$")
            if hasAPIKeyEnv && p.ID != catwalk.InferenceProviderAzure {
                filteredProviders = append(filteredProviders, p)
            }
        }
        m.providers = filteredProviders
    }
    cmds = append(cmds, m.list.Init(), m.SetModelType(m.modelType))
    return tea.Batch(cmds...)
}
```

### 6. 用户交互流程

**文件**: `internal/tui/components/chat/splash/splash.go` -> `Update()`

用户选择模型时的处理流程：

```go
case key.Matches(msg, s.keyMap.Select):
    if s.isOnboarding && !s.needsAPIKey {
        selectedItem := s.modelList.SelectedModel()  // 获取选中的模型
        if selectedItem == nil {
            return s, nil
        }
        
        // 检查provider是否已配置
        if s.isProviderConfigured(string(selectedItem.Provider.ID)) {
            // Provider已配置，直接设置模型
            cmd := s.setPreferredModel(*selectedItem)
            s.isOnboarding = false
            return s, tea.Batch(cmd, util.CmdHandler(OnboardingCompleteMsg{}))
        } else {
            // Provider未配置，需要输入API Key
            if selectedItem.Provider.ID == catwalk.InferenceProviderAnthropic {
                // Claude需要特殊处理（OAuth或API Key）
                s.showClaudeAuthMethodChooser = true
                return s, nil
            }
            // 其他provider显示API Key输入框
            s.needsAPIKey = true
            s.selectedModel = selectedItem
            s.apiKeyInput.SetProviderName(selectedItem.Provider.Name)
            return s, nil
        }
    }
```

### 7. API Key输入和验证

**文件**: `internal/tui/components/chat/splash/splash.go`

```go
case key.Matches(msg, s.keyMap.Select):
    if s.needsAPIKey {
        // 用户输入API Key后按回车
        s.apiKeyValue = strings.TrimSpace(s.apiKeyInput.Value())
        if s.apiKeyValue == "" {
            return s, nil
        }
        
        // 创建provider配置并测试连接
        providerConfig := config.ProviderConfig{
            ID:      string(s.selectedModel.Provider.ID),
            Name:    s.selectedModel.Provider.Name,
            APIKey:  s.apiKeyValue,
            Type:    provider.Type,
            BaseURL: provider.APIEndpoint,
        }
        
        // 测试API Key有效性（异步）
        return s, tea.Sequence(
            util.CmdHandler(models.APIKeyStateChangeMsg{
                State: models.APIKeyInputStateVerifying,
            }),
            func() tea.Msg {
                err := providerConfig.TestConnection(config.Get().Resolver())
                if err == nil {
                    s.isAPIKeyValid = true
                    return models.APIKeyStateChangeMsg{
                        State: models.APIKeyInputStateVerified,
                    }
                }
                return models.APIKeyStateChangeMsg{
                    State: models.APIKeyInputStateError,
                }
            },
        )
    }
```

### 8. 保存API Key和模型配置

**文件**: `internal/tui/components/chat/splash/splash.go` -> `saveAPIKeyAndContinue()`

```go
func (s *splashCmp) saveAPIKeyAndContinue(apiKey any, close bool) tea.Cmd {
    cfg := config.Get()
    
    // 保存API Key到配置
    err := cfg.SetProviderAPIKey(string(s.selectedModel.Provider.ID), apiKey)
    if err != nil {
        return util.ReportError(fmt.Errorf("failed to save API key: %w", err))
    }
    
    // 设置首选模型
    cmd := s.setPreferredModel(*s.selectedModel)
    s.isOnboarding = false
    s.selectedModel = nil
    s.isAPIKeyValid = false
    
    if close {
        return tea.Batch(cmd, util.CmdHandler(OnboardingCompleteMsg{}))
    }
    return cmd
}
```

### 9. 设置首选模型

**文件**: `internal/tui/components/chat/splash/splash.go` -> `setPreferredModel()`

```go
func (s *splashCmp) setPreferredModel(selectedItem models.ModelOption) tea.Cmd {
    cfg := config.Get()
    model := cfg.GetModel(string(selectedItem.Provider.ID), selectedItem.Model.ID)
    
    // 创建SelectedModel配置
    selectedModel := config.SelectedModel{
        Model:           selectedItem.Model.ID,
        Provider:        string(selectedItem.Provider.ID),
        ReasoningEffort: model.DefaultReasoningEffort,
        MaxTokens:       model.DefaultMaxTokens,
    }
    
    // 更新Large模型配置
    err := cfg.UpdatePreferredModel(config.SelectedModelTypeLarge, selectedModel)
    if err != nil {
        return util.ReportError(err)
    }
    
    // 自动设置Small模型
    knownProvider, err := s.getProvider(selectedItem.Provider.ID)
    if knownProvider == nil {
        // 本地provider使用相同模型
        err = cfg.UpdatePreferredModel(config.SelectedModelTypeSmall, selectedModel)
    } else {
        // 使用provider的默认小模型
        smallModel := knownProvider.DefaultSmallModelID
        model := cfg.GetModel(string(selectedItem.Provider.ID), smallModel)
        smallSelectedModel := config.SelectedModel{
            Model:           smallModel,
            Provider:        string(selectedItem.Provider.ID),
            ReasoningEffort: model.DefaultReasoningEffort,
            MaxTokens:       model.DefaultMaxTokens,
        }
        err = cfg.UpdatePreferredModel(config.SelectedModelTypeSmall, smallSelectedModel)
    }
    
    cfg.SetupAgents()  // 重新设置Agent配置
    return nil
}
```

### 10. 配置保存

**文件**: `internal/config/config.go`

```go
func (c *Config) UpdatePreferredModel(modelType SelectedModelType, model SelectedModel) error {
    // 更新内存中的配置
    c.Models[modelType] = model
    
    // 保存到配置文件
    return c.saveConfig()
}

func (c *Config) SetProviderAPIKey(providerID string, apiKey any) error {
    // 更新provider的API Key
    if p, ok := c.Providers.Get(providerID); ok {
        p.APIKey = apiKey
        c.Providers.Set(providerID, p)
    } else {
        // 创建新的provider配置
        // ...
    }
    
    // 保存到配置文件
    return c.saveConfig()
}
```

## 关键文件和函数

### 核心文件

1. **`internal/tui/page/chat/chat.go`**
   - `chatPage.Init()` - 初始化聊天页面，检查配置状态

2. **`internal/tui/components/chat/splash/splash.go`**
   - `SetOnboarding()` - 设置onboarding状态
   - `Update()` - 处理用户交互
   - `setPreferredModel()` - 设置首选模型
   - `saveAPIKeyAndContinue()` - 保存API Key并继续

3. **`internal/tui/components/dialogs/models/list.go`**
   - `NewModelListComponent()` - 创建模型列表组件
   - `Init()` - 初始化模型列表

4. **`internal/config/init.go`**
   - `HasInitialDataConfig()` - 检查是否有初始配置
   - `IsConfigured()` - 检查是否已配置provider

5. **`internal/config/config.go`**
   - `UpdatePreferredModel()` - 更新首选模型
   - `SetProviderAPIKey()` - 设置provider的API Key

### 配置检查逻辑

```go
HasInitialDataConfig() 
  └─> 检查 GlobalConfigData() 路径下的配置文件是否存在
  └─> 如果存在，调用 Get().IsConfigured()
      └─> IsConfigured() 检查是否有至少一个enabled的provider
```

### 状态流转

```
未配置状态
  └─> HasInitialDataConfig() 返回 false
      └─> SetOnboarding(true)
          └─> 显示模型选择界面
              └─> 用户选择模型
                  ├─> Provider已配置
                  │   └─> setPreferredModel()
                  │       └─> UpdatePreferredModel()
                  │           └─> 完成配置，发送OnboardingCompleteMsg
                  │
                  └─> Provider未配置
                      └─> 显示API Key输入
                          └─> 用户输入API Key
                              └─> TestConnection() 验证
                                  └─> saveAPIKeyAndContinue()
                                      └─> SetProviderAPIKey()
                                      └─> setPreferredModel()
                                          └─> 完成配置，发送OnboardingCompleteMsg
```

## 总结

TUI界面在初次启动时的模型配置流程：

1. **启动检查**: 通过 `HasInitialDataConfig()` 检查是否已有配置
2. **显示选择界面**: 如果未配置，显示模型选择列表
3. **Provider检查**: 检查选中的模型所属的provider是否已配置
4. **API Key输入**: 如果provider未配置，引导用户输入API Key
5. **验证保存**: 验证API Key有效性，保存配置
6. **模型设置**: 设置Large和Small模型的首选配置
7. **完成配置**: 发送 `OnboardingCompleteMsg`，进入正常使用状态

整个流程都在 `splash` 组件中处理，通过状态机模式管理不同的交互阶段。

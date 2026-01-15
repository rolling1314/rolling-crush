package agent

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	agentprompt "github.com/rolling1314/rolling-crush/internal/agent/prompt"
	"github.com/rolling1314/rolling-crush/internal/agent/tools"
	"github.com/rolling1314/rolling-crush/pkg/config"
	"github.com/rolling1314/rolling-crush/internal/pkg/csync"
	"github.com/rolling1314/rolling-crush/infra/postgres"
	"github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/domain/history"
	"github.com/rolling1314/rolling-crush/domain/toolcall"
	"github.com/rolling1314/rolling-crush/internal/pkg/log"
	"github.com/rolling1314/rolling-crush/internal/lsp"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/domain/session"
	"golang.org/x/sync/errgroup"

	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/azure"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/fantasy/providers/openrouter"
	openaisdk "github.com/openai/openai-go/v2/option"
	"github.com/qjebbs/go-jsons"
)

type Coordinator interface {
	// INFO: (kujtim) this is not used yet we will use this when we have multiple agents
	// SetMainAgent(string)
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string) error
	Model() Model
	UpdateModels(ctx context.Context) error
}

type coordinator struct {
	cfg         *config.Config
	sessions    session.Service
	messages    message.Service
	toolCalls   toolcall.Service
	redisCmd    *redis.CommandService
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
	dbReader    config.DBReader // For loading session-specific config from DB
	dbQuerier   postgres.Querier      // For querying session and project info

	currentAgent SessionAgent
	agents       map[string]SessionAgent

	readyWg errgroup.Group
}

func NewCoordinator(
	ctx context.Context,
	cfg *config.Config,
	sessions session.Service,
	messages message.Service,
	toolCalls toolcall.Service,
	redisCmd *redis.CommandService,
	permissions permission.Service,
	history history.Service,
	lspClients *csync.Map[string, *lsp.Client],
	dbReader config.DBReader, // Add dbReader parameter
) (Coordinator, error) {
	// dbReader also implements postgres.Querier (it's the same postgres.Queries instance)
	var dbQuerier postgres.Querier
	if dbReader != nil {
		if q, ok := dbReader.(postgres.Querier); ok {
			dbQuerier = q
		}
	}
	
	c := &coordinator{
		cfg:         cfg,
		sessions:    sessions,
		messages:    messages,
		toolCalls:   toolCalls,
		redisCmd:    redisCmd,
		permissions: permissions,
		history:     history,
		lspClients:  lspClients,
		dbReader:    dbReader,
		dbQuerier:   dbQuerier,
		agents:      make(map[string]SessionAgent),
	}

	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil, errors.New("coder agent not configured")
	}

	// TODO: make this dynamic when we support multiple agents
	coderAgentPrompt, err := coderPrompt(agentprompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(ctx, coderAgentPrompt, agentCfg)
	if err != nil {
		return nil, err
	}
	c.currentAgent = agent
	c.agents[config.AgentCoder] = agent
	return c, nil
}

// Run implements Coordinator.
func (c *coordinator) Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	fmt.Println("\n=== Coordinator.Run 方法调用 ===")
	fmt.Printf("SessionID: %s\n", sessionID)
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Printf("接收到的附件数量: %d\n", len(attachments))
	for i, att := range attachments {
		fmt.Printf("  [附件 %d] FileName: %s, MimeType: %s, Size: %d bytes\n", 
			i+1, att.FileName, att.MimeType, len(att.Content))
	}
	fmt.Println("=== Coordinator.Run 开始处理 ===\n")

	if err := c.readyWg.Wait(); err != nil {
		fmt.Println("readyWg.Wait failed:", err)
		return nil, err
	}
	fmt.Println("readyWg.Wait passed")

	// Check if currentAgent exists
	if c.currentAgent == nil {
		fmt.Println("ERROR: currentAgent is nil!")
		return nil, errors.New("agent not initialized")
	}
	fmt.Println("currentAgent exists")

	// Query workdir_path from session -> project for prompt
	workingDirForPrompt := c.cfg.WorkingDir() // Default to config working dir
	if c.dbQuerier != nil {
		dbSession, err := c.dbQuerier.GetSessionByID(ctx, sessionID)
		if err != nil {
			slog.Warn("Failed to get session for workdir lookup", "session_id", sessionID, "error", err)
		} else if dbSession.ProjectID.Valid && dbSession.ProjectID.String != "" {
			project, err := c.dbQuerier.GetProjectByID(ctx, dbSession.ProjectID.String)
			if err != nil {
				slog.Warn("Failed to get project for workdir lookup", "project_id", dbSession.ProjectID.String, "error", err)
			} else if project.WorkdirPath.Valid && project.WorkdirPath.String != "" {
				workingDirForPrompt = project.WorkdirPath.String
				slog.Info("Using project-specific working directory for prompt", "session_id", sessionID, "project_id", project.ID, "workdir", workingDirForPrompt)
			}
		}
	}

	// Load session-specific config from database if dbReader is available
	sessionCfg := c.cfg
	if c.dbReader != nil {
		fmt.Println("dbReader available, loading session config")
		var err error
		sessionCfg, err = config.LoadWithSessionConfig(
			ctx,
			c.cfg.WorkingDir(),
			c.cfg.Options.DataDirectory,
			c.cfg.Options.Debug,
			sessionID,
			c.dbReader,
		)
		if err != nil {
			fmt.Println("Failed to load session config:", err)
			slog.Error("Failed to load session config, using base config", "session_id", sessionID, "error", err)
			sessionCfg = c.cfg // Fallback to base config
		} else {
			fmt.Println("Session config loaded successfully")
		}
	} else {
		fmt.Println("dbReader is nil, using base config")
	}

	fmt.Println("About to build agent models with session config")
	// Build agent models using session config
	large, small, err := c.buildAgentModelsWithConfig(ctx, sessionCfg)
	fmt.Println(sessionCfg)
	fmt.Println("hello")
	if err != nil {
		fmt.Println("buildAgentModelsWithConfig failed:", err)
		// Fallback to current agent's models
		slog.Error("Failed to build session models, using default", "session_id", sessionID, "error", err)
		large = c.currentAgent.Model()
		// Try to build small model from base config
		small, _, _ = c.buildAgentModelsWithConfig(ctx, c.cfg)
	} else {
		fmt.Println("Models built successfully, updating agent")
		// Update current agent's models for this session
		c.currentAgent.SetModels(large, small)
	}

	// Rebuild system prompt with project-specific working directory
	sessionPrompt, err := coderPrompt(agentprompt.WithWorkingDir(workingDirForPrompt))
	if err != nil {
		slog.Error("Failed to build session-specific prompt", "error", err)
	} else {
		sessionSystemPrompt, err := sessionPrompt.Build(ctx, large.Model.Provider(), large.Model.Model(), *sessionCfg)
		if err != nil {
			slog.Error("Failed to build session system prompt", "error", err)
		} else {
			// Update agent's system prompt for this session
			c.currentAgent.(*sessionAgent).systemPrompt = sessionSystemPrompt
			fmt.Println("Updated system prompt with workdir:", workingDirForPrompt)
		}
	}

	model := large
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	fmt.Printf("\n=== Coordinator: 检查模型图片支持 ===\n")
	fmt.Printf("模型: %s\n", model.Model.Model())
	fmt.Printf("支持图片: %v\n", model.CatwalkCfg.SupportsImages)
	fmt.Printf("接收到的附件数量: %d\n", len(attachments))
	
	if !model.CatwalkCfg.SupportsImages && attachments != nil {
		fmt.Printf("⚠️  警告：模型不支持图片，移除 %d 个附件！\n", len(attachments))
		attachments = nil
	} else if len(attachments) > 0 {
		fmt.Printf("✅ 模型支持图片，保留 %d 个附件\n", len(attachments))
	}
	fmt.Printf("=== Coordinator: 检查完成 ===\n\n")

	providerCfg, ok := sessionCfg.Providers.Get(model.ModelCfg.Provider)
	if !ok {
		return nil, errors.New("model provider not configured")
	}

	mergedOptions, temp, topP, topK, freqPenalty, presPenalty := mergeCallOptions(model, providerCfg)

	fmt.Printf("\n=== Coordinator: 调用 currentAgent.Run ===\n")
	fmt.Printf("最终传递给 Agent 的附件数量: %d\n", len(attachments))
	for i, att := range attachments {
		fmt.Printf("  [附件 %d] FileName: %s, MimeType: %s, Size: %d bytes\n", 
			i+1, att.FileName, att.MimeType, len(att.Content))
	}
	fmt.Println("=== Coordinator: 开始调用 Agent ===\n")
	
	return c.currentAgent.Run(ctx, SessionAgentCall{
		SessionID:        sessionID,
		Prompt:           prompt,
		Attachments:      attachments,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  mergedOptions,
		Temperature:      temp,
		TopP:             topP,
		TopK:             topK,
		FrequencyPenalty: freqPenalty,
		PresencePenalty:  presPenalty,
	})
}

func getProviderOptions(model Model, providerCfg config.ProviderConfig) fantasy.ProviderOptions {
	options := fantasy.ProviderOptions{}

	cfgOpts := []byte("{}")
	providerCfgOpts := []byte("{}")
	catwalkOpts := []byte("{}")

	if model.ModelCfg.ProviderOptions != nil {
		data, err := json.Marshal(model.ModelCfg.ProviderOptions)
		if err == nil {
			cfgOpts = data
		}
	}

	if providerCfg.ProviderOptions != nil {
		data, err := json.Marshal(providerCfg.ProviderOptions)
		if err == nil {
			providerCfgOpts = data
		}
	}

	if model.CatwalkCfg.Options.ProviderOptions != nil {
		data, err := json.Marshal(model.CatwalkCfg.Options.ProviderOptions)
		if err == nil {
			catwalkOpts = data
		}
	}

	readers := []io.Reader{
		bytes.NewReader(catwalkOpts),
		bytes.NewReader(providerCfgOpts),
		bytes.NewReader(cfgOpts),
	}

	got, err := jsons.Merge(readers)
	if err != nil {
		slog.Error("Could not merge call config", "err", err)
		return options
	}

	mergedOptions := make(map[string]any)

	err = json.Unmarshal([]byte(got), &mergedOptions)
	if err != nil {
		slog.Error("Could not create config for call", "err", err)
		return options
	}

	switch providerCfg.Type {
	case openai.Name, azure.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.ModelCfg.ReasoningEffort
		}
		if openai.IsResponsesModel(model.CatwalkCfg.ID) {
			if openai.IsResponsesReasoningModel(model.CatwalkCfg.ID) {
				mergedOptions["reasoning_summary"] = "auto"
				mergedOptions["include"] = []openai.IncludeType{openai.IncludeReasoningEncryptedContent}
			}
			parsed, err := openai.ParseResponsesOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		} else {
			parsed, err := openai.ParseOptions(mergedOptions)
			if err == nil {
				options[openai.Name] = parsed
			}
		}
	case anthropic.Name:
		_, hasThink := mergedOptions["thinking"]
		if !hasThink && model.ModelCfg.Think {
			mergedOptions["thinking"] = map[string]any{
				// TODO: kujtim see if we need to make this dynamic
				"budget_tokens": 2000,
			}
		}
		parsed, err := anthropic.ParseOptions(mergedOptions)
		if err == nil {
			options[anthropic.Name] = parsed
		}

	case openrouter.Name:
		_, hasReasoning := mergedOptions["reasoning"]
		if !hasReasoning && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning"] = map[string]any{
				"enabled": true,
				"effort":  model.ModelCfg.ReasoningEffort,
			}
		}
		parsed, err := openrouter.ParseOptions(mergedOptions)
		if err == nil {
			options[openrouter.Name] = parsed
		}
	case google.Name:
		_, hasReasoning := mergedOptions["thinking_config"]
		if !hasReasoning {
			mergedOptions["thinking_config"] = map[string]any{
				"thinking_budget":  2000,
				"include_thoughts": true,
			}
		}
		parsed, err := google.ParseOptions(mergedOptions)
		if err == nil {
			options[google.Name] = parsed
		}
	case openaicompat.Name:
		_, hasReasoningEffort := mergedOptions["reasoning_effort"]
		if !hasReasoningEffort && model.ModelCfg.ReasoningEffort != "" {
			mergedOptions["reasoning_effort"] = model.ModelCfg.ReasoningEffort
		}
		parsed, err := openaicompat.ParseOptions(mergedOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}
	}

	return options
}

func mergeCallOptions(model Model, cfg config.ProviderConfig) (fantasy.ProviderOptions, *float64, *float64, *int64, *float64, *float64) {
	modelOptions := getProviderOptions(model, cfg)
	temp := cmp.Or(model.ModelCfg.Temperature, model.CatwalkCfg.Options.Temperature)
	topP := cmp.Or(model.ModelCfg.TopP, model.CatwalkCfg.Options.TopP)
	topK := cmp.Or(model.ModelCfg.TopK, model.CatwalkCfg.Options.TopK)
	freqPenalty := cmp.Or(model.ModelCfg.FrequencyPenalty, model.CatwalkCfg.Options.FrequencyPenalty)
	presPenalty := cmp.Or(model.ModelCfg.PresencePenalty, model.CatwalkCfg.Options.PresencePenalty)
	return modelOptions, temp, topP, topK, freqPenalty, presPenalty
}

func (c *coordinator) buildAgent(ctx context.Context, agentPrompt *agentprompt.Prompt, agent config.Agent) (SessionAgent, error) {
	large, small, err := c.buildAgentModels(ctx)

	// Build system prompt - use a default provider if models aren't configured yet
	var systemPrompt string
	var systemPromptPrefix string
	if err != nil {
		// In Web mode, models may not be configured yet (loaded per-session)
		slog.Warn("Failed to build initial agent models, will load from session config", "error", err)
		// Use default anthropic provider for prompt building
		systemPrompt, err = agentPrompt.Build(ctx, "anthropic", "claude-sonnet-4-5-20250929", *c.cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build system prompt: %w", err)
		}
		systemPromptPrefix = "" // Will be set when session model is loaded
	} else {
		systemPrompt, err = agentPrompt.Build(ctx, large.Model.Provider(), large.Model.Model(), *c.cfg)
		if err != nil {
			return nil, err
		}
		largeProviderCfg, _ := c.cfg.Providers.Get(large.ModelCfg.Provider)
		systemPromptPrefix = largeProviderCfg.SystemPromptPrefix
	}

	// Create agent with system prompt (models may be empty initially)
	result := NewSessionAgent(SessionAgentOptions{
		LargeModel:           large,
		SmallModel:           small,
		SystemPromptPrefix:   systemPromptPrefix,
		SystemPrompt:         systemPrompt,
		DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
		IsYolo:               c.permissions.SkipRequests(),
		Sessions:             c.sessions,
		Messages:             c.messages,
		ToolCalls:            c.toolCalls,
		RedisCmd:             c.redisCmd,
		Tools:                nil,
		DBQuerier:            c.dbQuerier,
	})

	// Build tools asynchronously (tools don't depend on models)
	c.readyWg.Go(func() error {
		tools, err := c.buildTools(ctx, agent, c.cfg.WorkingDir())
		if err != nil {
			return err
		}
		result.SetTools(tools)
		slog.Info("Agent tools initialized", "tool_count", len(tools))
		return nil
	})

	return result, nil
}

func (c *coordinator) buildTools(ctx context.Context, agent config.Agent, workingDir string) ([]fantasy.AgentTool, error) {
	var allTools []fantasy.AgentTool
	if slices.Contains(agent.AllowedTools, AgentToolName) {
		agentTool, err := c.agentTool(ctx)
		if err != nil {
			// Agent tool is optional - if task agent is not configured, skip it
			slog.Warn("Skipping agent tool - task agent not configured", "error", err)
		} else {
			allTools = append(allTools, agentTool)
		}
	}

	if slices.Contains(agent.AllowedTools, tools.AgenticFetchToolName) {
		agenticFetchTool, err := c.agenticFetchTool(ctx, nil)
		if err != nil {
			// Agentic fetch is optional
			slog.Warn("Skipping agentic fetch tool", "error", err)
		} else {
			allTools = append(allTools, agenticFetchTool)
		}
	}

	// Get the model name for the agent
	modelName := ""
	if modelCfg, ok := c.cfg.Models[agent.Model]; ok {
		if model := c.cfg.GetModel(modelCfg.Provider, modelCfg.Model); model != nil {
			modelName = model.Name
		}
	}

	allTools = append(allTools,
		tools.NewBashTool(c.permissions, workingDir, c.cfg.Options.Attribution, modelName),
		tools.NewJobOutputTool(),
		tools.NewJobKillTool(),
		tools.NewDownloadTool(c.permissions, workingDir, nil),
		tools.NewEditTool(c.lspClients, c.permissions, c.history, workingDir),
		tools.NewMultiEditTool(c.lspClients, c.permissions, c.history, workingDir),
		tools.NewFetchTool(c.permissions, workingDir, nil),
		tools.NewGlobTool(workingDir),
		tools.NewGrepTool(workingDir),
		tools.NewLsTool(c.permissions, workingDir, c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewViewTool(c.lspClients, c.permissions, workingDir),
		tools.NewWriteTool(c.lspClients, c.permissions, c.history, workingDir),
	)

	if len(c.cfg.LSP) > 0 {
		allTools = append(allTools, tools.NewDiagnosticsTool(c.lspClients), tools.NewReferencesTool(c.lspClients))
	}

	var filteredTools []fantasy.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	for _, tool := range tools.GetMCPTools(c.permissions, workingDir) {
		if agent.AllowedMCP == nil {
			// No MCP restrictions
			filteredTools = append(filteredTools, tool)
			continue
		}
		if len(agent.AllowedMCP) == 0 {
			// No MCPs allowed
			slog.Debug("no MCPs allowed", "tool", tool.Name(), "agent", agent.Name)
			break
		}

		for mcp, tools := range agent.AllowedMCP {
			if mcp != tool.MCP() {
				continue
			}
			if len(tools) == 0 || slices.Contains(tools, tool.MCPToolName()) {
				filteredTools = append(filteredTools, tool)
			}
		}
		slog.Debug("MCP not allowed", "tool", tool.Name(), "agent", agent.Name)
	}
	slices.SortFunc(filteredTools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return filteredTools, nil
}

// TODO: when we support multiple agents we need to change this so that we pass in the agent specific model config
func (c *coordinator) buildAgentModels(ctx context.Context) (Model, Model, error) {
	return c.buildAgentModelsWithConfig(ctx, c.cfg)
}

// buildAgentModelsWithConfig builds agent models using a specific config (for session-specific configs)
func (c *coordinator) buildAgentModelsWithConfig(ctx context.Context, cfg *config.Config) (Model, Model, error) {
	largeModelCfg, ok := cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return Model{}, Model{}, errors.New("large model not selected")
	}
	smallModelCfg, ok := cfg.Models[config.SelectedModelTypeSmall]
	if !ok {
		return Model{}, Model{}, errors.New("small model not selected")
	}

	largeProviderCfg, ok := cfg.Providers.Get(largeModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("large model provider not configured")
	}

	largeProvider, err := c.buildProviderWithConfig(largeProviderCfg, largeModelCfg, cfg)
	if err != nil {
		return Model{}, Model{}, err
	}

	smallProviderCfg, ok := cfg.Providers.Get(smallModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("small model provider not configured")
	}

	smallProvider, err := c.buildProviderWithConfig(smallProviderCfg, smallModelCfg, cfg)
	if err != nil {
		return Model{}, Model{}, err
	}

	var largeCatwalkModel *catwalk.Model
	var smallCatwalkModel *catwalk.Model

	for _, m := range largeProviderCfg.Models {
		if m.ID == largeModelCfg.Model {
			largeCatwalkModel = &m
		}
	}
	for _, m := range smallProviderCfg.Models {
		if m.ID == smallModelCfg.Model {
			smallCatwalkModel = &m
		}
	}

	if largeCatwalkModel == nil {
		return Model{}, Model{}, errors.New("large model not found in provider config")
	}

	if smallCatwalkModel == nil {
		return Model{}, Model{}, errors.New("small model not found in provider config")
	}

	largeModelID := largeModelCfg.Model
	smallModelID := smallModelCfg.Model

	if largeModelCfg.Provider == openrouter.Name && isExactoSupported(largeModelID) {
		largeModelID += ":exacto"
	}

	if smallModelCfg.Provider == openrouter.Name && isExactoSupported(smallModelID) {
		smallModelID += ":exacto"
	}

	largeModel, err := largeProvider.LanguageModel(ctx, largeModelID)
	if err != nil {
		return Model{}, Model{}, err
	}
	smallModel, err := smallProvider.LanguageModel(ctx, smallModelID)
	if err != nil {
		return Model{}, Model{}, err
	}

	return Model{
			Model:      largeModel,
			CatwalkCfg: *largeCatwalkModel,
			ModelCfg:   largeModelCfg,
		}, Model{
			Model:      smallModel,
			CatwalkCfg: *smallCatwalkModel,
			ModelCfg:   smallModelCfg,
		}, nil
}

func (c *coordinator) buildAnthropicProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	var opts []anthropic.Option

	if strings.HasPrefix(apiKey, "Bearer ") {
		// NOTE: Prevent the SDK from picking up the API key from env.
		os.Setenv("ANTHROPIC_API_KEY", "")

		headers["Authorization"] = apiKey
	} else if apiKey != "" {
		// X-Api-Key header
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}

	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}

	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}

	return anthropic.New(opts...)
}

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
		openai.WithUseResponsesAPI(),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func (c *coordinator) buildOpenrouterProvider(_, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openrouter.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func (c *coordinator) buildOpenaiCompatProvider(baseURL, apiKey string, headers map[string]string, extraBody map[string]any) (fantasy.Provider, error) {
	opts := []openaicompat.Option{
		openaicompat.WithBaseURL(baseURL),
		openaicompat.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}

	for extraKey, extraValue := range extraBody {
		opts = append(opts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(extraKey, extraValue)))
	}

	return openaicompat.New(opts...)
}

func (c *coordinator) buildAzureProvider(baseURL, apiKey string, headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []azure.Option{
		azure.WithBaseURL(baseURL),
		azure.WithAPIKey(apiKey),
		azure.WithUseResponsesAPI(),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, azure.WithHTTPClient(httpClient))
	}
	if options == nil {
		options = make(map[string]string)
	}
	if apiVersion, ok := options["apiVersion"]; ok {
		opts = append(opts, azure.WithAPIVersion(apiVersion))
	}
	if len(headers) > 0 {
		opts = append(opts, azure.WithHeaders(headers))
	}

	return azure.New(opts...)
}

func (c *coordinator) buildBedrockProvider(headers map[string]string) (fantasy.Provider, error) {
	var opts []bedrock.Option
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, bedrock.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, bedrock.WithHeaders(headers))
	}
	bearerToken := os.Getenv("AWS_BEARER_TOKEN_BEDROCK")
	if bearerToken != "" {
		opts = append(opts, bedrock.WithAPIKey(bearerToken))
	}
	return bedrock.New(opts...)
}

func (c *coordinator) buildGoogleProvider(baseURL, apiKey string, headers map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{
		google.WithBaseURL(baseURL),
		google.WithGeminiAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func (c *coordinator) buildGoogleVertexProvider(headers map[string]string, options map[string]string) (fantasy.Provider, error) {
	opts := []google.Option{}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}

	project := options["project"]
	location := options["location"]

	opts = append(opts, google.WithVertex(project, location))

	return google.New(opts...)
}

func (c *coordinator) isAnthropicThinking(model config.SelectedModel) bool {
	if model.Think {
		return true
	}

	if model.ProviderOptions == nil {
		return false
	}

	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	if err != nil {
		return false
	}
	if opts.Thinking != nil {
		return true
	}
	return false
}

func (c *coordinator) buildProvider(providerCfg config.ProviderConfig, model config.SelectedModel) (fantasy.Provider, error) {
	return c.buildProviderWithConfig(providerCfg, model, c.cfg)
}

func (c *coordinator) buildProviderWithConfig(providerCfg config.ProviderConfig, model config.SelectedModel, cfg *config.Config) (fantasy.Provider, error) {
	headers := maps.Clone(providerCfg.ExtraHeaders)
	if headers == nil {
		headers = make(map[string]string)
	}

	// handle special headers for anthropic
	if providerCfg.Type == anthropic.Name && c.isAnthropicThinking(model) {
		if v, ok := headers["anthropic-beta"]; ok {
			headers["anthropic-beta"] = v + ",interleaved-thinking-2025-05-14"
		} else {
			headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
		}
	}

	apiKey, _ := cfg.Resolve(providerCfg.APIKey)
	baseURL, _ := cfg.Resolve(providerCfg.BaseURL)

	switch providerCfg.Type {
	case openai.Name:
		return c.buildOpenaiProvider(baseURL, apiKey, headers)
	case anthropic.Name:
		return c.buildAnthropicProvider(baseURL, apiKey, headers)
	case openrouter.Name:
		return c.buildOpenrouterProvider(baseURL, apiKey, headers)
	case azure.Name:
		return c.buildAzureProvider(baseURL, apiKey, headers, providerCfg.ExtraParams)
	case bedrock.Name:
		return c.buildBedrockProvider(headers)
	case google.Name:
		return c.buildGoogleProvider(baseURL, apiKey, headers)
	case "google-vertex":
		return c.buildGoogleVertexProvider(headers, providerCfg.ExtraParams)
	case openaicompat.Name:
		return c.buildOpenaiCompatProvider(baseURL, apiKey, headers, providerCfg.ExtraBody)
	default:
		return nil, fmt.Errorf("provider type not supported: %q", providerCfg.Type)
	}
}

func isExactoSupported(modelID string) bool {
	supportedModels := []string{
		"moonshotai/kimi-k2-0905",
		"deepseek/deepseek-v3.1-terminus",
		"z-ai/glm-4.6",
		"openai/gpt-oss-120b",
		"qwen/qwen3-coder",
	}
	return slices.Contains(supportedModels, modelID)
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent.Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent.CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent.ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent.IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent.IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent.Model()
}

func (c *coordinator) UpdateModels(ctx context.Context) error {
	// build the models again so we make sure we get the latest config
	large, small, err := c.buildAgentModels(ctx)
	if err != nil {
		return err
	}
	c.currentAgent.SetModels(large, small)

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return errors.New("coder agent not configured")
	}

	tools, err := c.buildTools(ctx, agentCfg, c.cfg.WorkingDir())
	if err != nil {
		return err
	}
	c.currentAgent.SetTools(tools)
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent.QueuedPrompts(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	providerCfg, ok := c.cfg.Providers.Get(c.currentAgent.Model().ModelCfg.Provider)
	if !ok {
		return errors.New("model provider not configured")
	}
	return c.currentAgent.Summarize(ctx, sessionID, getProviderOptions(c.currentAgent.Model(), providerCfg))
}

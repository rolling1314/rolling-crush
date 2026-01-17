// Package agent is the core orchestration layer for Crush AI agents.
//
// It provides session-based AI agent functionality for managing
// conversations, tool execution, and message handling. It coordinates
// interactions between language models, messages, sessions, and tools while
// handling features like automatic summarization, queuing, and token
// management.
package agent

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/toolcall"
	"github.com/rolling1314/rolling-crush/infra/postgres"
	"github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/infra/storage"
	"github.com/rolling1314/rolling-crush/internal/agent/tools"
	"github.com/rolling1314/rolling-crush/internal/pkg/csync"
	"github.com/rolling1314/rolling-crush/internal/pkg/stringext"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

type SessionAgentCall struct {
	SessionID        string
	Prompt           string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string, fantasy.ProviderOptions) error
	Model() Model
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
}

type sessionAgent struct {
	largeModel           Model
	smallModel           Model
	systemPromptPrefix   string
	systemPrompt         string
	tools                []fantasy.AgentTool
	sessions             session.Service
	messages             message.Service
	toolCalls            toolcall.Service
	redisCmd             *redis.CommandService
	disableAutoSummarize bool
	isYolo               bool
	dbQuerier            postgres.Querier // For querying project info

	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, context.CancelFunc]
}

type SessionAgentOptions struct {
	LargeModel           Model
	SmallModel           Model
	SystemPromptPrefix   string
	SystemPrompt         string
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	ToolCalls            toolcall.Service
	RedisCmd             *redis.CommandService
	Tools                []fantasy.AgentTool
	DBQuerier            postgres.Querier
}

func NewSessionAgent(
	opts SessionAgentOptions,
) SessionAgent {
	return &sessionAgent{
		largeModel:           opts.LargeModel,
		smallModel:           opts.SmallModel,
		systemPromptPrefix:   opts.SystemPromptPrefix,
		systemPrompt:         opts.SystemPrompt,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		toolCalls:            opts.ToolCalls,
		redisCmd:             opts.RedisCmd,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                opts.Tools,
		isYolo:               opts.IsYolo,
		dbQuerier:            opts.DBQuerier,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, context.CancelFunc](),
	}
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	//f, err := os.OpenFile("/Users/apple/Downloads/crush-main/logs/all_content.txt",
	//	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	//
	//if _, err := f.WriteString("========================\n"); err != nil {
	//
	//}
	//if err != nil {
	//	panic(err)
	//}
	//defer f.Close()
	if call.Prompt == "" {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
		return nil, nil
	}

	if len(a.tools) > 0 {
		// Add Anthropic caching to the last tool.
		a.tools[len(a.tools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := fantasy.NewAgent(
		a.largeModel.Model,
		fantasy.WithSystemPrompt(a.systemPrompt),
		fantasy.WithTools(a.tools...),
	)
	//if _, err := f.WriteString(a.systemPrompt + "\n"); err != nil {
	//	panic(err)
	//}
	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)

	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	var wg sync.WaitGroup
	// Generate title if first message.
	if len(msgs) == 0 {
		wg.Go(func() {
			sessionLock.Lock()
			a.generateTitle(ctx, &currentSession, call.Prompt)
			sessionLock.Unlock()
		})
	}

	// Add the user message to the session.
	_, err = a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}

	// Add the session to the context.
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)

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

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(call.SessionID, cancel)

	defer cancel()
	defer a.activeRequests.Del(call.SessionID)

	history, files := a.preparePrompt(msgs, call.Attachments...)

	//historyData, err := json.MarshalIndent(history, "", "  ")
	//if err != nil {
	//	panic(err)
	//}
	//if _, err := f.WriteString(string(historyData) + "\n"); err != nil {
	//	panic(err)
	//}

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	//if _, err := f.WriteString(call.Prompt + "\n"); err != nil {
	//	panic(err)
	//}

	var currentAssistant *message.Message
	var shouldSummarize bool
	result, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:           call.Prompt,
		Files:            files,
		Messages:         history,
		ProviderOptions:  call.ProviderOptions,
		MaxOutputTokens:  &call.MaxOutputTokens,
		TopP:             call.TopP,
		Temperature:      call.Temperature,
		PresencePenalty:  call.PresencePenalty,
		TopK:             call.TopK,
		FrequencyPenalty: call.FrequencyPenalty,
		// Before each step create a new assistant message.
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			// Reset all cached items.
			for i := range prepared.Messages {
				prepared.Messages[i].ProviderOptions = nil
			}

			queuedCalls, _ := a.messageQueue.Get(call.SessionID)
			a.messageQueue.Del(call.SessionID)
			for _, queued := range queuedCalls {
				userMessage, createErr := a.createUserMessage(callContext, queued)
				if createErr != nil {
					return callContext, prepared, createErr
				}
				prepared.Messages = append(prepared.Messages, userMessage.ToAIMessage()...)
			}

			lastSystemRoleInx := 0
			systemMessageUpdated := false
			for i, msg := range prepared.Messages {
				// Only add cache control to the last message.
				if msg.Role == fantasy.MessageRoleSystem {
					lastSystemRoleInx = i
				} else if !systemMessageUpdated {
					prepared.Messages[lastSystemRoleInx].ProviderOptions = a.getCacheControlOptions()
					systemMessageUpdated = true
				}
				// Than add cache control to the last 2 messages.
				if i > len(prepared.Messages)-3 {
					prepared.Messages[i].ProviderOptions = a.getCacheControlOptions()
				}
			}

			if promptPrefix := a.promptPrefix(); promptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(promptPrefix)}, prepared.Messages...)
			}

			var assistantMsg message.Message
			assistantMsg, err = a.messages.Create(callContext, call.SessionID, message.CreateMessageParams{
				Role:     message.Assistant,
				Parts:    []message.ContentPart{},
				Model:    a.largeModel.ModelCfg.Model,
				Provider: a.largeModel.ModelCfg.Provider,
			})
			if err != nil {
				return callContext, prepared, err
			}
			callContext = context.WithValue(callContext, tools.MessageIDContextKey, assistantMsg.ID)
			currentAssistant = &assistantMsg
			return callContext, prepared, err
		},
		OnReasoningStart: func(id string, reasoning fantasy.ReasoningContent) error {
			currentAssistant.AppendReasoningContent(reasoning.Text)
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnReasoningDelta: func(id string, text string) error {
			// DEBUG: 打印推理/思考流式输出
			fmt.Printf("[REASONING] %s", text)

			currentAssistant.AppendReasoningContent(text)
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// handle anthropic signature
			if anthropicData, ok := reasoning.ProviderMetadata[anthropic.Name]; ok {
				if reasoning, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok {
					currentAssistant.AppendReasoningSignature(reasoning.Signature)
				}
			}
			if googleData, ok := reasoning.ProviderMetadata[google.Name]; ok {
				if reasoning, ok := googleData.(*google.ReasoningMetadata); ok {
					currentAssistant.AppendThoughtSignature(reasoning.Signature, reasoning.ToolID)
				}
			}
			if openaiData, ok := reasoning.ProviderMetadata[openai.Name]; ok {
				if reasoning, ok := openaiData.(*openai.ResponsesReasoningMetadata); ok {
					currentAssistant.SetReasoningResponsesData(reasoning)
				}
			}
			currentAssistant.FinishThinking()
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnTextDelta: func(id string, text string) error {
			// Strip leading newline from initial text content. This is is
			// particularly important in non-interactive mode where leading
			// newlines are very visible.
			if len(currentAssistant.Parts) == 0 {
				text = strings.TrimPrefix(text, "\n")
			}

			// DEBUG: 打印流式文本输出
			fmt.Printf("[STREAM TEXT] %s", text)

			currentAssistant.AppendContent(text)
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnToolInputStart: func(id string, toolName string) error {
			// DEBUG: 打印工具调用开始
			fmt.Printf("\n[TOOL START] id=%s, name=%s\n", id, toolName)

			toolCall := message.ToolCall{
				ID:               id,
				Name:             toolName,
				ProviderExecuted: false,
				Finished:         false,
			}
			currentAssistant.AddToolCall(toolCall)

			// Track tool call state in database and Redis
			if a.toolCalls != nil {
				messageID := ""
				if currentAssistant != nil {
					messageID = currentAssistant.ID
				}
				_, tcErr := a.toolCalls.Create(genCtx, call.SessionID, messageID, id, toolName)
				if tcErr != nil {
					slog.Warn("Failed to create tool call record", "tool_call_id", id, "error", tcErr)
				}
			}

			// Update Redis for real-time status and publish to frontend
			if a.redisCmd != nil {
				_ = a.redisCmd.SetToolCallState(genCtx, redis.ToolCallState{
					ID:        id,
					SessionID: call.SessionID,
					MessageID: currentAssistant.ID,
					Name:      toolName,
					Status:    "pending",
				})
				// Publish tool call update to frontend via Redis
				_ = a.redisCmd.PublishToolCallUpdate(genCtx, redis.ToolCallUpdatePayload{
					ID:        id,
					SessionID: call.SessionID,
					MessageID: currentAssistant.ID,
					Name:      toolName,
					Status:    "pending",
				})
			}

			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnRetry: func(err *fantasy.ProviderError, delay time.Duration) {
			// TODO: implement
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			// DEBUG: 打印工具调用完成 (含参数)
			fmt.Printf("\n[TOOL CALL] id=%s, name=%s, input=%s\n", tc.ToolCallID, tc.ToolName, tc.Input)

			toolCall := message.ToolCall{
				ID:               tc.ToolCallID,
				Name:             tc.ToolName,
				Input:            tc.Input,
				ProviderExecuted: false,
				Finished:         true,
			}
			currentAssistant.AddToolCall(toolCall)

			// Update tool call state to running with input
			if a.toolCalls != nil {
				if err := a.toolCalls.UpdateInput(genCtx, tc.ToolCallID, tc.Input); err != nil {
					slog.Warn("Failed to update tool call input", "tool_call_id", tc.ToolCallID, "error", err)
				}
			}

			// Update Redis for real-time status and publish to frontend
			if a.redisCmd != nil {
				_ = a.redisCmd.SetToolCallState(genCtx, redis.ToolCallState{
					ID:        tc.ToolCallID,
					SessionID: call.SessionID,
					MessageID: currentAssistant.ID,
					Name:      tc.ToolName,
					Status:    "running",
					Input:     tc.Input,
				})
				// Publish tool call update to frontend via Redis
				_ = a.redisCmd.PublishToolCallUpdate(genCtx, redis.ToolCallUpdatePayload{
					ID:        tc.ToolCallID,
					SessionID: call.SessionID,
					MessageID: currentAssistant.ID,
					Name:      tc.ToolName,
					Input:     tc.Input,
					Status:    "running",
				})
			}

			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(*currentAssistant)
			return nil
		},
		OnToolResult: func(result fantasy.ToolResultContent) error {
			var resultContent string
			isError := false
			switch result.Result.GetType() {
			case fantasy.ToolResultContentTypeText:
				r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result)
				if ok {
					resultContent = r.Text
				}
			case fantasy.ToolResultContentTypeError:
				r, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result)
				if ok {
					isError = true
					resultContent = r.Error.Error()
				}
			case fantasy.ToolResultContentTypeMedia:
				// TODO: handle this message type
			}

			// DEBUG: 打印工具调用结果
			fmt.Printf("\n[TOOL RESULT] id=%s, name=%s, isError=%v, content=%s\n", result.ToolCallID, result.ToolName, isError, resultContent)

			// Update tool call state to completed/error
			if a.toolCalls != nil {
				errorMsg := ""
				if isError {
					errorMsg = resultContent
				}
				if err := a.toolCalls.Complete(genCtx, result.ToolCallID, resultContent, isError, errorMsg); err != nil {
					slog.Warn("Failed to complete tool call", "tool_call_id", result.ToolCallID, "error", err)
				}
			}

			// Update Redis for real-time status and publish to frontend
			if a.redisCmd != nil {
				status := "completed"
				if isError {
					status = "error"
				}
				_ = a.redisCmd.SetToolCallState(genCtx, redis.ToolCallState{
					ID:        result.ToolCallID,
					SessionID: call.SessionID,
					MessageID: currentAssistant.ID,
					Name:      result.ToolName,
					Status:    status,
				})
				// Publish tool call update to frontend via Redis
				_ = a.redisCmd.PublishToolCallUpdate(genCtx, redis.ToolCallUpdatePayload{
					ID:           result.ToolCallID,
					SessionID:    call.SessionID,
					MessageID:    currentAssistant.ID,
					Name:         result.ToolName,
					Status:       status,
					Result:       resultContent,
					IsError:      isError,
					ErrorMessage: resultContent,
				})
			}

			toolResult := message.ToolResult{
				ToolCallID: result.ToolCallID,
				Name:       result.ToolName,
				Content:    resultContent,
				IsError:    isError,
				Metadata:   result.ClientMetadata,
			}
			_, createMsgErr := a.messages.Create(genCtx, currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createMsgErr != nil {
				return createMsgErr
			}
			return nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			finishReason := message.FinishReasonUnknown
			switch stepResult.FinishReason {
			case fantasy.FinishReasonLength:
				finishReason = message.FinishReasonMaxTokens
			case fantasy.FinishReasonStop:
				finishReason = message.FinishReasonEndTurn
			case fantasy.FinishReasonToolCalls:
				finishReason = message.FinishReasonToolUse
			}
			currentAssistant.AddFinish(finishReason, "", "")
			a.updateSessionUsage(a.largeModel, &currentSession, stepResult.Usage, a.openrouterCost(stepResult.ProviderMetadata))
			sessionLock.Lock()
			_, sessionErr := a.sessions.Save(genCtx, currentSession)
			sessionLock.Unlock()
			if sessionErr != nil {
				return sessionErr
			}
			return a.messages.Update(genCtx, *currentAssistant)
		},
		StopWhen: []fantasy.StopCondition{
			func(_ []fantasy.StepResult) bool {
				cw := int64(a.largeModel.CatwalkCfg.ContextWindow)
				tokens := currentSession.CompletionTokens + currentSession.PromptTokens
				remaining := cw - tokens
				var threshold int64
				if cw > 200_000 {
					threshold = 20_000
				} else {
					threshold = int64(float64(cw) * 0.2)
				}
				if (remaining <= threshold) && !a.disableAutoSummarize {
					shouldSummarize = true
					return true
				}
				return false
			},
		},
	})
	//-----------------
	//data, err := json.MarshalIndent(result.Response.Content, "", "  ")
	//if err != nil {
	//	panic(err)
	//}

	// 追加写入 all_content.txt

	// 可以在每次写入前加换行分隔
	//if _, err := f.Write(append(data, '\n')); err != nil {
	//	panic(err)
	//}
	////-----------------
	//if _, err := f.WriteString("========================\n"); err != nil {
	//
	//}

	a.eventPromptResponded(call.SessionID, time.Since(startTime).Truncate(time.Second))

	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
		if currentAssistant == nil {
			return result, err
		}
		// Ensure we finish thinking on error to close the reasoning state.
		currentAssistant.FinishThinking()
		toolCalls := currentAssistant.ToolCalls()
		// INFO: we use the parent context here because the genCtx has been cancelled.
		msgs, createErr := a.messages.List(ctx, currentAssistant.SessionID)
		if createErr != nil {
			return nil, createErr
		}
		for _, tc := range toolCalls {
			if !tc.Finished {
				tc.Finished = true
				tc.Input = "{}"
				currentAssistant.AddToolCall(tc)
				updateErr := a.messages.Update(ctx, *currentAssistant)
				if updateErr != nil {
					return nil, updateErr
				}
			}

			found := false
			for _, msg := range msgs {
				if msg.Role == message.Tool {
					for _, tr := range msg.ToolResults() {
						if tr.ToolCallID == tc.ID {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				continue
			}
			content := "There was an error while executing the tool"
			if isCancelErr {
				content = "Tool execution canceled by user"
			} else if isPermissionErr {
				content = "User denied permission"
			}
			toolResult := message.ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
				IsError:    true,
			}
			_, createErr = a.messages.Create(context.Background(), currentAssistant.SessionID, message.CreateMessageParams{
				Role: message.Tool,
				Parts: []message.ContentPart{
					toolResult,
				},
			})
			if createErr != nil {
				return nil, createErr
			}
		}
		var fantasyErr *fantasy.Error
		var providerErr *fantasy.ProviderError
		const defaultTitle = "Provider Error"
		if isCancelErr {
			currentAssistant.AddFinish(message.FinishReasonCanceled, "User canceled request", "")
		} else if isPermissionErr {
			currentAssistant.AddFinish(message.FinishReasonPermissionDenied, "User denied permission", "")
		} else if errors.As(err, &providerErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(providerErr.Title), defaultTitle), providerErr.Message)
		} else if errors.As(err, &fantasyErr) {
			currentAssistant.AddFinish(message.FinishReasonError, cmp.Or(stringext.Capitalize(fantasyErr.Title), defaultTitle), fantasyErr.Message)
		} else {
			currentAssistant.AddFinish(message.FinishReasonError, defaultTitle, err.Error())
		}
		// Note: we use the parent context here because the genCtx has been
		// cancelled.
		updateErr := a.messages.Update(ctx, *currentAssistant)
		if updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	wg.Wait()

	if shouldSummarize {
		a.activeRequests.Del(call.SessionID)
		if summarizeErr := a.Summarize(genCtx, call.SessionID, call.ProviderOptions); summarizeErr != nil {
			return nil, summarizeErr
		}
		// If the agent wasn't done...
		if len(currentAssistant.ToolCalls()) > 0 {
			existing, ok := a.messageQueue.Get(call.SessionID)
			if !ok {
				existing = []SessionAgentCall{}
			}
			call.Prompt = fmt.Sprintf("The previous session was interrupted because it got too long, the initial user request was: `%s`", call.Prompt)
			existing = append(existing, call)
			a.messageQueue.Set(call.SessionID, existing)
		}
	}

	// Release active request before processing queued messages.
	a.activeRequests.Del(call.SessionID)
	cancel()

	queuedMessages, ok := a.messageQueue.Get(call.SessionID)
	if !ok || len(queuedMessages) == 0 {
		return result, err
	}
	// There are queued messages restart the loop.
	firstQueuedMessage := queuedMessages[0]
	a.messageQueue.Set(call.SessionID, queuedMessages[1:])
	return a.Run(ctx, firstQueuedMessage)
}

func (a *sessionAgent) Summarize(ctx context.Context, sessionID string, opts fantasy.ProviderOptions) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		// Nothing to summarize.
		return nil
	}

	aiMsgs, _ := a.preparePrompt(msgs)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()

	agent := fantasy.NewAgent(a.largeModel.Model,
		fantasy.WithSystemPrompt(string(summaryPrompt)),
	)
	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            a.largeModel.Model.Model(),
		Provider:         a.largeModel.Model.Provider(),
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	resp, err := agent.Stream(genCtx, fantasy.AgentStreamCall{
		Prompt:          "Provide a detailed summary of our conversation above.",
		Messages:        aiMsgs,
		ProviderOptions: opts,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(summaryMessage)
			return nil
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			// Handle anthropic signature.
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(summaryMessage)
			return nil
		},
		OnTextDelta: func(id, text string) error {
			summaryMessage.AppendContent(text)
			// Only publish to frontend, don't write to DB during streaming
			a.messages.PublishUpdate(summaryMessage)
			return nil
		},
	})
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			// User cancelled summarize we need to remove the summary message.
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return deleteErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.largeModel, &currentSession, resp.TotalUsage, openrouterCost)

	// Just in case, get just the last usage info.
	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = usage.OutputTokens
	currentSession.PromptTokens = 0
	_, err = a.sessions.Save(genCtx, currentSession)
	return err
}

func (a *sessionAgent) getCacheControlOptions() fantasy.ProviderOptions {
	if t, _ := strconv.ParseBool(os.Getenv("CRUSH_DISABLE_ANTHROPIC_CACHE")); t {
		return fantasy.ProviderOptions{}
	}
	return fantasy.ProviderOptions{
		anthropic.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
		bedrock.Name: &anthropic.ProviderCacheControlOptions{
			CacheControl: anthropic.CacheControl{Type: "ephemeral"},
		},
	}
}

func (a *sessionAgent) createUserMessage(ctx context.Context, call SessionAgentCall) (message.Message, error) {
	fmt.Println("\n=== Agent: 创建用户消息 ===")
	fmt.Printf("接收到的附件数量: %d\n", len(call.Attachments))

	var attachmentParts []message.ContentPart
	for i, attachment := range call.Attachments {
		fmt.Printf("[附件 %d/%d]\n", i+1, len(call.Attachments))
		fmt.Printf("  - FilePath: %s\n", attachment.FilePath)
		fmt.Printf("  - FileName: %s\n", attachment.FileName)
		fmt.Printf("  - MimeType: %s\n", attachment.MimeType)
		fmt.Printf("  - Content Size: %d bytes\n", len(attachment.Content))
		attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
	}

	parts := []message.ContentPart{message.TextContent{Text: call.Prompt}}
	parts = append(parts, attachmentParts...)
	fmt.Printf("总共创建 %d 个内容部分 (1 文本 + %d 附件)\n", len(parts), len(attachmentParts))

	msg, err := a.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
	if err != nil {
		fmt.Printf("❌ 创建消息失败: %v\n", err)
		return message.Message{}, fmt.Errorf("failed to create user message: %w", err)
	}
	fmt.Printf("✅ 用户消息创建成功，消息ID: %s\n", msg.ID)
	fmt.Println("=== Agent: 用户消息创建完成 ===\n")
	return msg, nil
}

func (a *sessionAgent) preparePrompt(msgs []message.Message, attachments ...message.Attachment) ([]fantasy.Message, []fantasy.FilePart) {
	fmt.Println("\n=== Agent: 准备 Prompt ===")

	// Hydrate binary contents in historical messages (fetch image data from URLs)
	fmt.Println("=== Agent: 水合历史消息中的图片数据 ===")
	if err := message.HydrateMessages(msgs, createImageFetcher()); err != nil {
		fmt.Printf("⚠️ 警告: 水合图片数据失败: %v\n", err)
		slog.Warn("Failed to hydrate binary contents", "error", err)
	}
	fmt.Println("=== Agent: 图片数据水合完成 ===")

	var history []fantasy.Message
	for _, m := range msgs {
		if len(m.Parts) == 0 {
			continue
		}
		// Assistant message without content or tool calls (cancelled before it
		// returned anything).
		if m.Role == message.Assistant && len(m.ToolCalls()) == 0 && m.Content().Text == "" && m.ReasoningContent().String() == "" {
			continue
		}
		history = append(history, m.ToAIMessage()...)
	}
	fmt.Printf("历史消息数量: %d\n", len(history))

	fmt.Printf("当前请求的附件数量: %d\n", len(attachments))
	var files []fantasy.FilePart
	for i, attachment := range attachments {
		fmt.Printf("[附件 %d/%d] 转换为 FilePart\n", i+1, len(attachments))
		fmt.Printf("  - Filename: %s\n", attachment.FileName)
		fmt.Printf("  - MediaType: %s\n", attachment.MimeType)
		fmt.Printf("  - Data Size: %d bytes\n", len(attachment.Content))
		files = append(files, fantasy.FilePart{
			Filename:  attachment.FileName,
			Data:      attachment.Content,
			MediaType: attachment.MimeType,
		})
	}
	fmt.Printf("✅ Prompt 准备完成：%d 条历史消息 + %d 个文件附件\n", len(history), len(files))
	fmt.Println("=== Agent: Prompt 准备完成 ===\n")

	return history, files
}

// createImageFetcher creates an ImageFetcher function that fetches image data from URLs.
// It supports both MinIO URLs and external HTTP URLs.
func createImageFetcher() message.ImageFetcher {
	return func(url string) ([]byte, string, error) {
		// Try MinIO client first if available
		minioClient := storage.GetMinIOClient()
		if minioClient != nil && minioClient.IsMinIOURL(url) {
			fmt.Printf("[ImageFetcher] Fetching from MinIO: %s\n", url)
			return minioClient.GetFile(context.Background(), url)
		}

		// Fetch from external URL
		fmt.Printf("[ImageFetcher] Fetching from external URL: %s\n", url)
		return fetchImageFromURL(url)
	}
}

// fetchImageFromURL fetches an image from an external URL.
func fetchImageFromURL(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	return data, mimeType, nil
}

func (a *sessionAgent) getSessionMessages(ctx context.Context, session session.Session) ([]message.Message, error) {
	msgs, err := a.messages.List(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}
	return msgs, nil
}

func (a *sessionAgent) generateTitle(ctx context.Context, session *session.Session, prompt string) {
	if prompt == "" {
		return
	}

	var maxOutput int64 = 40
	if a.smallModel.CatwalkCfg.CanReason {
		maxOutput = a.smallModel.CatwalkCfg.DefaultMaxTokens
	}

	agent := fantasy.NewAgent(a.smallModel.Model,
		fantasy.WithSystemPrompt(string(titlePrompt)+"\n /no_think"),
		fantasy.WithMaxOutputTokens(maxOutput),
	)

	resp, err := agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt: fmt.Sprintf("Generate a concise title for the following content:\n\n%s\n <think>\n\n</think>", prompt),
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if a.systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(a.systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
	})
	if err != nil {
		slog.Error("error generating title", "err", err)
		return
	}

	title := resp.Response.Content.Text()

	title = strings.ReplaceAll(title, "\n", " ")

	// Remove thinking tags if present.
	if idx := strings.Index(title, "</think>"); idx > 0 {
		title = title[idx+len("</think>"):]
	}

	title = strings.TrimSpace(title)
	if title == "" {
		slog.Warn("failed to generate title", "warn", "empty title")
		return
	}

	session.Title = title

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(a.smallModel, session, resp.TotalUsage, openrouterCost)
	_, saveErr := a.sessions.Save(ctx, *session)
	if saveErr != nil {
		slog.Error("failed to save session title & usage", "error", saveErr)
		return
	}
}

func (a *sessionAgent) openrouterCost(metadata fantasy.ProviderMetadata) *float64 {
	openrouterMetadata, ok := metadata[openrouter.Name]
	if !ok {
		return nil
	}

	opts, ok := openrouterMetadata.(*openrouter.ProviderMetadata)
	if !ok {
		return nil
	}
	return &opts.Usage.Cost
}

func (a *sessionAgent) updateSessionUsage(model Model, session *session.Session, usage fantasy.Usage, overrideCost *float64) {
	modelConfig := model.CatwalkCfg
	cost := modelConfig.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
		modelConfig.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
		modelConfig.CostPer1MIn/1e6*float64(usage.InputTokens) +
		modelConfig.CostPer1MOut/1e6*float64(usage.OutputTokens)

	if a.isClaudeCode() {
		cost = 0
	}

	a.eventTokensUsed(session.ID, model, usage, cost)

	if overrideCost != nil {
		session.Cost += *overrideCost
	} else {
		session.Cost += cost
	}

	session.CompletionTokens = usage.OutputTokens + usage.CacheReadTokens
	session.PromptTokens = usage.InputTokens + usage.CacheCreationTokens
}

func (a *sessionAgent) Cancel(sessionID string) {
	// Cancel regular requests.
	if cancel, ok := a.activeRequests.Take(sessionID); ok && cancel != nil {
		slog.Info("Request cancellation initiated", "session_id", sessionID)
		cancel()
	}

	// Also check for summarize requests.
	if cancel, ok := a.activeRequests.Take(sessionID + "-summarize"); ok && cancel != nil {
		slog.Info("Summarize cancellation initiated", "session_id", sessionID)
		cancel()
	}

	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}

	// Cancel all pending tool calls for this session
	if a.toolCalls != nil {
		ctx := context.Background()
		if err := a.toolCalls.CancelSession(ctx, sessionID); err != nil {
			slog.Warn("Failed to cancel session tool calls", "session_id", sessionID, "error", err)
		}
	}

	// Clear Redis tool call states
	if a.redisCmd != nil {
		ctx := context.Background()
		if err := a.redisCmd.ClearSessionToolCalls(ctx, sessionID); err != nil {
			slog.Warn("Failed to clear Redis tool call states", "session_id", sessionID, "error", err)
		}
	}
}

func (a *sessionAgent) ClearQueue(sessionID string) {
	if a.QueuedPrompts(sessionID) > 0 {
		slog.Info("Clearing queued prompts", "session_id", sessionID)
		a.messageQueue.Del(sessionID)
	}
}

func (a *sessionAgent) CancelAll() {
	if !a.IsBusy() {
		return
	}
	for key := range a.activeRequests.Seq2() {
		a.Cancel(key) // key is sessionID
	}

	timeout := time.After(5 * time.Second)
	for a.IsBusy() {
		select {
		case <-timeout:
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (a *sessionAgent) IsBusy() bool {
	var busy bool
	for cancelFunc := range a.activeRequests.Seq() {
		if cancelFunc != nil {
			busy = true
			break
		}
	}
	return busy
}

func (a *sessionAgent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Get(sessionID)
	return busy
}

func (a *sessionAgent) QueuedPrompts(sessionID string) int {
	l, ok := a.messageQueue.Get(sessionID)
	if !ok {
		return 0
	}
	return len(l)
}

func (a *sessionAgent) SetModels(large Model, small Model) {
	a.largeModel = large
	a.smallModel = small
}

func (a *sessionAgent) SetTools(tools []fantasy.AgentTool) {
	a.tools = tools
}

func (a *sessionAgent) Model() Model {
	return a.largeModel
}

func (a *sessionAgent) promptPrefix() string {
	if a.isClaudeCode() {
		return "You are Claude Code, Anthropic's official CLI for Claude."
	}
	return a.systemPromptPrefix
}

func (a *sessionAgent) isClaudeCode() bool {
	cfg := config.Get()
	pc, ok := cfg.Providers.Get(a.largeModel.ModelCfg.Provider)
	return ok && pc.ID == string(catwalk.InferenceProviderAnthropic) && pc.OAuthToken != nil
}

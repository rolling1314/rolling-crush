// Package app wires together services, coordinates agents, and manages
// application lifecycle.
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	apihttp "github.com/rolling1314/rolling-crush/cmd/http-server/handler"
	apiws "github.com/rolling1314/rolling-crush/cmd/ws-server/handler"
	"github.com/rolling1314/rolling-crush/domain/history"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/domain/project"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/toolcall"
	"github.com/rolling1314/rolling-crush/domain/user"
	"github.com/rolling1314/rolling-crush/infra/postgres"
	storeredis "github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
	"github.com/rolling1314/rolling-crush/infra/storage"
	"github.com/rolling1314/rolling-crush/internal/agent"
	"github.com/rolling1314/rolling-crush/internal/agent/tools/mcp"
	"github.com/rolling1314/rolling-crush/internal/lsp"
	"github.com/rolling1314/rolling-crush/internal/pkg/csync"
	"github.com/rolling1314/rolling-crush/internal/pkg/format"
	"github.com/rolling1314/rolling-crush/internal/pkg/log"
	"github.com/rolling1314/rolling-crush/internal/pkg/term"
	"github.com/rolling1314/rolling-crush/internal/pubsub"
	"github.com/rolling1314/rolling-crush/internal/shell"
	"github.com/rolling1314/rolling-crush/internal/tui/components/anim"
	"github.com/rolling1314/rolling-crush/internal/tui/styles"
	"github.com/rolling1314/rolling-crush/internal/update"
	"github.com/rolling1314/rolling-crush/internal/version"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	ToolCalls   toolcall.Service
	History     history.Service
	Permissions permission.Service
	Users       user.Service
	Projects    project.Service

	AgentCoordinator agent.Coordinator

	LSPClients *csync.Map[string, *lsp.Client]

	config *config.Config
	db     *postgres.Queries // Add DB queries for session config loading

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	WSServer   *apiws.Server
	HTTPServer *apihttp.Server

	// Redis stream service for message buffering during WebSocket disconnection
	RedisStream *storeredis.StreamService

	// Track the current active session for the single-user mode
	currentSessionID string

	// Track connected sessions (session ID -> connected status)
	connectedSessions *csync.Map[string, bool]

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func() error
}

// New initializes a new applcation instance.
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
	q := postgres.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	toolCalls := toolcall.NewService(q)
	files := history.NewService(q, conn)
	users := user.NewService(q)
	projects := project.NewService(q)
	skipPermissionsRequests := cfg.Permissions != nil && cfg.Permissions.SkipRequests
	allowedTools := []string{}
	if cfg.Permissions != nil && cfg.Permissions.AllowedTools != nil {
		allowedTools = cfg.Permissions.AllowedTools
	}

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		ToolCalls:   toolCalls,
		History:     files,
		Users:       users,
		Projects:    projects,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools),
		LSPClients:  csync.NewMap[string, *lsp.Client](),

		globalCtx: ctx,

		config: cfg,
		db:     q, // Store DB queries for session config loading

		events:            make(chan tea.Msg, 100),
		serviceEventsWG:   &sync.WaitGroup{},
		tuiWG:             &sync.WaitGroup{},
		connectedSessions: csync.NewMap[string, bool](),

		WSServer:   apiws.New(),
		HTTPServer: apihttp.New("8001", users, projects, sessions, messages, q, cfg),
	}

	// Initialize Redis client and stream service
	if err := storeredis.InitGlobalClient(); err != nil {
		slog.Warn("Failed to initialize Redis client, message buffering will be unavailable", "error", err)
	} else {
		app.RedisStream = storeredis.GetGlobalStreamService()
		slog.Info("Redis stream service initialized")
	}

	// Register the handler for incoming WebSocket messages
	app.WSServer.SetMessageHandler(app.HandleClientMessage)
	fmt.Println("=== WebSocket message handler registered ===")
	fmt.Println("Handler function:", app.HandleClientMessage != nil)

	// Register disconnect handler to clean up agent state when WebSocket disconnects
	app.WSServer.SetDisconnectHandler(app.HandleClientDisconnect)

	app.setupEvents()

	// Initialize storage client from app config
	appCfg := config.GetGlobalAppConfig()
	if err := storage.InitGlobalClientFromConfig(appCfg); err != nil {
		slog.Warn("Failed to initialize storage client from config, trying default config", "error", err)
		// Fallback to default initialization
		if err := storage.InitGlobalMinIOClient(); err != nil {
			slog.Warn("Failed to initialize storage client, image upload will be unavailable", "error", err)
		}
	}

	// Initialize sandbox client from app config
	if appCfg.Sandbox.BaseURL != "" {
		sandbox.SetDefaultClient(appCfg.Sandbox.BaseURL)
		slog.Info("Sandbox client configured", "base_url", appCfg.Sandbox.BaseURL)
	}

	// Initialize LSP clients in the background.
	app.initLSPClients(ctx)

	// Check for updates in the background.
	go app.checkForUpdates(ctx)

	go func() {
		slog.Info("Initializing MCP clients")
		mcp.Initialize(ctx, app.Permissions, cfg)
	}()

	// cleanup database upon app shutdown
	app.cleanupFuncs = append(app.cleanupFuncs, conn.Close, mcp.Close)

	// TODO: remove the concept of agent config, most likely.
	// Try to initialize agent if config is available
	// In Web mode, agent may be initialized later when session config is loaded
	if cfg.IsConfigured() {
		if err := app.InitCoderAgent(ctx); err != nil {
			slog.Warn("Failed to initialize coder agent, will retry later", "error", err)
			// Don't fail the app, agent can be initialized later
		}
	} else {
		slog.Warn("No agent configuration found, agent will be initialized when session config is loaded")
	}

	return app, nil
}

// HandleClientDisconnect handles WebSocket disconnection
// Instead of cancelling the agent, we mark the session as disconnected so messages
// continue to be buffered in Redis for later retrieval
func (app *App) HandleClientDisconnect() {
	fmt.Println("=== HandleClientDisconnect called ===")
	slog.Info("WebSocket client disconnected", "sessionID", app.currentSessionID)

	// Mark session as disconnected but DON'T cancel the agent
	// The agent will continue running and messages will be buffered in Redis
	if app.currentSessionID != "" {
		app.connectedSessions.Set(app.currentSessionID, false)

		// Update Redis connection status
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetConnectionStatus(ctx, app.currentSessionID, false); err != nil {
				slog.Warn("Failed to update Redis connection status", "error", err)
			}
		}

		fmt.Printf("Session %s marked as disconnected, agent continues running\n", app.currentSessionID)
		slog.Info("Session marked as disconnected, agent continues running", "sessionID", app.currentSessionID)
	}

	// Clear the current session ID so new connections start fresh
	app.currentSessionID = ""
	fmt.Println("Current session ID cleared")
}

// ImageAttachment represents an image attached to a message
type ImageAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Filename string `json:"filename"`
}

// HandleClientMessage processes messages from the WebSocket client
func (app *App) HandleClientMessage(rawMsg []byte) {
	fmt.Println("=== HandleClientMessage called ===")
	fmt.Println("Raw message:", string(rawMsg))

	type ClientMsg struct {
		Type       string            `json:"type"`
		Content    string            `json:"content"`
		SessionID  string            `json:"sessionID"` // Optional: if frontend sends it
		ID         string            `json:"id"`
		ToolCallID string            `json:"tool_call_id"`
		Granted    bool              `json:"granted"`
		Denied     bool              `json:"denied"`
		Images     []ImageAttachment `json:"images"`    // Image attachments
		LastMsgID  string            `json:"lastMsgId"` // For reconnection - last received Redis stream message ID
	}

	var msg ClientMsg
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		slog.Error("Failed to unmarshal client message", "error", err)
		return
	}

	fmt.Println("Parsed message type:", msg.Type, "content:", msg.Content, "sessionID:", msg.SessionID)

	// Handle reconnection request - client wants to resume receiving messages
	if msg.Type == "reconnect" {
		app.handleReconnection(msg.SessionID, msg.LastMsgID)
		return
	}

	// Handle permission responses
	if msg.Type == "permission_response" {
		// Find the permission request by ID
		ctx := context.Background()
		permissionChan := app.Permissions.Subscribe(ctx)

		// Create a permission request object
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

		// Clean up subscription
		go func() {
			<-permissionChan
		}()
		return
	}

	// Handle cancel requests - 取消当前会话的 agent 请求
	if msg.Type == "cancel" {
		sessionID := msg.SessionID
		if sessionID == "" {
			sessionID = app.currentSessionID
		}
		if sessionID != "" && app.AgentCoordinator != nil {
			fmt.Printf("[CANCEL] Cancelling agent request for session: %s\n", sessionID)
			slog.Info("Cancelling agent request", "sessionID", sessionID)
			app.AgentCoordinator.Cancel(sessionID)
		}
		return
	}

	// Use existing session or create new one
	sessionID := msg.SessionID
	fmt.Println("Processing message, sessionID from message:", sessionID)

	if sessionID == "" {
		fmt.Println("No sessionID in message, checking currentSessionID:", app.currentSessionID)
		if app.currentSessionID == "" {
			fmt.Println("Creating new session...")
			// Create a default session if none exists
			sess, err := app.Sessions.Create(context.Background(), "", "Web Session")
			if err != nil {
				slog.Error("Failed to create session", "error", err)
				return
			}
			app.currentSessionID = sess.ID
			fmt.Println("Created session with ID:", sess.ID)
			// Don't auto-approve - let frontend handle permissions
			// app.Permissions.AutoApproveSession(sess.ID)
		}
		sessionID = app.currentSessionID
	} else {
		app.currentSessionID = sessionID
	}

	// Mark session as connected
	app.connectedSessions.Set(sessionID, true)
	if app.RedisStream != nil {
		ctx := context.Background()
		if err := app.RedisStream.SetConnectionStatus(ctx, sessionID, true); err != nil {
			slog.Warn("Failed to update Redis connection status", "error", err)
		}
	}

	fmt.Println("Final sessionID:", sessionID)
	slog.Info("Received message from client", "content", msg.Content, "sessionID", sessionID)

	// Ensure AgentCoordinator is initialized
	if app.AgentCoordinator == nil {
		fmt.Println("AgentCoordinator is nil, attempting to initialize...")
		slog.Warn("AgentCoordinator not initialized, attempting to initialize now")
		if err := app.InitCoderAgent(context.Background()); err != nil {
			fmt.Println("Failed to initialize AgentCoordinator:", err)
			slog.Error("Failed to initialize AgentCoordinator", "error", err)
			return
		}
		fmt.Println("AgentCoordinator initialized successfully")
	} else {
		fmt.Println("AgentCoordinator already initialized")
	}

	// Fetch image attachments if any
	var attachments []message.Attachment
	fmt.Println("=== 开始检查图片附件 ===")
	fmt.Printf("收到的消息中包含图片数量: %d\n", len(msg.Images))
	if len(msg.Images) > 0 {
		fmt.Printf("Processing %d image attachments\n", len(msg.Images))
		minioClient := storage.GetMinIOClient()

		for i, img := range msg.Images {
			fmt.Printf("\n[图片 %d/%d] 开始处理\n", i+1, len(msg.Images))
			fmt.Printf("  - URL: %s\n", img.URL)
			fmt.Printf("  - Filename: %s\n", img.Filename)
			fmt.Printf("  - MimeType: %s\n", img.MimeType)
			fmt.Printf("Fetching image: %s\n", img.URL)

			var imageData []byte
			var mimeType string
			var err error

			// Check if it's a MinIO URL and fetch accordingly
			if minioClient != nil && minioClient.IsMinIOURL(img.URL) {
				fmt.Println("  - 检测到 MinIO URL，从 MinIO 获取图片")
				imageData, mimeType, err = minioClient.GetFile(context.Background(), img.URL)
			} else {
				// Fetch from external URL
				fmt.Println("  - 检测到外部 URL，开始下载图片")
				imageData, mimeType, err = fetchImageFromURL(img.URL)
			}

			if err != nil {
				fmt.Printf("  ❌ Failed to fetch image %s: %v\n", img.URL, err)
				slog.Error("Failed to fetch image", "url", img.URL, "error", err)
				continue
			}
			fmt.Printf("  ✅ 图片下载成功！大小: %d bytes, MIME类型: %s\n", len(imageData), mimeType)

			// Use provided mime type if available
			if img.MimeType != "" {
				fmt.Printf("  - 使用客户端提供的 MIME 类型: %s\n", img.MimeType)
				mimeType = img.MimeType
			}

			filename := img.Filename
			if filename == "" {
				// Extract filename from URL
				parts := strings.Split(img.URL, "/")
				filename = parts[len(parts)-1]
				fmt.Printf("  - 从 URL 提取文件名: %s\n", filename)
			} else {
				fmt.Printf("  - 使用客户端提供的文件名: %s\n", filename)
			}

			attachments = append(attachments, message.Attachment{
				FilePath: img.URL,
				FileName: filename,
				MimeType: mimeType,
				Content:  imageData,
			})
			fmt.Printf("  ✅ Image attachment added: %s (%s, %d bytes)\n", filename, mimeType, len(imageData))
			fmt.Printf("[图片 %d/%d] 处理完成\n", i+1, len(msg.Images))
		}
	} else {
		fmt.Println("  - 没有图片附件")
	}
	fmt.Printf("\n=== 图片处理完成，共添加 %d 个附件 ===\n\n", len(attachments))

	fmt.Println("\n=== About to call AgentCoordinator.Run in goroutine ===")
	fmt.Printf("准备传递的附件数量: %d\n", len(attachments))
	for i, att := range attachments {
		fmt.Printf("  [附件 %d] FileName: %s, MimeType: %s, Size: %d bytes\n",
			i+1, att.FileName, att.MimeType, len(att.Content))
	}

	// Run the agent asynchronously
	go func() {
		fmt.Println("\n=== Inside goroutine, calling AgentCoordinator.Run ===")
		fmt.Printf("Goroutine 中的附件数量: %d\n", len(attachments))

		// Mark generation as active in Redis
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetActiveGeneration(ctx, sessionID, true); err != nil {
				slog.Warn("Failed to mark generation as active", "error", err)
			}
		}

		_, err := app.AgentCoordinator.Run(context.Background(), sessionID, msg.Content, attachments...)

		// Mark generation as complete in Redis
		if app.RedisStream != nil {
			ctx := context.Background()
			if err := app.RedisStream.SetActiveGeneration(ctx, sessionID, false); err != nil {
				slog.Warn("Failed to mark generation as complete", "error", err)
			}

			// Publish generation complete event
			if err := app.RedisStream.PublishMessage(ctx, sessionID, "generation_complete", map[string]interface{}{
				"session_id": sessionID,
				"error":      err != nil,
			}); err != nil {
				slog.Warn("Failed to publish generation complete event", "error", err)
			}
		}

		// Send generation complete to WebSocket if connected
		isConnected, _ := app.connectedSessions.Get(sessionID)
		if isConnected {
			app.WSServer.SendToSession(sessionID, map[string]interface{}{
				"Type":       "generation_complete",
				"session_id": sessionID,
				"error":      err != nil,
			})
		}

		if err != nil {
			fmt.Println("Agent run error:", err)
			slog.Error("Agent run error", "error", err)
		} else {
			fmt.Println("Agent run completed successfully")
		}
	}()
	fmt.Println("Goroutine started, HandleClientMessage returning")
}

// handleReconnection handles client reconnection and sends missed messages
func (app *App) handleReconnection(sessionID string, lastMsgID string) {
	fmt.Printf("=== handleReconnection called for session %s, lastMsgID: %s ===\n", sessionID, lastMsgID)
	slog.Info("Handling reconnection", "sessionID", sessionID, "lastMsgID", lastMsgID)

	if sessionID == "" {
		slog.Warn("Reconnection request without session ID")
		return
	}

	// Mark session as connected
	app.currentSessionID = sessionID
	app.connectedSessions.Set(sessionID, true)

	if app.RedisStream == nil {
		slog.Warn("Redis stream service not available, cannot replay messages")
		return
	}

	ctx := context.Background()

	// Update Redis connection status
	if err := app.RedisStream.SetConnectionStatus(ctx, sessionID, true); err != nil {
		slog.Warn("Failed to update Redis connection status", "error", err)
	}

	// Read missed messages from Redis stream
	messages, newLastID, err := app.RedisStream.ReadMessages(ctx, sessionID, lastMsgID, 0)
	if err != nil {
		slog.Error("Failed to read missed messages from Redis", "error", err)
		return
	}

	fmt.Printf("Found %d missed messages for session %s\n", len(messages), sessionID)
	slog.Info("Replaying missed messages", "sessionID", sessionID, "count", len(messages))

	// Send missed messages to the client
	for _, msg := range messages {
		var payload interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			slog.Warn("Failed to unmarshal message payload", "error", err)
			continue
		}

		// Send the message with its original type
		app.WSServer.SendToSession(sessionID, map[string]interface{}{
			"_replay":    true,
			"_streamId":  msg.ID,
			"_type":      msg.Type,
			"_timestamp": msg.Timestamp,
			"_payload":   payload,
		})
	}

	// Update last read ID
	if newLastID != "" {
		if err := app.RedisStream.SetLastReadID(ctx, sessionID, newLastID); err != nil {
			slog.Warn("Failed to update last read ID", "error", err)
		}
	}

	// Check if generation is still active
	isActive, err := app.RedisStream.IsGenerationActive(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to check generation status", "error", err)
	} else {
		// Notify client about generation status
		app.WSServer.SendToSession(sessionID, map[string]interface{}{
			"Type":              "reconnection_status",
			"session_id":        sessionID,
			"messages_replayed": len(messages),
			"generation_active": isActive,
			"last_stream_id":    newLastID,
		})
	}

	// Send current session info including context_window
	app.sendSessionUpdate(ctx, sessionID)

	fmt.Printf("Reconnection complete for session %s\n", sessionID)
}

// sendSessionUpdate sends the current session info to the client via WebSocket
// This ensures the client has the latest session data including context_window
func (app *App) sendSessionUpdate(ctx context.Context, sessionID string) {
	// Get session from database
	sess, err := app.Sessions.Get(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to get session for update", "sessionID", sessionID, "error", err)
		return
	}

	// Get context window for this session
	contextWindow := app.getSessionContextWindow(ctx, sessionID)

	slog.Info("Sending session update on connect", "sessionID", sessionID, "context_window", contextWindow, "cost", sess.Cost)

	// Send session update to client
	sessionMsg := map[string]interface{}{
		"Type":              "session_update",
		"id":                sessionID,
		"project_id":        sess.ProjectID,
		"title":             sess.Title,
		"message_count":     sess.MessageCount,
		"prompt_tokens":     sess.PromptTokens,
		"completion_tokens": sess.CompletionTokens,
		"cost":              sess.Cost,
		"context_window":    contextWindow,
		"created_at":        sess.CreatedAt,
		"updated_at":        sess.UpdatedAt,
	}

	app.WSServer.SendToSession(sessionID, sessionMsg)
}

// fetchImageFromURL fetches an image from an external URL
func fetchImageFromURL(url string) ([]byte, string, error) {
	fmt.Printf("    → 开始 HTTP GET 请求: %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("    ❌ HTTP 请求失败: %v\n", err)
		return nil, "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("    → HTTP 状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("    ❌ HTTP 状态码错误: %d\n", resp.StatusCode)
		return nil, "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	fmt.Println("    → 开始读取响应数据...")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("    ❌ 读取数据失败: %v\n", err)
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}
	fmt.Printf("    → 读取完成，数据大小: %d bytes\n", len(data))

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
		fmt.Printf("    → 自动检测 MIME 类型: %s\n", mimeType)
	} else {
		fmt.Printf("    → 从响应头获取 MIME 类型: %s\n", mimeType)
	}

	return data, mimeType, nil
}

// Config returns the application configuration.
func (app *App) Config() *config.Config {
	return app.config
}

// RunNonInteractive runs the application in non-interactive mode with the
// given prompt, printing to stdout.
func (app *App) RunNonInteractive(ctx context.Context, output io.Writer, prompt string, quiet bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var spinner *format.Spinner
	if !quiet {
		t := styles.CurrentTheme()

		// Detect background color to set the appropriate color for the
		// spinner's 'Generating...' text. Without this, that text would be
		// unreadable in light terminals.
		hasDarkBG := true
		if f, ok := output.(*os.File); ok {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, f)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(charmtone.Pepper, t.FgBase)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			LabelColor:  defaultFG,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		})
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !quiet && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}
	defer stopSpinner()

	const maxPromptLengthForTitle = 100
	const titlePrefix = "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := app.Sessions.Create(ctx, "", title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	slog.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive
	// session.
	app.Permissions.AutoApproveSession(sess.ID)

	type response struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan response, 1)

	go func(ctx context.Context, sessionID, prompt string) {
		result, err := app.AgentCoordinator.Run(ctx, sess.ID, prompt)
		if err != nil {
			done <- response{
				err: fmt.Errorf("failed to start agent processing stream: %w", err),
			}
		}
		done <- response{
			result: result,
		}
	}(ctx, sess.ID, prompt)

	messageEvents := app.Messages.Subscribe(ctx)
	messageReadBytes := make(map[string]int)
	supportsProgressBar := term.SupportsProgressBar()

	defer func() {
		if supportsProgressBar {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}

		// Always print a newline at the end. If output is a TTY this will
		// prevent the prompt from overwriting the last line of output.
		_, _ = fmt.Fprintln(output)
	}()

	for {
		if supportsProgressBar {
			// HACK: Reinitialize the terminal progress bar on every iteration so
			// it doesn't get hidden by the terminal due to inactivity.
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case result := <-done:
			stopSpinner()
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, agent.ErrRequestCancelled) {
					slog.Info("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.err)
			}
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				fmt.Fprint(output, part)
				messageReadBytes[msg.ID] = len(content)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

func (app *App) UpdateAgentModel(ctx context.Context) error {
	return app.AgentCoordinator.UpdateModels(ctx)
}

func (app *App) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	setupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "mcp", mcp.SubscribeEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events)
	cleanupFunc := func() error {
		cancel()
		app.serviceEventsWG.Wait()
		return nil
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

// getSessionContextWindow retrieves the context window size for a session from its config
// This mirrors the logic in HTTP handler and TUI components
func (app *App) getSessionContextWindow(ctx context.Context, sessionID string) int64 {
	// Debug: Check if app.config has providers loaded
	if app.config.Providers == nil {
		slog.Error("app.config.Providers is nil!", "session_id", sessionID)
		return 0
	}

	providerCount := 0
	for range app.config.Providers.Seq() {
		providerCount++
	}
	slog.Debug("app.config has providers", "session_id", sessionID, "provider_count", providerCount)

	configJSON, err := app.db.GetSessionConfigJSON(ctx, sessionID)
	slog.Info("getSessionContextWindow called", "session_id", sessionID, "config_json_length", len(configJSON), "error", err)

	if err != nil || configJSON == "" || configJSON == "{}" {
		slog.Warn("No session config found", "session_id", sessionID, "config_json", configJSON, "error", err)
		return 0
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configData); err != nil {
		slog.Error("Failed to parse session config JSON", "session_id", sessionID, "error", err)
		return 0
	}

	slog.Info("Parsed config data", "session_id", sessionID, "has_models", configData["models"] != nil, "has_providers", configData["providers"] != nil)

	if models, ok := configData["models"].(map[string]interface{}); ok {
		slog.Info("Found models in config", "session_id", sessionID, "models_keys", getKeys(models))

		if largeModel, ok := models["large"].(map[string]interface{}); ok {
			provider, _ := largeModel["provider"].(string)
			modelID, _ := largeModel["model"].(string)

			slog.Info("Found large model config", "session_id", sessionID, "provider", provider, "model", modelID)

			if provider != "" && modelID != "" {
				// First try from session config's providers section (if saved)
				if providers, ok := configData["providers"].(map[string]interface{}); ok {
					if providerData, ok := providers[provider].(map[string]interface{}); ok {
						if modelsData, ok := providerData["models"].([]interface{}); ok {
							for _, md := range modelsData {
								if modelData, ok := md.(map[string]interface{}); ok {
									if id, _ := modelData["id"].(string); id == modelID {
										if ctxWindow, ok := modelData["context_window"].(float64); ok && ctxWindow > 0 {
											slog.Info("✅ Found model info in session config providers", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", int64(ctxWindow))
											return int64(ctxWindow)
										}
									}
								}
							}
						}
					}
				}

				// Second try from app.config.Providers
				if providerConfig, ok := app.config.Providers.Get(provider); ok {
					slog.Info("Provider found in config", "provider", provider, "model_count", len(providerConfig.Models))
					for _, m := range providerConfig.Models {
						if m.ID == modelID {
							slog.Info("✅ Found model info in app.config", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", m.ContextWindow)
							return int64(m.ContextWindow)
						}
					}
				}

				// Fallback: try from knownProviders (catwalk providers)
				knownProviders, err := config.Providers(app.config)
				if err == nil {
					for _, p := range knownProviders {
						if string(p.ID) == provider {
							for _, m := range p.Models {
								if m.ID == modelID {
									slog.Info("✅ Found model info in knownProviders", "session_id", sessionID, "provider", provider, "model", modelID, "context_window", m.ContextWindow)
									return int64(m.ContextWindow)
								}
							}
							break
						}
					}
				}

				slog.Warn("❌ Model not found in config or knownProviders", "session_id", sessionID, "provider", provider, "model", modelID)
			} else {
				slog.Warn("Provider or model ID is empty", "session_id", sessionID, "provider", provider, "model", modelID)
			}
		} else {
			slog.Warn("No large model config found in models", "session_id", sessionID)
		}
	} else {
		slog.Warn("No models section in config", "session_id", sessionID)
	}

	return 0
}

// Helper function to get map keys for logging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
) {
	wg.Go(func() {
		subCh := subscriber(ctx)
		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("subscription channel closed", "name", name)
					return
				}
				var msg tea.Msg = event
				select {
				case outputCh <- msg:
				case <-time.After(2 * time.Second):
					slog.Warn("message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					slog.Debug("subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				slog.Debug("subscription cancelled", "name", name)
				return
			}
		}
	})
}

func (app *App) InitCoderAgent(ctx context.Context) error {
	fmt.Println("=== InitCoderAgent called ===")

	// Ensure agent configuration exists (for Web mode)
	if app.config.Agents == nil {
		app.config.Agents = make(map[string]config.Agent)
	}

	coderAgentCfg, ok := app.config.Agents[config.AgentCoder]
	if !ok || coderAgentCfg.ID == "" {
		fmt.Println("No coder agent config found, creating default config")
		// Create a default coder agent config for Web mode
		coderAgentCfg = config.Agent{
			ID:    config.AgentCoder,
			Name:  "Coder",
			Model: config.SelectedModelTypeLarge,
			AllowedTools: []string{
				"agent",
				"agentic_fetch",
				"bash",
				"job_output",
				"job_kill",
				"download",
				"edit",
				"multi_edit",
				"fetch",
				"glob",
				"grep",
				"ls",
				"sourcegraph",
				"view",
				"write",
				"diagnostics",
				"references",
			},
		}
		app.config.Agents[config.AgentCoder] = coderAgentCfg
		fmt.Println("Default coder agent config created")
	}

	var err error
	fmt.Println("Creating coordinator with dbReader:", app.db != nil)

	// Get Redis command service for real-time tool call state updates
	var redisCmd *storeredis.CommandService
	if storeredis.GetClient() != nil {
		redisCmd = storeredis.GetGlobalCommandService()
	}

	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
		app.ToolCalls,
		redisCmd,
		app.Permissions,
		app.History,
		app.LSPClients,
		app.db, // Pass DB queries as DBReader for session config loading
	)
	if err != nil {
		fmt.Println("Failed to create coordinator:", err)
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}
	fmt.Println("Coordinator created successfully")
	return nil
}

// Subscribe handles event processing and broadcasting.
// Note: This was previously connected to the TUI (tea.Program), but now runs independently.
func (app *App) Subscribe() {
	fmt.Println("=== Subscribe() started - listening for events ===")
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("Subscription panic: attempting graceful shutdown")
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func() error {
		slog.Debug("Cancelling message handler")
		tuiCancel()
		app.tuiWG.Wait()
		return nil
	})
	defer app.tuiWG.Done()

	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("Message handler shutting down")
			return
		case msg, ok := <-app.events:
			if !ok {
				slog.Debug("Message channel closed")
				return
			}

			// DEBUG: 打印收到的事件类型
			fmt.Printf("[EVENT] Received event type: %T\n", msg)

			// Send messages to specific session via WebSocket
			if event, ok := msg.(pubsub.Event[message.Message]); ok {
				sessionID := event.Payload.SessionID
				fmt.Printf("[SEND] Sending message to session: ID=%s, Role=%s, SessionID=%s\n", event.Payload.ID, event.Payload.Role, sessionID)

				// Always publish to Redis stream for buffering
				if app.RedisStream != nil {
					ctx := context.Background()
					if err := app.RedisStream.PublishMessage(ctx, sessionID, "message", event.Payload); err != nil {
						slog.Warn("Failed to publish message to Redis stream", "error", err)
					}
				}

				// Check if session is connected before sending via WebSocket
				isConnected, _ := app.connectedSessions.Get(sessionID)
				if isConnected {
					app.WSServer.SendToSession(sessionID, event.Payload)
				} else {
					slog.Debug("Session disconnected, message buffered in Redis", "sessionID", sessionID)
				}
			}

			// Send permission requests to specific session via WebSocket
			if event, ok := msg.(pubsub.Event[permission.PermissionRequest]); ok {
				sessionID := event.Payload.SessionID
				slog.Info("Sending permission request to session", "session_id", sessionID, "tool_call_id", event.Payload.ToolCallID)

				permMsg := map[string]interface{}{
					"Type":         "permission_request",
					"id":           event.Payload.ID,
					"session_id":   sessionID,
					"tool_call_id": event.Payload.ToolCallID,
					"tool_name":    event.Payload.ToolName,
					"description":  event.Payload.Description,
					"action":       event.Payload.Action,
					"params":       event.Payload.Params,
					"path":         event.Payload.Path,
				}

				// Publish to Redis
				if app.RedisStream != nil {
					ctx := context.Background()
					if err := app.RedisStream.PublishMessage(ctx, sessionID, "permission_request", permMsg); err != nil {
						slog.Warn("Failed to publish permission request to Redis stream", "error", err)
					}
				}

				// Send via WebSocket if connected
				isConnected, _ := app.connectedSessions.Get(sessionID)
				if isConnected {
					app.WSServer.SendToSession(sessionID, permMsg)
				}
			}

			// Send permission notifications to specific session via WebSocket
			if event, ok := msg.(pubsub.Event[permission.PermissionNotification]); ok {
				sessionID := event.Payload.SessionID
				slog.Info("Sending permission notification to session", "session_id", sessionID, "tool_call_id", event.Payload.ToolCallID, "granted", event.Payload.Granted)

				notifMsg := map[string]interface{}{
					"Type":         "permission_notification",
					"tool_call_id": event.Payload.ToolCallID,
					"granted":      event.Payload.Granted,
					"denied":       event.Payload.Denied,
				}

				// Publish to Redis
				if app.RedisStream != nil {
					ctx := context.Background()
					if err := app.RedisStream.PublishMessage(ctx, sessionID, "permission_notification", notifMsg); err != nil {
						slog.Warn("Failed to publish permission notification to Redis stream", "error", err)
					}
				}

				// Send via WebSocket if connected
				isConnected, _ := app.connectedSessions.Get(sessionID)
				if isConnected {
					app.WSServer.SendToSession(sessionID, notifMsg)
				}
			}

			// Send session updates to specific session via WebSocket (like TUI does)
			if event, ok := msg.(pubsub.Event[session.Session]); ok {
				if event.Type == pubsub.UpdatedEvent {
					sessionID := event.Payload.ID
					slog.Info("Session updated event received", "session_id", sessionID, "prompt_tokens", event.Payload.PromptTokens, "completion_tokens", event.Payload.CompletionTokens, "cost", event.Payload.Cost)

					// Get context window for this session
					ctx := context.Background()
					contextWindow := app.getSessionContextWindow(ctx, sessionID)

					slog.Info("Sending session update to WebSocket clients", "session_id", sessionID, "context_window", contextWindow, "total_tokens", event.Payload.PromptTokens+event.Payload.CompletionTokens)

					sessionMsg := map[string]interface{}{
						"Type":              "session_update",
						"id":                sessionID,
						"project_id":        event.Payload.ProjectID,
						"title":             event.Payload.Title,
						"message_count":     event.Payload.MessageCount,
						"prompt_tokens":     event.Payload.PromptTokens,
						"completion_tokens": event.Payload.CompletionTokens,
						"cost":              event.Payload.Cost,
						"context_window":    contextWindow,
						"created_at":        event.Payload.CreatedAt,
						"updated_at":        event.Payload.UpdatedAt,
					}

					// Publish to Redis
					if app.RedisStream != nil {
						if err := app.RedisStream.PublishMessage(ctx, sessionID, "session_update", sessionMsg); err != nil {
							slog.Warn("Failed to publish session update to Redis stream", "error", err)
						}
					}

					// Send via WebSocket if connected
					isConnected, _ := app.connectedSessions.Get(sessionID)
					if isConnected {
						app.WSServer.SendToSession(sessionID, sessionMsg)
					}
				}
			}
		}
	}
}

// Shutdown performs a graceful shutdown of the application.
func (app *App) Shutdown() {
	if app.AgentCoordinator != nil {
		app.AgentCoordinator.CancelAll()
	}

	// Kill all background shells.
	shell.GetBackgroundShellManager().KillAll()

	// Shutdown all LSP clients.
	for name, client := range app.LSPClients.Seq2() {
		shutdownCtx, cancel := context.WithTimeout(app.globalCtx, 5*time.Second)
		if err := client.Close(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown LSP client", "name", name, "error", err)
		}
		cancel()
	}

	// Call call cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			if err := cleanup(); err != nil {
				slog.Error("Failed to cleanup app properly on shutdown", "error", err)
			}
		}
	}
}

// checkForUpdates checks for available updates.
func (app *App) checkForUpdates(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info, err := update.Check(checkCtx, version.Version, update.Default)
	if err != nil || !info.Available() {
		return
	}
	app.events <- pubsub.UpdateAvailableMsg{
		CurrentVersion: info.Current,
		LatestVersion:  info.Latest,
		IsDevelopment:  info.IsDevelopment(),
	}
}

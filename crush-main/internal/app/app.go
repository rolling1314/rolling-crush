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
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/httpserver"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/project"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/storage"
	"github.com/charmbracelet/crush/internal/term"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/update"
	"github.com/charmbracelet/crush/internal/user"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service
	Users       user.Service
	Projects    project.Service

	AgentCoordinator agent.Coordinator

	LSPClients *csync.Map[string, *lsp.Client]

	config *config.Config
	db     *db.Queries // Add DB queries for session config loading

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	WSServer   *server.Server
	HTTPServer *httpserver.Server

	// Track the current active session for the single-user mode
	currentSessionID string

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func() error
}

// New initializes a new applcation instance.
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
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
		History:     files,
		Users:       users,
		Projects:    projects,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools),
		LSPClients:  csync.NewMap[string, *lsp.Client](),

		globalCtx: ctx,

		config: cfg,
		db:     q, // Store DB queries for session config loading

		events:          make(chan tea.Msg, 100),
		serviceEventsWG: &sync.WaitGroup{},
		tuiWG:           &sync.WaitGroup{},

		WSServer:   server.New(),
		HTTPServer: httpserver.New("8001", users, projects, sessions, messages, q, cfg),
	}

	// Register the handler for incoming WebSocket messages
	app.WSServer.SetMessageHandler(app.HandleClientMessage)
	fmt.Println("=== WebSocket message handler registered ===")
	fmt.Println("Handler function:", app.HandleClientMessage != nil)

	// Register disconnect handler to clean up agent state when WebSocket disconnects
	app.WSServer.SetDisconnectHandler(app.HandleClientDisconnect)

	app.setupEvents()

	// Initialize MinIO storage client
	if err := storage.InitGlobalMinIOClient(); err != nil {
		slog.Warn("Failed to initialize MinIO client, image upload will be unavailable", "error", err)
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

// HandleClientDisconnect handles WebSocket disconnection by cleaning up agent state
func (app *App) HandleClientDisconnect() {
	fmt.Println("=== HandleClientDisconnect called ===")
	slog.Info("WebSocket client disconnected, cleaning up agent state", "sessionID", app.currentSessionID)

	// Cancel the current session's agent request to prevent stuck session
	if app.AgentCoordinator != nil && app.currentSessionID != "" {
		fmt.Printf("Cancelling agent request for session: %s\n", app.currentSessionID)
		app.AgentCoordinator.Cancel(app.currentSessionID)
		fmt.Println("Agent request cancelled for current session")
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
		Images     []ImageAttachment `json:"images"` // Image attachments
	}

	var msg ClientMsg
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		slog.Error("Failed to unmarshal client message", "error", err)
		return
	}

	fmt.Println("Parsed message type:", msg.Type, "content:", msg.Content, "sessionID:", msg.SessionID)

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
		_, err := app.AgentCoordinator.Run(context.Background(), sessionID, msg.Content, attachments...)
		if err != nil {
			fmt.Println("Agent run error:", err)
			slog.Error("Agent run error", "error", err)
		} else {
			fmt.Println("Agent run completed successfully")
		}
	}()
	fmt.Println("Goroutine started, HandleClientMessage returning")
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
	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
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
				fmt.Printf("[SEND] Sending message to session: ID=%s, Role=%s, SessionID=%s\n", event.Payload.ID, event.Payload.Role, event.Payload.SessionID)
				app.WSServer.SendToSession(event.Payload.SessionID, event.Payload)
			}

			// Send permission requests to specific session via WebSocket
			if event, ok := msg.(pubsub.Event[permission.PermissionRequest]); ok {
				slog.Info("Sending permission request to session", "session_id", event.Payload.SessionID, "tool_call_id", event.Payload.ToolCallID)
				app.WSServer.SendToSession(event.Payload.SessionID, map[string]interface{}{
					"Type":         "permission_request",
					"id":           event.Payload.ID,
					"session_id":   event.Payload.SessionID,
					"tool_call_id": event.Payload.ToolCallID,
					"tool_name":    event.Payload.ToolName,
					"description":  event.Payload.Description,
					"action":       event.Payload.Action,
					"params":       event.Payload.Params,
					"path":         event.Payload.Path,
				})
			}

			// Send permission notifications to specific session via WebSocket
			if event, ok := msg.(pubsub.Event[permission.PermissionNotification]); ok {
				slog.Info("Sending permission notification to session", "session_id", event.Payload.SessionID, "tool_call_id", event.Payload.ToolCallID, "granted", event.Payload.Granted)
				app.WSServer.SendToSession(event.Payload.SessionID, map[string]interface{}{
					"Type":         "permission_notification",
					"tool_call_id": event.Payload.ToolCallID,
					"granted":      event.Payload.Granted,
					"denied":       event.Payload.Denied,
				})
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

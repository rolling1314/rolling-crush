// Package app provides application initialization for WebSocket services.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/rolling1314/rolling-crush/cmd/ws-server/handler"
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
	"github.com/rolling1314/rolling-crush/internal/pubsub"
	"github.com/rolling1314/rolling-crush/internal/shell"
	"github.com/rolling1314/rolling-crush/internal/update"
	"github.com/rolling1314/rolling-crush/internal/version"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// WSApp represents the WebSocket + Agent application instance.
// It contains all services required for WebSocket communication and agent processing.
type WSApp struct {
	Sessions    session.Service
	Messages    message.Service
	ToolCalls   toolcall.Service
	History     history.Service
	Permissions permission.Service
	Users       user.Service
	Projects    project.Service

	AgentCoordinator agent.Coordinator
	AgentWorkerPool  agent.AgentWorkerPool // Worker pool for concurrent agent tasks

	LSPClients *csync.Map[string, *lsp.Client]

	config *config.Config
	db     *postgres.Queries // DB queries for session config loading

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	WSServer *handler.Server

	// Redis stream service for message buffering during WebSocket disconnection
	RedisStream *storeredis.StreamService
	// Redis command service for tool call state management
	RedisCmd *storeredis.CommandService

	// Track the current active session for the single-user mode
	currentSessionID string

	// Track connected sessions (session ID -> connected status)
	connectedSessions *csync.Map[string, bool]

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func() error
}

// NewWSApp creates a new WebSocket + Agent application instance.
func NewWSApp(ctx context.Context, conn *sql.DB, cfg *config.Config) (*WSApp, error) {
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

	app := &WSApp{
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
		db:     q,

		events:            make(chan tea.Msg, 1000), // Increased buffer for streaming messages
		serviceEventsWG:   &sync.WaitGroup{},
		tuiWG:             &sync.WaitGroup{},
		connectedSessions: csync.NewMap[string, bool](),

		WSServer: handler.New(),
	}

	// Initialize Redis client and stream service
	if err := storeredis.InitGlobalClient(); err != nil {
		slog.Warn("Failed to initialize Redis client, message buffering will be unavailable", "error", err)
	} else {
		app.RedisStream = storeredis.GetGlobalStreamService()
		app.RedisCmd = storeredis.GetGlobalCommandService()
		slog.Info("Redis stream service initialized")

		// Set up allowlist checker for permission service using Redis adapter
		if app.RedisStream != nil {
			allowlistAdapter := storeredis.NewAllowlistAdapter(app.RedisStream)
			app.Permissions.SetAllowlistChecker(allowlistAdapter)
			slog.Info("Session allowlist checker configured with Redis backend")
		}
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
	if appCfg != nil && appCfg.Sandbox.BaseURL != "" {
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

	// Initialize the agent worker pool from app config
	if appCfg != nil {
		agentCfg := &appCfg.Agent
		// Create worker pool with a task executor that runs the agent
		executor := func(taskCtx context.Context, task agent.AgentTask) error {
			if app.AgentCoordinator == nil {
				return fmt.Errorf("agent coordinator not initialized")
			}
			_, err := app.AgentCoordinator.Run(taskCtx, task.SessionID, task.Prompt, task.Attachments...)
			return err
		}
		app.AgentWorkerPool = agent.NewAgentWorkerPool(agentCfg, executor)
		slog.Info("[GOROUTINE] Agent worker pool initialized",
			"max_workers", agentCfg.MaxWorkers,
			"queue_size", agentCfg.TaskQueueSize,
		)
	}

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

// Start starts the WebSocket server on the specified port.
func (app *WSApp) Start(port string) {
	slog.Info("Starting WebSocket server", "port", port)
	app.WSServer.Start(port)
}

// Config returns the application configuration.
func (app *WSApp) Config() *config.Config {
	return app.config
}

// Shutdown performs a graceful shutdown of the application.
func (app *WSApp) Shutdown() {
	slog.Info("[GOROUTINE] Starting graceful shutdown")

	// Shutdown the worker pool first (wait for running tasks to complete)
	if app.AgentWorkerPool != nil {
		slog.Info("[GOROUTINE] Shutting down agent worker pool")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := app.AgentWorkerPool.Shutdown(shutdownCtx); err != nil {
			slog.Warn("[GOROUTINE] Worker pool shutdown timeout", "error", err)
		}
		cancel()
		stats := app.AgentWorkerPool.Stats()
		slog.Info("[GOROUTINE] Worker pool shutdown complete",
			"completed_tasks", stats.CompletedTasks,
			"failed_tasks", stats.FailedTasks,
		)
	}

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

	// Call all cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			if err := cleanup(); err != nil {
				slog.Error("Failed to cleanup app properly on shutdown", "error", err)
			}
		}
	}

	slog.Info("[GOROUTINE] Graceful shutdown complete")
}

// checkForUpdates checks for available updates.
func (app *WSApp) checkForUpdates(ctx context.Context) {
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

// Package app provides application initialization for HTTP and WebSocket services.
package app

import (
	"context"
	"database/sql"
	"log/slog"

	apihttp "github.com/rolling1314/rolling-crush/api/http"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/project"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/user"
	"github.com/rolling1314/rolling-crush/pkg/config"
	"github.com/rolling1314/rolling-crush/sandbox"
	"github.com/rolling1314/rolling-crush/store/postgres"
	"github.com/rolling1314/rolling-crush/store/storage"
)

// HTTPApp represents the HTTP-only application instance.
// It contains only the services required for HTTP API operations.
type HTTPApp struct {
	Users    user.Service
	Projects project.Service
	Sessions session.Service
	Messages message.Service

	HTTPServer *apihttp.Server

	config *config.Config
	db     *sql.DB
}

// NewHTTPApp creates a new HTTP-only application instance.
func NewHTTPApp(ctx context.Context, conn *sql.DB, cfg *config.Config, port string) (*HTTPApp, error) {
	q := postgres.New(conn)

	users := user.NewService(q)
	projects := project.NewService(q)
	sessions := session.NewService(q)
	messages := message.NewService(q)

	app := &HTTPApp{
		Users:    users,
		Projects: projects,
		Sessions: sessions,
		Messages: messages,

		config: cfg,
		db:     conn,

		HTTPServer: apihttp.New(port, users, projects, sessions, messages, q, cfg),
	}

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

	return app, nil
}

// Start starts the HTTP server.
func (app *HTTPApp) Start() error {
	slog.Info("Starting HTTP API server")
	return app.HTTPServer.Start()
}

// Shutdown performs graceful shutdown of the HTTP application.
func (app *HTTPApp) Shutdown() {
	slog.Info("Shutting down HTTP application")
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			slog.Error("Failed to close database connection", "error", err)
		}
	}
}

// Config returns the application configuration.
func (app *HTTPApp) Config() *config.Config {
	return app.config
}

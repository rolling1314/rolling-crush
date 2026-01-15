// Package app provides shared initialization logic for HTTP and WebSocket services.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rolling1314/rolling-crush/domain/history"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/domain/project"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/user"
	"github.com/rolling1314/rolling-crush/infra/postgres"
	storeredis "github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/infra/sandbox"
	"github.com/rolling1314/rolling-crush/infra/storage"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// SharedServices contains services shared between HTTP and WebSocket apps
type SharedServices struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service
	Users       user.Service
	Projects    project.Service

	DB          *postgres.Queries
	DBConn      *sql.DB
	Config      *config.Config
	RedisStream *storeredis.StreamService
	RedisCmd    *storeredis.CommandService
}

// InitConfig initializes the application configuration
func InitConfig(cwd, dataDir string, debug bool) (*config.Config, error) {
	// Initialize application configuration (database, sandbox, storage)
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	slog.Info("Initializing application configuration", "environment", env)

	// Load app config to set global config
	appCfg, err := config.LoadAppConfig("", env)
	if err != nil {
		// If config file doesn't exist, log a warning and continue with defaults
		slog.Warn("Failed to load config.yaml, using default configuration", "error", err)
		appCfg = nil // Will use defaults
	}
	if appCfg != nil {
		config.SetGlobalAppConfig(appCfg)
		slog.Info("Application configuration loaded successfully",
			"db_host", appCfg.Database.Host,
			"sandbox_url", appCfg.Sandbox.BaseURL,
			"storage_type", appCfg.Storage.Type,
		)
	}

	cfg, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// InitSharedServices initializes all shared services
func InitSharedServices(ctx context.Context, cfg *config.Config) (*SharedServices, error) {
	// Connect to DB
	conn, err := postgres.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	q := postgres.New(conn)
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

	services := &SharedServices{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Users:       users,
		Projects:    projects,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools),
		DB:          q,
		DBConn:      conn,
		Config:      cfg,
	}

	// Initialize Redis client and services
	if err := storeredis.InitGlobalClient(); err != nil {
		slog.Warn("Failed to initialize Redis client, some features will be unavailable", "error", err)
	} else {
		services.RedisStream = storeredis.GetGlobalStreamService()
		services.RedisCmd = storeredis.GetGlobalCommandService()
		slog.Info("Redis services initialized")
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

	return services, nil
}

// Close closes all shared service connections
func (s *SharedServices) Close() error {
	if s.DBConn != nil {
		return s.DBConn.Close()
	}
	return nil
}

// GetCwd returns the current working directory or the provided cwd
func GetCwd(cwd string) (string, error) {
	if cwd != "" {
		if err := os.Chdir(cwd); err != nil {
			return "", fmt.Errorf("failed to change directory: %v", err)
		}
		return cwd, nil
	}
	return os.Getwd()
}

// CreateDataDir creates the .crush data directory
func CreateDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %q %w", dir, err)
	}

	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitIgnorePath, []byte("*\n"), 0o644); err != nil {
			return fmt.Errorf("failed to create .gitignore file: %q %w", gitIgnorePath, err)
		}
	}

	return nil
}

// ServiceConfig holds configuration for a specific service
type ServiceConfig struct {
	Port    string
	Debug   bool
	DataDir string
	Cwd     string
}

// DefaultHTTPPort is the default port for the HTTP service
const DefaultHTTPPort = "8001"

// DefaultWSPort is the default port for the WebSocket service
const DefaultWSPort = "8002"

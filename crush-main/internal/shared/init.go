// Package shared provides common initialization logic for both HTTP and WebSocket services.
package shared

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rolling1314/rolling-crush/pkg/config"
	"github.com/rolling1314/rolling-crush/infra/postgres"
)

// InitOptions contains options for initialization.
type InitOptions struct {
	WorkingDir string
	DataDir    string
	Debug      bool
	Yolo       bool // Skip permission requests
}

// InitResult contains the result of initialization.
type InitResult struct {
	Config   *config.Config
	AppCfg   *config.AppConfig
	DB       *sql.DB
	Queries  *postgres.Queries
}

// Initialize performs common initialization for both services.
// It loads configuration, connects to database, and returns all necessary components.
func Initialize(ctx context.Context, opts InitOptions) (*InitResult, error) {
	// Resolve working directory
	cwd, err := ResolveCwd(opts.WorkingDir)
	if err != nil {
		return nil, err
	}

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

	// Initialize crush config
	cfg, err := config.Init(cwd, opts.DataDir, opts.Debug)
	if err != nil {
		return nil, err
	}

	// Set permission options
	if cfg.Permissions == nil {
		cfg.Permissions = &config.Permissions{}
	}
	cfg.Permissions.SkipRequests = opts.Yolo

	// Create data directory
	if err := CreateDotCrushDir(cfg.Options.DataDirectory); err != nil {
		return nil, err
	}

	// Connect to database; this will also run migrations
	conn, err := postgres.Connect(ctx, cfg.Options.DataDirectory)
	if err != nil {
		return nil, err
	}

	return &InitResult{
		Config:  cfg,
		AppCfg:  appCfg,
		DB:      conn,
		Queries: postgres.New(conn),
	}, nil
}

// ResolveCwd resolves the working directory.
func ResolveCwd(cwd string) (string, error) {
	if cwd != "" {
		err := os.Chdir(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to change directory: %v", err)
		}
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}
	return cwd, nil
}

// CreateDotCrushDir creates the .crush data directory.
func CreateDotCrushDir(dir string) error {
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

// ServerConfig contains server configuration.
type ServerConfig struct {
	HTTPPort string
	WSPort   string
	Debug    bool
}

// GetServerConfig returns server configuration from config.yaml.
// Falls back to defaults if config is not loaded.
func GetServerConfig() ServerConfig {
	appCfg := config.GetGlobalAppConfig()

	// Default values
	httpPort := "8001"
	wsPort := "8002"
	debug := false

	if appCfg != nil {
		if appCfg.Server.HTTPPort != "" {
			httpPort = appCfg.Server.HTTPPort
		}
		if appCfg.Server.WSPort != "" {
			wsPort = appCfg.Server.WSPort
		}
		debug = appCfg.Server.Debug
	}

	slog.Info("Server configuration loaded",
		"http_port", httpPort,
		"ws_port", wsPort,
		"debug", debug,
	)

	return ServerConfig{
		HTTPPort: httpPort,
		WSPort:   wsPort,
		Debug:    debug,
	}
}

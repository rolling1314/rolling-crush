package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rolling1314/rolling-crush/internal/app"
	"github.com/rolling1314/rolling-crush/internal/shared"
)

func main() {
	// Start pprof server if CRUSH_PROFILE is set
	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	fmt.Println()
	slog.Info("Starting Crush HTTP API Server")

	ctx := context.Background()

	// Get working directory from environment or use current directory
	cwd := os.Getenv("CRUSH_CWD")
	dataDir := os.Getenv("CRUSH_DATA_DIR")
	debug := os.Getenv("CRUSH_DEBUG") == "true"

	// Initialize shared components
	initResult, err := shared.Initialize(ctx, shared.InitOptions{
		WorkingDir: cwd,
		DataDir:    dataDir,
		Debug:      debug,
		Yolo:       false,
	})
	if err != nil {
		slog.Error("Failed to initialize", "error", err)
		os.Exit(1)
	}

	// Get server configuration
	serverCfg := shared.GetServerConfig()

	// Create HTTP application
	httpApp, err := app.NewHTTPApp(ctx, initResult.DB, initResult.Config, serverCfg.HTTPPort)
	if err != nil {
		slog.Error("Failed to create HTTP app", "error", err)
		os.Exit(1)
	}
	defer httpApp.Shutdown()

	// Start HTTP server in a goroutine
	go func() {
		slog.Info("HTTP Server starting", "port", serverCfg.HTTPPort)
		slog.Info("HTTP Server URL", "url", fmt.Sprintf("http://localhost:%s", serverCfg.HTTPPort))
		if err := httpApp.Start(); err != nil {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Crush HTTP API Server is running")
	slog.Info("Press Ctrl+C to stop.")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down HTTP server...")
}

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
	"github.com/rolling1314/rolling-crush/internal/event"
	"github.com/rolling1314/rolling-crush/internal/shared"
)

func main() {
	// Start pprof server if CRUSH_PROFILE is set
	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6061")
			if httpErr := http.ListenAndServe("localhost:6061", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	fmt.Println()
	slog.Info("Starting Crush WebSocket + Agent Server")

	ctx := context.Background()

	// Get working directory from environment or use current directory
	cwd := os.Getenv("CRUSH_CWD")
	dataDir := os.Getenv("CRUSH_DATA_DIR")
	debug := os.Getenv("CRUSH_DEBUG") == "true"
	yolo := os.Getenv("CRUSH_YOLO") == "true"

	// Initialize shared components
	initResult, err := shared.Initialize(ctx, shared.InitOptions{
		WorkingDir: cwd,
		DataDir:    dataDir,
		Debug:      debug,
		Yolo:       yolo,
	})
	if err != nil {
		slog.Error("Failed to initialize", "error", err)
		os.Exit(1)
	}

	// Get server configuration
	serverCfg := shared.GetServerConfig()

	// Create WebSocket application
	wsApp, err := app.NewWSApp(ctx, initResult.DB, initResult.Config)
	if err != nil {
		slog.Error("Failed to create WebSocket app", "error", err)
		os.Exit(1)
	}
	defer wsApp.Shutdown()

	// Initialize event tracking if metrics are enabled
	if shouldEnableMetrics() {
		event.Init()
	}
	event.AppInitialized()

	// Start background subscription (handles event processing and WebSocket broadcasting)
	go wsApp.Subscribe()

	// Start WebSocket server in a goroutine
	go func() {
		slog.Info("WebSocket Server starting", "port", serverCfg.WSPort)
		slog.Info("WebSocket Server URL", "url", fmt.Sprintf("ws://localhost:%s/ws", serverCfg.WSPort))
		wsApp.Start(serverCfg.WSPort)
	}()

	slog.Info("Crush WebSocket + Agent Server is running")
	slog.Info("Press Ctrl+C to stop.")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down WebSocket server...")
	event.AppExited()
}

// shouldEnableMetrics checks if metrics should be enabled based on environment variables and config
func shouldEnableMetrics() bool {
	if os.Getenv("CRUSH_DISABLE_METRICS") == "true" {
		return false
	}
	if os.Getenv("DO_NOT_TRACK") == "true" {
		return false
	}
	return true
}

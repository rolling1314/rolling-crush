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

	// Initialize shared components (loads config.yaml)
	initResult, err := shared.Initialize(ctx, shared.InitOptions{
		WorkingDir: os.Getenv("CRUSH_CWD"),      // Optional: override working directory
		DataDir:    os.Getenv("CRUSH_DATA_DIR"), // Optional: override data directory
		Debug:      false,
		Yolo:       os.Getenv("CRUSH_YOLO") == "true", // Skip permission requests
	})
	if err != nil {
		slog.Error("Failed to initialize", "error", err)
		os.Exit(1)
	}

	// Get server configuration from config.yaml
	serverCfg := shared.GetServerConfig()

	// Create WebSocket application
	wsApp, err := app.NewWSApp(ctx, initResult.DB, initResult.Config)
	if err != nil {
		slog.Error("Failed to create WebSocket app", "error", err)
		os.Exit(1)
	}
	defer wsApp.Shutdown()

	// Initialize event tracking if metrics are enabled
	if !initResult.Config.Options.DisableMetrics {
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

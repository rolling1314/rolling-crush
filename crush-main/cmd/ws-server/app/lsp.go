package app

import (
	"context"
	"log/slog"
	"time"

	internalapp "github.com/rolling1314/rolling-crush/internal/app"
	"github.com/rolling1314/rolling-crush/internal/lsp"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// initLSPClients initializes LSP clients in the background.
func (app *WSApp) initLSPClients(ctx context.Context) {
	for name, clientConfig := range app.config.LSP {
		if clientConfig.Disabled {
			slog.Info("Skipping disabled LSP client", "name", name)
			continue
		}
		go app.createAndStartLSPClient(ctx, name, clientConfig)
	}
	slog.Info("LSP clients initialization started in background")
}

// createAndStartLSPClient creates a new LSP client, initializes it, and starts its workspace watcher
func (app *WSApp) createAndStartLSPClient(ctx context.Context, name string, lspConfig config.LSPConfig) {
	slog.Info("Creating LSP client", "name", name, "command", lspConfig.Command, "fileTypes", lspConfig.FileTypes, "args", lspConfig.Args)

	// Check if any root markers exist in the working directory
	if !lsp.HasRootMarkers(app.config.WorkingDir(), lspConfig.RootMarkers) {
		slog.Info("Skipping LSP client - no root markers found", "name", name, "rootMarkers", lspConfig.RootMarkers)
		internalapp.UpdateLSPState(name, lsp.StateDisabled, nil, nil, 0)
		return
	}

	// Update state to starting
	internalapp.UpdateLSPState(name, lsp.StateStarting, nil, nil, 0)

	// Create LSP client.
	lspClient, err := lsp.New(ctx, name, lspConfig, app.config.Resolver())
	if err != nil {
		slog.Error("Failed to create LSP client for", name, err)
		internalapp.UpdateLSPState(name, lsp.StateError, err, nil, 0)
		return
	}

	// Set diagnostics callback
	lspClient.SetDiagnosticsCallback(internalapp.UpdateLSPDiagnostics)

	// Increase initialization timeout as some servers take more time to start.
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Initialize LSP client.
	_, err = lspClient.Initialize(initCtx, app.config.WorkingDir())
	if err != nil {
		slog.Error("Initialize failed", "name", name, "error", err)
		internalapp.UpdateLSPState(name, lsp.StateError, err, lspClient, 0)
		lspClient.Close(ctx)
		return
	}

	// Wait for the server to be ready.
	if err := lspClient.WaitForServerReady(initCtx); err != nil {
		slog.Error("Server failed to become ready", "name", name, "error", err)
		lspClient.SetServerState(lsp.StateError)
		internalapp.UpdateLSPState(name, lsp.StateError, err, lspClient, 0)
	} else {
		slog.Info("LSP server is ready", "name", name)
		lspClient.SetServerState(lsp.StateReady)
		internalapp.UpdateLSPState(name, lsp.StateReady, nil, lspClient, 0)
	}

	slog.Info("LSP client initialized", "name", name)

	// Add to map with mutex protection
	app.LSPClients.Set(name, lspClient)
}

package tools

import (
	"context"
	"log/slog"
	"time"

	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

// DefaultPermissionTimeout is the default timeout for permission requests (5 minutes)
const DefaultPermissionTimeout = 5 * time.Minute

// GetPermissionTimeout returns the configured permission timeout duration
func GetPermissionTimeout() time.Duration {
	appCfg := config.GetGlobalAppConfig()
	if appCfg != nil && appCfg.Agent.PermissionTimeout > 0 {
		return time.Duration(appCfg.Agent.PermissionTimeout) * time.Second
	}
	return DefaultPermissionTimeout
}

// RequestPermissionWithTimeout wraps the permission request with timeout support.
// It returns (granted, error) where error is:
// - nil if granted
// - permission.ErrorPermissionDenied if denied
// - permission.ErrorPermissionTimeout if timeout
// - ctx.Err() if context cancelled
func RequestPermissionWithTimeout(
	ctx context.Context,
	permissions permission.Service,
	opts permission.CreatePermissionRequest,
	originalPrompt string,
) (bool, error) {
	timeout := GetPermissionTimeout()

	// Callback for when permission times out - log the event
	// The actual DB persistence is handled by the coordinator/app layer
	onTimeout := func(req permission.PermissionRequest, prompt string) {
		slog.Warn("[PERMISSION] Permission request timed out, tool call suspended",
			"tool_name", req.ToolName,
			"tool_call_id", req.ToolCallID,
			"session_id", req.SessionID,
			"timeout", timeout,
		)
	}

	return permissions.RequestWithTimeout(ctx, opts, timeout, originalPrompt, onTimeout)
}

// RequestPermissionWithTimeoutSimple is a simplified version that doesn't track original prompt.
// Use this when you don't need resume capability.
func RequestPermissionWithTimeoutSimple(
	ctx context.Context,
	permissions permission.Service,
	opts permission.CreatePermissionRequest,
) (bool, error) {
	return RequestPermissionWithTimeout(ctx, permissions, opts, "")
}

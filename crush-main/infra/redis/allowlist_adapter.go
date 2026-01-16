// Package redis provides the allowlist adapter for permission service integration.
package redis

import (
	"context"

	"github.com/rolling1314/rolling-crush/domain/permission"
)

// AllowlistAdapter adapts StreamService to implement permission.AllowlistChecker.
type AllowlistAdapter struct {
	stream *StreamService
}

// NewAllowlistAdapter creates a new adapter for the stream service.
func NewAllowlistAdapter(stream *StreamService) *AllowlistAdapter {
	return &AllowlistAdapter{stream: stream}
}

// IsToolAllowedInSession checks if a tool is allowed in the session's allowlist.
func (a *AllowlistAdapter) IsToolAllowedInSession(ctx context.Context, sessionID, toolName, action, path string) (bool, error) {
	return a.stream.IsToolAllowedInSession(ctx, sessionID, toolName, action, path)
}

// AddToSessionAllowlist adds a tool to the session's allowlist.
func (a *AllowlistAdapter) AddToSessionAllowlist(ctx context.Context, sessionID string, entry permission.AllowlistEntry) error {
	redisEntry := ToolAllowlistEntry{
		ToolName: entry.ToolName,
		Action:   entry.Action,
		Path:     entry.Path,
		AddedAt:  entry.AddedAt,
	}
	return a.stream.AddToSessionAllowlist(ctx, sessionID, redisEntry)
}

// Ensure AllowlistAdapter implements permission.AllowlistChecker
var _ permission.AllowlistChecker = (*AllowlistAdapter)(nil)

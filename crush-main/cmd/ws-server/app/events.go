package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/rolling1314/rolling-crush/domain/message"
	"github.com/rolling1314/rolling-crush/domain/permission"
	"github.com/rolling1314/rolling-crush/domain/session"
	"github.com/rolling1314/rolling-crush/domain/toolcall"
	storeredis "github.com/rolling1314/rolling-crush/infra/redis"
	"github.com/rolling1314/rolling-crush/internal/agent/tools/mcp"
	"github.com/rolling1314/rolling-crush/internal/pkg/log"
	"github.com/rolling1314/rolling-crush/internal/pubsub"
)

func (app *WSApp) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	wsSetupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "toolcalls", app.ToolCalls.Subscribe, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "mcp", mcp.SubscribeEvents, app.events)
	wsSetupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events)
	cleanupFunc := func() error {
		cancel()
		app.serviceEventsWG.Wait()
		return nil
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

func wsSetupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
) {
	wg.Go(func() {
		subCh := subscriber(ctx)
		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("subscription channel closed", "name", name)
					return
				}
				var msg tea.Msg = event
				select {
				case outputCh <- msg:
				case <-time.After(2 * time.Second):
					slog.Warn("message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					slog.Debug("subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				slog.Debug("subscription cancelled", "name", name)
				return
			}
		}
	})
}

// Subscribe handles event processing and broadcasting.
func (app *WSApp) Subscribe() {
	fmt.Println("=== Subscribe() started - listening for events ===")
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("Subscription panic: attempting graceful shutdown")
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func() error {
		slog.Debug("Cancelling message handler")
		tuiCancel()
		app.tuiWG.Wait()
		return nil
	})
	defer app.tuiWG.Done()

	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("Message handler shutting down")
			return
		case msg, ok := <-app.events:
			if !ok {
				slog.Debug("Message channel closed")
				return
			}

			app.handleEvent(msg)
		}
	}
}

// handleEvent processes a single event from the events channel
func (app *WSApp) handleEvent(msg tea.Msg) {
	// DEBUG: 打印收到的事件类型
	fmt.Printf("[EVENT] Received event type: %T\n", msg)

	// Send messages to specific session via WebSocket
	if event, ok := msg.(pubsub.Event[message.Message]); ok {
		app.handleMessageEvent(event)
	}

	// Send tool call updates to specific session via WebSocket
	if event, ok := msg.(pubsub.Event[toolcall.ToolCall]); ok {
		app.handleToolCallEvent(event)
	}

	// Send permission requests to specific session via WebSocket
	if event, ok := msg.(pubsub.Event[permission.PermissionRequest]); ok {
		app.handlePermissionRequestEvent(event)
	}

	// Send permission notifications to specific session via WebSocket
	if event, ok := msg.(pubsub.Event[permission.PermissionNotification]); ok {
		app.handlePermissionNotificationEvent(event)
	}

	// Send session updates to specific session via WebSocket (like TUI does)
	if event, ok := msg.(pubsub.Event[session.Session]); ok {
		app.handleSessionEvent(event)
	}
}

// handleMessageEvent handles message events
func (app *WSApp) handleMessageEvent(event pubsub.Event[message.Message]) {
	sessionID := event.Payload.SessionID
	fmt.Printf("[SEND] Sending message to session: ID=%s, Role=%s, SessionID=%s\n", event.Payload.ID, event.Payload.Role, sessionID)

	// Always publish to Redis stream for buffering
	if app.RedisStream != nil {
		ctx := context.Background()
		if err := app.RedisStream.PublishMessage(ctx, sessionID, "message", event.Payload); err != nil {
			slog.Warn("Failed to publish message to Redis stream", "error", err)
		}
	}

	// Check if session is connected before sending via WebSocket
	isConnected, _ := app.connectedSessions.Get(sessionID)
	if isConnected {
		app.WSServer.SendToSession(sessionID, event.Payload)
	} else {
		slog.Debug("Session disconnected, message buffered in Redis", "sessionID", sessionID)
	}
}

// handlePermissionRequestEvent handles permission request events
func (app *WSApp) handlePermissionRequestEvent(event pubsub.Event[permission.PermissionRequest]) {
	sessionID := event.Payload.SessionID
	slog.Info("Sending permission request to session", "session_id", sessionID, "tool_call_id", event.Payload.ToolCallID)

	permMsg := map[string]interface{}{
		"Type":         "permission_request",
		"id":           event.Payload.ID,
		"session_id":   sessionID,
		"tool_call_id": event.Payload.ToolCallID,
		"tool_name":    event.Payload.ToolName,
		"description":  event.Payload.Description,
		"action":       event.Payload.Action,
		"params":       event.Payload.Params,
		"path":         event.Payload.Path,
	}

	// Store pending permission in Redis (separate from stream)
	// This allows proper state management for reconnection
	if app.RedisStream != nil {
		ctx := context.Background()
		perm := storeredis.PendingPermission{
			ID:          event.Payload.ID,
			SessionID:   sessionID,
			ToolCallID:  event.Payload.ToolCallID,
			ToolName:    event.Payload.ToolName,
			Description: event.Payload.Description,
			Action:      event.Payload.Action,
			Params:      event.Payload.Params,
			Path:        event.Payload.Path,
		}
		if err := app.RedisStream.SetPendingPermission(ctx, perm); err != nil {
			slog.Warn("Failed to store pending permission in Redis", "error", err)
		}
	}

	// Note: We don't publish permission_request to Redis Stream anymore
	// because it's transient state that should be managed separately.
	// On reconnection, we'll check the pending permissions directly.

	// Send via WebSocket if connected
	isConnected, _ := app.connectedSessions.Get(sessionID)
	if isConnected {
		app.WSServer.SendToSession(sessionID, permMsg)
	}
}

// handlePermissionNotificationEvent handles permission notification events
func (app *WSApp) handlePermissionNotificationEvent(event pubsub.Event[permission.PermissionNotification]) {
	sessionID := event.Payload.SessionID
	slog.Info("Sending permission notification to session", "session_id", sessionID, "tool_call_id", event.Payload.ToolCallID, "granted", event.Payload.Granted)

	notifMsg := map[string]interface{}{
		"Type":         "permission_notification",
		"tool_call_id": event.Payload.ToolCallID,
		"granted":      event.Payload.Granted,
		"denied":       event.Payload.Denied,
	}

	// Update permission status in Redis
	if app.RedisStream != nil {
		ctx := context.Background()
		status := "pending"
		if event.Payload.Granted {
			status = "granted"
		} else if event.Payload.Denied {
			status = "denied"
		}
		// Only update if it's a final status (granted or denied)
		if status != "pending" {
			if err := app.RedisStream.UpdatePermissionStatus(ctx, sessionID, event.Payload.ToolCallID, status); err != nil {
				slog.Warn("Failed to update permission status in Redis", "error", err)
			}
		}
	}

	// Note: We don't publish permission_notification to Redis Stream anymore
	// The permission state is managed separately.

	// Send via WebSocket if connected
	isConnected, _ := app.connectedSessions.Get(sessionID)
	if isConnected {
		app.WSServer.SendToSession(sessionID, notifMsg)
	}
}

// handleToolCallEvent handles tool call update events
func (app *WSApp) handleToolCallEvent(event pubsub.Event[toolcall.ToolCall]) {
	sessionID := event.Payload.SessionID
	slog.Info("Tool call event received",
		"event_type", event.Type,
		"session_id", sessionID,
		"tool_call_id", event.Payload.ID,
		"name", event.Payload.Name,
		"status", event.Payload.Status,
	)

	toolCallMsg := map[string]interface{}{
		"Type":          "tool_call_update",
		"id":            event.Payload.ID,
		"session_id":    sessionID,
		"message_id":    event.Payload.MessageID,
		"name":          event.Payload.Name,
		"input":         event.Payload.Input,
		"status":        string(event.Payload.Status),
		"result":        event.Payload.Result,
		"is_error":      event.Payload.IsError,
		"error_message": event.Payload.ErrorMessage,
		"created_at":    event.Payload.CreatedAt,
		"updated_at":    event.Payload.UpdatedAt,
	}
	if event.Payload.StartedAt != nil {
		toolCallMsg["started_at"] = *event.Payload.StartedAt
	}
	if event.Payload.FinishedAt != nil {
		toolCallMsg["finished_at"] = *event.Payload.FinishedAt
	}

	// Publish to Redis for buffering
	if app.RedisStream != nil {
		ctx := context.Background()
		if err := app.RedisStream.PublishMessage(ctx, sessionID, "tool_call_update", toolCallMsg); err != nil {
			slog.Warn("Failed to publish tool call update to Redis stream", "error", err)
		}
	}

	// Send via WebSocket if connected
	isConnected, _ := app.connectedSessions.Get(sessionID)
	if isConnected {
		app.WSServer.SendToSession(sessionID, toolCallMsg)
	}
}

// handleSessionEvent handles session update events
func (app *WSApp) handleSessionEvent(event pubsub.Event[session.Session]) {
	if event.Type != pubsub.UpdatedEvent {
		return
	}

	sessionID := event.Payload.ID
	slog.Info("Session updated event received", "session_id", sessionID, "prompt_tokens", event.Payload.PromptTokens, "completion_tokens", event.Payload.CompletionTokens, "cost", event.Payload.Cost)

	// Get context window for this session
	ctx := context.Background()
	contextWindow := app.getSessionContextWindow(ctx, sessionID)

	slog.Info("Sending session update to WebSocket clients", "session_id", sessionID, "context_window", contextWindow, "total_tokens", event.Payload.PromptTokens+event.Payload.CompletionTokens)

	sessionMsg := map[string]interface{}{
		"Type":              "session_update",
		"id":                sessionID,
		"project_id":        event.Payload.ProjectID,
		"title":             event.Payload.Title,
		"message_count":     event.Payload.MessageCount,
		"prompt_tokens":     event.Payload.PromptTokens,
		"completion_tokens": event.Payload.CompletionTokens,
		"cost":              event.Payload.Cost,
		"context_window":    contextWindow,
		"created_at":        event.Payload.CreatedAt,
		"updated_at":        event.Payload.UpdatedAt,
	}

	// Publish to Redis
	if app.RedisStream != nil {
		if err := app.RedisStream.PublishMessage(ctx, sessionID, "session_update", sessionMsg); err != nil {
			slog.Warn("Failed to publish session update to Redis stream", "error", err)
		}
	}

	// Send via WebSocket if connected
	isConnected, _ := app.connectedSessions.Get(sessionID)
	if isConnected {
		app.WSServer.SendToSession(sessionID, sessionMsg)
	}
}

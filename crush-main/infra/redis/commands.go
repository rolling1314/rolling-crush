// Package redis provides Redis operations for inter-service communication.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// CommandChannelPrefix is the prefix for inter-service command channels
	CommandChannelPrefix = "crush:cmd:"
	// GlobalCommandChannel is the channel for broadcasting commands to all WS instances
	GlobalCommandChannel = "crush:cmd:global"
	// SessionCommandChannel is the prefix for session-specific commands
	SessionCommandChannelPrefix = "crush:cmd:session:"
)

// CommandType defines the type of inter-service command
type CommandType string

const (
	// CmdCancel cancels an ongoing agent request
	CmdCancel CommandType = "cancel"
	// CmdPermissionResponse sends permission response from HTTP to WS
	CmdPermissionResponse CommandType = "permission_response"
	// CmdSessionUpdate notifies about session updates
	CmdSessionUpdate CommandType = "session_update"
	// CmdClientMessage forwards client message to WS service
	CmdClientMessage CommandType = "client_message"
)

// Command represents an inter-service command
type Command struct {
	Type      CommandType     `json:"type"`
	SessionID string          `json:"session_id"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
	Source    string          `json:"source"` // "http" or "ws"
}

// CancelPayload is the payload for cancel commands
type CancelPayload struct {
	Reason string `json:"reason,omitempty"`
}

// PermissionResponsePayload is the payload for permission response commands
type PermissionResponsePayload struct {
	ID         string `json:"id"`
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

// ClientMessagePayload is the payload for forwarded client messages
type ClientMessagePayload struct {
	Type       string          `json:"type"`
	Content    string          `json:"content"`
	SessionID  string          `json:"sessionID"`
	ID         string          `json:"id"`
	ToolCallID string          `json:"tool_call_id"`
	Granted    bool            `json:"granted"`
	Denied     bool            `json:"denied"`
	Images     json.RawMessage `json:"images,omitempty"`
	LastMsgID  string          `json:"lastMsgId,omitempty"`
}

// CommandService provides Redis pub/sub operations for inter-service communication.
type CommandService struct {
	client *Client
}

// NewCommandService creates a new command service.
func NewCommandService(client *Client) *CommandService {
	return &CommandService{client: client}
}

// GetGlobalCommandService returns a command service using the global client.
func GetGlobalCommandService() *CommandService {
	client := GetClient()
	if client == nil {
		return nil
	}
	return NewCommandService(client)
}

// sessionCommandChannel returns the Redis channel for session-specific commands.
func (s *CommandService) sessionCommandChannel(sessionID string) string {
	return SessionCommandChannelPrefix + sessionID
}

// PublishCommand publishes a command to the appropriate channel.
func (s *CommandService) PublishCommand(ctx context.Context, cmd Command) error {
	cmd.Timestamp = time.Now().UnixMilli()

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Publish to session-specific channel if sessionID is provided
	channel := GlobalCommandChannel
	if cmd.SessionID != "" {
		channel = s.sessionCommandChannel(cmd.SessionID)
	}

	err = s.client.rdb.Publish(ctx, channel, string(cmdJSON)).Err()
	if err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	slog.Debug("Published command",
		"type", cmd.Type,
		"session_id", cmd.SessionID,
		"channel", channel,
	)

	return nil
}

// PublishCancelCommand publishes a cancel command for a session.
func (s *CommandService) PublishCancelCommand(ctx context.Context, sessionID string, reason string) error {
	payload, _ := json.Marshal(CancelPayload{Reason: reason})
	return s.PublishCommand(ctx, Command{
		Type:      CmdCancel,
		SessionID: sessionID,
		Payload:   payload,
		Source:    "http",
	})
}

// PublishPermissionResponse publishes a permission response command.
func (s *CommandService) PublishPermissionResponse(ctx context.Context, sessionID string, resp PermissionResponsePayload) error {
	payload, _ := json.Marshal(resp)
	return s.PublishCommand(ctx, Command{
		Type:      CmdPermissionResponse,
		SessionID: sessionID,
		Payload:   payload,
		Source:    "http",
	})
}

// PublishClientMessage publishes a client message to be processed by WS service.
func (s *CommandService) PublishClientMessage(ctx context.Context, sessionID string, msg ClientMessagePayload) error {
	payload, _ := json.Marshal(msg)
	return s.PublishCommand(ctx, Command{
		Type:      CmdClientMessage,
		SessionID: sessionID,
		Payload:   payload,
		Source:    "http",
	})
}

// CommandHandler is a callback function for handling received commands
type CommandHandler func(cmd Command)

// SubscribeCommands subscribes to commands for specific sessions and/or global channel.
// It returns a channel that will receive commands and a cancel function.
func (s *CommandService) SubscribeCommands(ctx context.Context, sessionIDs []string, includeGlobal bool) (<-chan Command, func()) {
	cmdChan := make(chan Command, 100)

	// Build list of channels to subscribe
	channels := make([]string, 0, len(sessionIDs)+1)
	if includeGlobal {
		channels = append(channels, GlobalCommandChannel)
	}
	for _, sid := range sessionIDs {
		channels = append(channels, s.sessionCommandChannel(sid))
	}

	// Create pattern subscription for all session commands if no specific sessions
	var pubsub *redis.PubSub
	if len(sessionIDs) == 0 && includeGlobal {
		// Subscribe to global channel and use pattern for session channels
		pubsub = s.client.rdb.PSubscribe(ctx, GlobalCommandChannel, SessionCommandChannelPrefix+"*")
	} else if len(channels) > 0 {
		pubsub = s.client.rdb.Subscribe(ctx, channels...)
	} else {
		close(cmdChan)
		return cmdChan, func() {}
	}

	subCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(cmdChan)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				var cmd Command
				if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
					slog.Warn("Failed to unmarshal command", "error", err, "payload", msg.Payload)
					continue
				}

				select {
				case cmdChan <- cmd:
				case <-subCtx.Done():
					return
				default:
					slog.Warn("Command channel full, dropping command", "type", cmd.Type)
				}
			}
		}
	}()

	return cmdChan, cancel
}

// SubscribeSessionCommands subscribes to commands for a specific session.
func (s *CommandService) SubscribeSessionCommands(ctx context.Context, sessionID string) (<-chan Command, func()) {
	return s.SubscribeCommands(ctx, []string{sessionID}, false)
}

// SubscribeAllCommands subscribes to all commands (global + all sessions).
func (s *CommandService) SubscribeAllCommands(ctx context.Context) (<-chan Command, func()) {
	return s.SubscribeCommands(ctx, nil, true)
}

// AddSessionSubscription dynamically adds a session to an existing subscription.
// Note: This requires creating a new subscription as Redis doesn't support dynamic subscribe in Go client easily.
func (s *CommandService) AddSessionSubscription(ctx context.Context, sessionID string) error {
	// This is a placeholder - in practice, the WS service will manage session subscriptions
	// by tracking connected sessions and using pattern subscription
	slog.Debug("Session subscription added", "session_id", sessionID)
	return nil
}

// RemoveSessionSubscription removes a session from subscription.
func (s *CommandService) RemoveSessionSubscription(ctx context.Context, sessionID string) error {
	slog.Debug("Session subscription removed", "session_id", sessionID)
	return nil
}

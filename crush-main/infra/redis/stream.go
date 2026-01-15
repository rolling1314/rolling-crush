// Package redis provides Redis stream operations for message buffering.
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
	// StreamKeyPrefix is the prefix for session message streams
	StreamKeyPrefix = "crush:stream:session:"
	// ConnectionKeyPrefix is the prefix for tracking session connections
	ConnectionKeyPrefix = "crush:conn:session:"
	// LastReadKeyPrefix is the prefix for tracking last read message ID
	LastReadKeyPrefix = "crush:lastread:session:"
	// ActiveGenerationKeyPrefix tracks if a generation is still active
	ActiveGenerationKeyPrefix = "crush:active:session:"
)

// StreamMessage represents a message stored in Redis stream.
type StreamMessage struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	Type      string          `json:"type"` // "message", "session_update", "permission_request", etc.
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// StreamService provides Redis stream operations for message buffering.
type StreamService struct {
	client *Client
}

// NewStreamService creates a new stream service.
func NewStreamService(client *Client) *StreamService {
	return &StreamService{client: client}
}

// GetGlobalStreamService returns a stream service using the global client.
func GetGlobalStreamService() *StreamService {
	client := GetClient()
	if client == nil {
		return nil
	}
	return NewStreamService(client)
}

// streamKey returns the Redis key for a session's message stream.
func (s *StreamService) streamKey(sessionID string) string {
	return StreamKeyPrefix + sessionID
}

// connectionKey returns the Redis key for tracking session connections.
func (s *StreamService) connectionKey(sessionID string) string {
	return ConnectionKeyPrefix + sessionID
}

// lastReadKey returns the Redis key for tracking last read message ID.
func (s *StreamService) lastReadKey(sessionID string) string {
	return LastReadKeyPrefix + sessionID
}

// activeGenerationKey returns the Redis key for tracking active generation.
func (s *StreamService) activeGenerationKey(sessionID string) string {
	return ActiveGenerationKeyPrefix + sessionID
}

// PublishMessage publishes a message to the session's stream.
func (s *StreamService) PublishMessage(ctx context.Context, sessionID string, msgType string, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	msg := StreamMessage{
		SessionID: sessionID,
		Type:      msgType,
		Payload:   payloadJSON,
		Timestamp: time.Now().UnixMilli(),
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal stream message: %w", err)
	}

	// Add to stream with max length limit
	streamKey := s.streamKey(sessionID)
	args := &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: s.client.streamMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"data": string(msgJSON),
		},
	}

	result, err := s.client.rdb.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	// Set TTL on the stream
	s.client.rdb.Expire(ctx, streamKey, s.client.streamTTL)

	slog.Debug("Published message to stream",
		"session_id", sessionID,
		"type", msgType,
		"stream_id", result,
	)

	return nil
}

// ReadMessages reads messages from the session's stream starting from the given ID.
// If startID is empty or "0", it reads from the beginning.
// If startID is "$", it only reads new messages.
func (s *StreamService) ReadMessages(ctx context.Context, sessionID string, startID string, count int64) ([]StreamMessage, string, error) {
	if startID == "" {
		startID = "0"
	}

	streamKey := s.streamKey(sessionID)
	result, err := s.client.rdb.XRange(ctx, streamKey, startID, "+").Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to read from stream: %w", err)
	}

	// If startID is not "0", skip the first message (it's the one we already have)
	startIdx := 0
	if startID != "0" && len(result) > 0 && result[0].ID == startID {
		startIdx = 1
	}

	messages := make([]StreamMessage, 0, len(result)-startIdx)
	var lastID string

	for i := startIdx; i < len(result); i++ {
		entry := result[i]
		lastID = entry.ID

		data, ok := entry.Values["data"].(string)
		if !ok {
			continue
		}

		var msg StreamMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			slog.Warn("Failed to unmarshal stream message", "error", err)
			continue
		}
		msg.ID = entry.ID
		messages = append(messages, msg)

		if count > 0 && int64(len(messages)) >= count {
			break
		}
	}

	return messages, lastID, nil
}

// ReadNewMessages reads only messages that arrived after the given ID using blocking read.
func (s *StreamService) ReadNewMessages(ctx context.Context, sessionID string, lastID string, blockTimeout time.Duration) ([]StreamMessage, string, error) {
	if lastID == "" {
		lastID = "$"
	}

	streamKey := s.streamKey(sessionID)
	result, err := s.client.rdb.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKey, lastID},
		Block:   blockTimeout,
		Count:   100,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, lastID, nil
		}
		return nil, "", fmt.Errorf("failed to read new messages: %w", err)
	}

	if len(result) == 0 || len(result[0].Messages) == 0 {
		return nil, lastID, nil
	}

	messages := make([]StreamMessage, 0, len(result[0].Messages))
	var newLastID string

	for _, entry := range result[0].Messages {
		newLastID = entry.ID

		data, ok := entry.Values["data"].(string)
		if !ok {
			continue
		}

		var msg StreamMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			slog.Warn("Failed to unmarshal stream message", "error", err)
			continue
		}
		msg.ID = entry.ID
		messages = append(messages, msg)
	}

	return messages, newLastID, nil
}

// SetConnectionStatus sets the connection status for a session.
func (s *StreamService) SetConnectionStatus(ctx context.Context, sessionID string, connected bool) error {
	key := s.connectionKey(sessionID)
	var value string
	if connected {
		value = "1"
	} else {
		value = "0"
	}

	err := s.client.rdb.Set(ctx, key, value, s.client.streamTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set connection status: %w", err)
	}

	slog.Debug("Set connection status",
		"session_id", sessionID,
		"connected", connected,
	)

	return nil
}

// IsConnected checks if a session has an active WebSocket connection.
func (s *StreamService) IsConnected(ctx context.Context, sessionID string) (bool, error) {
	key := s.connectionKey(sessionID)
	result, err := s.client.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("failed to get connection status: %w", err)
	}
	return result == "1", nil
}

// SetLastReadID stores the last read message ID for a session.
func (s *StreamService) SetLastReadID(ctx context.Context, sessionID string, messageID string) error {
	key := s.lastReadKey(sessionID)
	err := s.client.rdb.Set(ctx, key, messageID, s.client.streamTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set last read ID: %w", err)
	}
	return nil
}

// GetLastReadID gets the last read message ID for a session.
func (s *StreamService) GetLastReadID(ctx context.Context, sessionID string) (string, error) {
	key := s.lastReadKey(sessionID)
	result, err := s.client.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "0", nil
		}
		return "", fmt.Errorf("failed to get last read ID: %w", err)
	}
	return result, nil
}

// SetActiveGeneration marks a session as having an active generation in progress.
func (s *StreamService) SetActiveGeneration(ctx context.Context, sessionID string, active bool) error {
	key := s.activeGenerationKey(sessionID)
	if active {
		err := s.client.rdb.Set(ctx, key, "1", s.client.streamTTL).Err()
		if err != nil {
			return fmt.Errorf("failed to set active generation: %w", err)
		}
	} else {
		err := s.client.rdb.Del(ctx, key).Err()
		if err != nil {
			return fmt.Errorf("failed to clear active generation: %w", err)
		}
	}
	return nil
}

// IsGenerationActive checks if a session has an active generation in progress.
func (s *StreamService) IsGenerationActive(ctx context.Context, sessionID string) (bool, error) {
	key := s.activeGenerationKey(sessionID)
	result, err := s.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check active generation: %w", err)
	}
	return result > 0, nil
}

// ClearStream deletes a session's message stream.
func (s *StreamService) ClearStream(ctx context.Context, sessionID string) error {
	streamKey := s.streamKey(sessionID)
	err := s.client.rdb.Del(ctx, streamKey).Err()
	if err != nil {
		return fmt.Errorf("failed to clear stream: %w", err)
	}
	return nil
}

// GetStreamLength returns the number of messages in a session's stream.
func (s *StreamService) GetStreamLength(ctx context.Context, sessionID string) (int64, error) {
	streamKey := s.streamKey(sessionID)
	length, err := s.client.rdb.XLen(ctx, streamKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get stream length: %w", err)
	}
	return length, nil
}

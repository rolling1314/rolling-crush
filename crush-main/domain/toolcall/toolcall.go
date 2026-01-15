// Package toolcall provides tool call state management for agent interactions.
package toolcall

import (
	"context"
	"database/sql"
	"time"

	"github.com/rolling1314/rolling-crush/infra/postgres"
	"github.com/rolling1314/rolling-crush/internal/pubsub"
)

// Status represents the current status of a tool call
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
)

// ToolCall represents a tool call with its current state
type ToolCall struct {
	ID           string `json:"id"`
	SessionID    string `json:"session_id"`
	MessageID    string `json:"message_id,omitempty"`
	Name         string `json:"name"`
	Input        string `json:"input,omitempty"`
	Status       Status `json:"status"`
	Result       string `json:"result,omitempty"`
	IsError      bool   `json:"is_error"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	StartedAt    *int64 `json:"started_at,omitempty"`
	FinishedAt   *int64 `json:"finished_at,omitempty"`
}

// Service provides tool call state management operations
type Service interface {
	pubsub.Suscriber[ToolCall]
	// Create creates a new tool call record
	Create(ctx context.Context, sessionID, messageID, toolCallID, name string) (ToolCall, error)
	// Get retrieves a tool call by ID
	Get(ctx context.Context, id string) (ToolCall, error)
	// ListBySession lists all tool calls for a session
	ListBySession(ctx context.Context, sessionID string) ([]ToolCall, error)
	// ListByMessage lists all tool calls for a message
	ListByMessage(ctx context.Context, messageID string) ([]ToolCall, error)
	// ListPending lists pending/running tool calls for a session
	ListPending(ctx context.Context, sessionID string) ([]ToolCall, error)
	// UpdateInput updates the tool call input and marks it as running
	UpdateInput(ctx context.Context, id, input string) error
	// UpdateStatus updates the tool call status
	UpdateStatus(ctx context.Context, id string, status Status) error
	// Complete marks the tool call as completed with result
	Complete(ctx context.Context, id, result string, isError bool, errorMsg string) error
	// Cancel cancels a pending/running tool call
	Cancel(ctx context.Context, id string) error
	// CancelSession cancels all pending/running tool calls for a session
	CancelSession(ctx context.Context, sessionID string) error
	// Delete deletes a tool call
	Delete(ctx context.Context, id string) error
	// DeleteSession deletes all tool calls for a session
	DeleteSession(ctx context.Context, sessionID string) error
}

type service struct {
	*pubsub.Broker[ToolCall]
	q postgres.Querier
}

// NewService creates a new tool call service
func NewService(q postgres.Querier) Service {
	return &service{
		Broker: pubsub.NewBroker[ToolCall](),
		q:      q,
	}
}

func (s *service) Create(ctx context.Context, sessionID, messageID, toolCallID, name string) (ToolCall, error) {
	msgID := sql.NullString{}
	if messageID != "" {
		msgID = sql.NullString{String: messageID, Valid: true}
	}

	dbToolCall, err := s.q.CreateToolCall(ctx, postgres.CreateToolCallParams{
		ID:        toolCallID,
		SessionID: sessionID,
		MessageID: msgID,
		Name:      name,
		Status:    string(StatusPending),
	})
	if err != nil {
		return ToolCall{}, err
	}

	tc := s.fromDB(dbToolCall)
	s.Publish(pubsub.CreatedEvent, tc)
	return tc, nil
}

func (s *service) Get(ctx context.Context, id string) (ToolCall, error) {
	dbToolCall, err := s.q.GetToolCall(ctx, id)
	if err != nil {
		return ToolCall{}, err
	}
	return s.fromDB(dbToolCall), nil
}

func (s *service) ListBySession(ctx context.Context, sessionID string) ([]ToolCall, error) {
	dbToolCalls, err := s.q.ListToolCallsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.fromDBList(dbToolCalls), nil
}

func (s *service) ListByMessage(ctx context.Context, messageID string) ([]ToolCall, error) {
	dbToolCalls, err := s.q.ListToolCallsByMessage(ctx, sql.NullString{String: messageID, Valid: true})
	if err != nil {
		return nil, err
	}
	return s.fromDBList(dbToolCalls), nil
}

func (s *service) ListPending(ctx context.Context, sessionID string) ([]ToolCall, error) {
	dbToolCalls, err := s.q.ListPendingToolCalls(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.fromDBList(dbToolCalls), nil
}

func (s *service) UpdateInput(ctx context.Context, id, input string) error {
	err := s.q.UpdateToolCallInput(ctx, postgres.UpdateToolCallInputParams{
		ID:    id,
		Input: sql.NullString{String: input, Valid: true},
	})
	if err != nil {
		return err
	}

	tc, err := s.Get(ctx, id)
	if err == nil {
		s.Publish(pubsub.UpdatedEvent, tc)
	}
	return nil
}

func (s *service) UpdateStatus(ctx context.Context, id string, status Status) error {
	err := s.q.UpdateToolCallStatus(ctx, postgres.UpdateToolCallStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		return err
	}

	tc, err := s.Get(ctx, id)
	if err == nil {
		s.Publish(pubsub.UpdatedEvent, tc)
	}
	return nil
}

func (s *service) Complete(ctx context.Context, id, result string, isError bool, errorMsg string) error {
	err := s.q.UpdateToolCallResult(ctx, postgres.UpdateToolCallResultParams{
		ID:           id,
		Result:       sql.NullString{String: result, Valid: result != ""},
		IsError:      isError,
		ErrorMessage: sql.NullString{String: errorMsg, Valid: errorMsg != ""},
	})
	if err != nil {
		return err
	}

	tc, err := s.Get(ctx, id)
	if err == nil {
		s.Publish(pubsub.UpdatedEvent, tc)
	}
	return nil
}

func (s *service) Cancel(ctx context.Context, id string) error {
	err := s.q.CancelToolCall(ctx, id)
	if err != nil {
		return err
	}

	tc, err := s.Get(ctx, id)
	if err == nil {
		s.Publish(pubsub.UpdatedEvent, tc)
	}
	return nil
}

func (s *service) CancelSession(ctx context.Context, sessionID string) error {
	// Get pending tool calls before cancelling
	pending, _ := s.ListPending(ctx, sessionID)

	err := s.q.CancelSessionToolCalls(ctx, sessionID)
	if err != nil {
		return err
	}

	// Publish update events for all cancelled tool calls
	for _, tc := range pending {
		tc.Status = StatusCancelled
		tc.UpdatedAt = time.Now().UnixMilli()
		s.Publish(pubsub.UpdatedEvent, tc)
	}
	return nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	tc, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	err = s.q.DeleteToolCall(ctx, id)
	if err != nil {
		return err
	}

	s.Publish(pubsub.DeletedEvent, tc)
	return nil
}

func (s *service) DeleteSession(ctx context.Context, sessionID string) error {
	return s.q.DeleteSessionToolCalls(ctx, sessionID)
}

func (s *service) fromDB(db postgres.ToolCall) ToolCall {
	tc := ToolCall{
		ID:        db.ID,
		SessionID: db.SessionID,
		Name:      db.Name,
		Status:    Status(db.Status),
		IsError:   db.IsError,
		CreatedAt: db.CreatedAt,
		UpdatedAt: db.UpdatedAt,
	}

	if db.MessageID.Valid {
		tc.MessageID = db.MessageID.String
	}
	if db.Input.Valid {
		tc.Input = db.Input.String
	}
	if db.Result.Valid {
		tc.Result = db.Result.String
	}
	if db.ErrorMessage.Valid {
		tc.ErrorMessage = db.ErrorMessage.String
	}
	if db.StartedAt.Valid {
		tc.StartedAt = &db.StartedAt.Int64
	}
	if db.FinishedAt.Valid {
		tc.FinishedAt = &db.FinishedAt.Int64
	}

	return tc
}

func (s *service) fromDBList(dbList []postgres.ToolCall) []ToolCall {
	result := make([]ToolCall, len(dbList))
	for i, db := range dbList {
		result[i] = s.fromDB(db)
	}
	return result
}

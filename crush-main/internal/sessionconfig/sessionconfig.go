package sessionconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Config represents the model configuration stored as JSON
type Config struct {
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	BaseURL         string   `json:"base_url,omitempty"`
	APIKey          string   `json:"api_key"`
	MaxTokens       *int64   `json:"max_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	Think           bool     `json:"think,omitempty"`
}

type Service interface {
	Save(ctx context.Context, sessionID string, config Config) error
	Get(ctx context.Context, sessionID string) (*Config, error)
	Delete(ctx context.Context, sessionID string) error
}

type service struct {
	db DBTX
}

// DBTX is the database interface we need (matches db.DBTX)
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// NewService creates a new session config service using raw SQL queries
func NewService(q interface{}) Service {
	// The querier (db.Queries) itself implements DBTX
	if dbtx, ok := q.(DBTX); ok {
		slog.Info("Session config service initialized with database connection")
		return &service{db: dbtx}
	}

	// Fallback: if we can't get the DB, log a warning
	slog.Warn("Could not extract database connection from querier, session config will not be saved")
	return &noopService{}
}

func (s *service) Save(ctx context.Context, sessionID string, config Config) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	// First, try to check if the table exists
	var tableExists bool
	err = s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'session_model_configs'
		)
	`).Scan(&tableExists)

	if err != nil || !tableExists {
		slog.Warn("session_model_configs table does not exist, config not saved",
			"session_id", sessionID,
			"provider", config.Provider,
			"model", config.Model)
		return nil // Don't fail, just skip
	}

	// Try to update existing config
	result, err := s.db.ExecContext(ctx, `
		UPDATE session_model_configs
		SET config_json = $1, updated_at = $2
		WHERE session_id = $3
	`, configJSON, now, sessionID)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were updated, insert a new record
	if rowsAffected == 0 {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO session_model_configs (id, session_id, config_json, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New().String(), sessionID, configJSON, now, now)

		if err != nil {
			return err
		}

		slog.Info("Created session model config in database",
			"session_id", sessionID,
			"provider", config.Provider,
			"model", config.Model)
	} else {
		slog.Info("Updated session model config in database",
			"session_id", sessionID,
			"provider", config.Provider,
			"model", config.Model)
	}

	return nil
}

func (s *service) Get(ctx context.Context, sessionID string) (*Config, error) {
	var configJSON []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT config_json FROM session_model_configs WHERE session_id = $1 LIMIT 1
	`, sessionID).Scan(&configJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (s *service) Delete(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM session_model_configs WHERE session_id = $1
	`, sessionID)

	if err != nil {
		return err
	}

	slog.Info("Deleted session model config from database", "session_id", sessionID)
	return nil
}

// noopService is a fallback that does nothing
type noopService struct{}

func (n *noopService) Save(ctx context.Context, sessionID string, config Config) error {
	slog.Warn("Session config not saved (no database connection)")
	return nil
}

func (n *noopService) Get(ctx context.Context, sessionID string) (*Config, error) {
	return nil, nil
}

func (n *noopService) Delete(ctx context.Context, sessionID string) error {
	return nil
}

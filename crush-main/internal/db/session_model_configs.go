package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// SessionModelConfig represents the model configuration stored as JSON
type SessionModelConfig struct {
	ID         string
	SessionID  string
	ConfigJSON []byte
	CreatedAt  int64
	UpdatedAt  int64
}

// SessionConfigParams is used to save/update session config
type SessionConfigParams struct {
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

// CreateSessionModelConfig inserts a new session model config
func (q *Queries) CreateSessionModelConfig(ctx context.Context, sessionID string, config SessionConfigParams) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	// First, check if the table exists
	var tableExists bool
	err = q.db.QueryRowContext(ctx, `
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
	result, err := q.db.ExecContext(ctx, `
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
		_, err = q.db.ExecContext(ctx, `
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

// GetSessionModelConfig retrieves the config for a session
func (q *Queries) GetSessionModelConfig(ctx context.Context, sessionID string) (*SessionConfigParams, error) {
	var configJSON []byte
	err := q.db.QueryRowContext(ctx, `
		SELECT config_json FROM session_model_configs WHERE session_id = $1 LIMIT 1
	`, sessionID).Scan(&configJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var config SessionConfigParams
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// DeleteSessionModelConfig deletes the config for a session
func (q *Queries) DeleteSessionModelConfig(ctx context.Context, sessionID string) error {
	_, err := q.db.ExecContext(ctx, `
		DELETE FROM session_model_configs WHERE session_id = $1
	`, sessionID)

	if err != nil {
		return err
	}

	slog.Info("Deleted session model config from database", "session_id", sessionID)
	return nil
}

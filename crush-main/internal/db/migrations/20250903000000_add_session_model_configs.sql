-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS session_model_configs (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    config_json JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_model_configs_session_id ON session_model_configs(session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_session_model_configs_session_id;
DROP TABLE IF EXISTS session_model_configs;
-- +goose StatementEnd

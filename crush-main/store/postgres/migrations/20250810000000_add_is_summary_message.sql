-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS is_summary_message BIGINT DEFAULT 0 NOT NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS is_summary_message;

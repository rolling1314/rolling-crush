-- +goose Up
-- +goose StatementBegin
-- Add provider column to messages table
ALTER TABLE messages ADD COLUMN IF NOT EXISTS provider TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove provider column from messages table
ALTER TABLE messages DROP COLUMN IF EXISTS provider;
-- +goose StatementEnd
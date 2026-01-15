-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS summary_message_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN IF EXISTS summary_message_id;
-- +goose StatementEnd

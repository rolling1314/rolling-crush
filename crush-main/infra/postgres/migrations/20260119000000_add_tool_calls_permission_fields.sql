-- +goose Up
-- +goose StatementBegin

-- Add permission-related fields to tool_calls table
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS permission_requested_at BIGINT;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS original_prompt TEXT;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS permission_action TEXT;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS permission_path TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove permission-related fields from tool_calls table
ALTER TABLE tool_calls DROP COLUMN IF EXISTS permission_path;
ALTER TABLE tool_calls DROP COLUMN IF EXISTS permission_action;
ALTER TABLE tool_calls DROP COLUMN IF EXISTS original_prompt;
ALTER TABLE tool_calls DROP COLUMN IF EXISTS permission_requested_at;

-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT,
    title TEXT NOT NULL,
    message_count BIGINT NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    prompt_tokens  BIGINT NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens  BIGINT NOT NULL DEFAULT 0 CHECK (completion_tokens>= 0),
    cost DOUBLE PRECISION NOT NULL DEFAULT 0.0 CHECK (cost >= 0.0),
    updated_at BIGINT NOT NULL,  -- Unix timestamp in milliseconds
    created_at BIGINT NOT NULL   -- Unix timestamp in milliseconds
);

-- Trigger to auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_sessions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = EXTRACT(EPOCH FROM NOW()) * 1000;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_sessions_updated_at_trigger
BEFORE UPDATE ON sessions
FOR EACH ROW
EXECUTE FUNCTION update_sessions_updated_at();

-- Files
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    path TEXT NOT NULL,
    content TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,  -- Unix timestamp in milliseconds
    updated_at BIGINT NOT NULL,  -- Unix timestamp in milliseconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    UNIQUE(path, session_id, version)
);

CREATE INDEX IF NOT EXISTS idx_files_session_id ON files (session_id);
CREATE INDEX IF NOT EXISTS idx_files_path ON files (path);

-- Trigger to auto-update updated_at timestamp for files
CREATE OR REPLACE FUNCTION update_files_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = EXTRACT(EPOCH FROM NOW()) * 1000;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_files_updated_at_trigger
BEFORE UPDATE ON files
FOR EACH ROW
EXECUTE FUNCTION update_files_updated_at();

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    parts TEXT NOT NULL default '[]',
    model TEXT,
    created_at BIGINT NOT NULL,  -- Unix timestamp in milliseconds
    updated_at BIGINT NOT NULL,  -- Unix timestamp in milliseconds
    finished_at BIGINT,  -- Unix timestamp in milliseconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages (session_id);

-- Trigger to auto-update updated_at timestamp for messages
CREATE OR REPLACE FUNCTION update_messages_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = EXTRACT(EPOCH FROM NOW()) * 1000;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_messages_updated_at_trigger
BEFORE UPDATE ON messages
FOR EACH ROW
EXECUTE FUNCTION update_messages_updated_at();

-- Trigger to update session message count on insert
CREATE OR REPLACE FUNCTION update_session_message_count_on_insert()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE sessions SET
        message_count = message_count + 1
    WHERE id = NEW.session_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_session_message_count_on_insert_trigger
AFTER INSERT ON messages
FOR EACH ROW
EXECUTE FUNCTION update_session_message_count_on_insert();

-- Trigger to update session message count on delete
CREATE OR REPLACE FUNCTION update_session_message_count_on_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE sessions SET
        message_count = message_count - 1
    WHERE id = OLD.session_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_session_message_count_on_delete_trigger
AFTER DELETE ON messages
FOR EACH ROW
EXECUTE FUNCTION update_session_message_count_on_delete();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_sessions_updated_at_trigger ON sessions;
DROP FUNCTION IF EXISTS update_sessions_updated_at();

DROP TRIGGER IF EXISTS update_messages_updated_at_trigger ON messages;
DROP FUNCTION IF EXISTS update_messages_updated_at();

DROP TRIGGER IF EXISTS update_files_updated_at_trigger ON files;
DROP FUNCTION IF EXISTS update_files_updated_at();

DROP TRIGGER IF EXISTS update_session_message_count_on_delete_trigger ON messages;
DROP FUNCTION IF EXISTS update_session_message_count_on_delete();

DROP TRIGGER IF EXISTS update_session_message_count_on_insert_trigger ON messages;
DROP FUNCTION IF EXISTS update_session_message_count_on_insert();

DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS sessions;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Add host, port, and workspace_path columns to projects table if they don't exist
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='host') THEN
        ALTER TABLE projects ADD COLUMN host TEXT NOT NULL DEFAULT 'localhost';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='port') THEN
        ALTER TABLE projects ADD COLUMN port INTEGER NOT NULL DEFAULT 8080;
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='workspace_path') THEN
        ALTER TABLE projects ADD COLUMN workspace_path TEXT NOT NULL DEFAULT '.';
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS workspace_path;
ALTER TABLE projects DROP COLUMN IF EXISTS port;
ALTER TABLE projects DROP COLUMN IF EXISTS host;

-- +goose StatementEnd


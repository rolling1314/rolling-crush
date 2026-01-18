-- +goose Up
-- +goose StatementBegin

-- Safely rename or create external_ip column
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='host') THEN
        ALTER TABLE projects RENAME COLUMN host TO external_ip;
    ELSIF NOT EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='external_ip') THEN
        ALTER TABLE projects ADD COLUMN external_ip TEXT NOT NULL DEFAULT 'localhost';
    END IF;
END $$;

-- Safely rename or create frontend_port column
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='port') THEN
        ALTER TABLE projects RENAME COLUMN port TO frontend_port;
    ELSIF NOT EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='frontend_port') THEN
        ALTER TABLE projects ADD COLUMN frontend_port INTEGER NOT NULL DEFAULT 8080;
    END IF;
END $$;

-- Add missing columns
ALTER TABLE projects ADD COLUMN IF NOT EXISTS container_name TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS workdir_path TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS db_host TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS db_port INTEGER;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS db_user TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS db_password TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS db_name TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS backend_port INTEGER;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS frontend_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS frontend_language TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS backend_command TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS backend_language TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS backend_language;
ALTER TABLE projects DROP COLUMN IF EXISTS backend_command;
ALTER TABLE projects DROP COLUMN IF EXISTS frontend_language;
ALTER TABLE projects DROP COLUMN IF EXISTS frontend_command;
ALTER TABLE projects DROP COLUMN IF EXISTS backend_port;
ALTER TABLE projects DROP COLUMN IF EXISTS db_name;
ALTER TABLE projects DROP COLUMN IF EXISTS db_password;
ALTER TABLE projects DROP COLUMN IF EXISTS db_user;
ALTER TABLE projects DROP COLUMN IF EXISTS db_port;
ALTER TABLE projects DROP COLUMN IF EXISTS db_host;
ALTER TABLE projects DROP COLUMN IF EXISTS workdir_path;
ALTER TABLE projects DROP COLUMN IF EXISTS container_name;

-- Revert external_ip to host
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='external_ip') THEN
        ALTER TABLE projects RENAME COLUMN external_ip TO host;
    END IF;
END $$;

-- Revert frontend_port to port
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='projects' AND column_name='frontend_port') THEN
        ALTER TABLE projects RENAME COLUMN frontend_port TO port;
    END IF;
END $$;

-- +goose StatementEnd

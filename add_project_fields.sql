-- 手动添加 host, port, workspace_path 字段到 projects 表

-- 添加 host 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='host') THEN
        ALTER TABLE projects ADD COLUMN host TEXT NOT NULL DEFAULT 'localhost';
        RAISE NOTICE 'Added column host to projects';
    ELSE
        RAISE NOTICE 'Column host already exists in projects';
    END IF;
END $$;

-- 添加 port 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='port') THEN
        ALTER TABLE projects ADD COLUMN port INTEGER NOT NULL DEFAULT 8080;
        RAISE NOTICE 'Added column port to projects';
    ELSE
        RAISE NOTICE 'Column port already exists in projects';
    END IF;
END $$;

-- 添加 workspace_path 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='workspace_path') THEN
        ALTER TABLE projects ADD COLUMN workspace_path TEXT NOT NULL DEFAULT '.';
        RAISE NOTICE 'Added column workspace_path to projects';
    ELSE
        RAISE NOTICE 'Column workspace_path already exists in projects';
    END IF;
END $$;

-- 查看 projects 表结构（验证）
SELECT column_name, data_type, column_default 
FROM information_schema.columns 
WHERE table_name = 'projects' 
ORDER BY ordinal_position;


#!/bin/bash

# 手动执行迁移脚本

echo "🔧 Adding host, port, workspace_path fields to projects table..."

# 使用 Docker 执行
docker exec -i crush-postgres psql -U crush -d crush << 'EOF'

-- 添加 host 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='host') THEN
        ALTER TABLE projects ADD COLUMN host TEXT NOT NULL DEFAULT 'localhost';
        RAISE NOTICE '✅ Added column host';
    ELSE
        RAISE NOTICE '⚠️  Column host already exists';
    END IF;
END $$;

-- 添加 port 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='port') THEN
        ALTER TABLE projects ADD COLUMN port INTEGER NOT NULL DEFAULT 8080;
        RAISE NOTICE '✅ Added column port';
    ELSE
        RAISE NOTICE '⚠️  Column port already exists';
    END IF;
END $$;

-- 添加 workspace_path 字段
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='projects' AND column_name='workspace_path') THEN
        ALTER TABLE projects ADD COLUMN workspace_path TEXT NOT NULL DEFAULT '.';
        RAISE NOTICE '✅ Added column workspace_path';
    ELSE
        RAISE NOTICE '⚠️  Column workspace_path already exists';
    END IF;
END $$;

EOF

echo ""
echo "📋 Verifying projects table structure..."

docker exec -i crush-postgres psql -U crush -d crush -c "
SELECT column_name, data_type, column_default 
FROM information_schema.columns 
WHERE table_name = 'projects' 
ORDER BY ordinal_position;
"

echo ""
echo "✅ Done! Please restart the Crush application."


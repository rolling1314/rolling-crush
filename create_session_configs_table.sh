#!/bin/bash

echo "================================================"
echo "  创建 session_model_configs 表"
echo "================================================"
echo ""

# 检查是否提供了密码
if [ -z "$POSTGRES_PASSWORD" ]; then
    echo "请输入 PostgreSQL 密码（默认: 123456）:"
    read -s password
    POSTGRES_PASSWORD=${password:-123456}
fi

# 数据库连接信息
PG_HOST=${POSTGRES_HOST:-localhost}
PG_PORT=${POSTGRES_PORT:-5432}
PG_USER=${POSTGRES_USER:-crushuser}
PG_DB=${POSTGRES_DB:-crushdb}

echo "连接信息:"
echo "  Host: $PG_HOST"
echo "  Port: $PG_PORT"
echo "  User: $PG_USER"
echo "  Database: $PG_DB"
echo ""

# 执行 SQL
PGPASSWORD=$POSTGRES_PASSWORD psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" << 'EOF'
-- 1. 确保 projects 表有必要的字段
ALTER TABLE projects ADD COLUMN IF NOT EXISTS host TEXT NOT NULL DEFAULT 'localhost';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS port INTEGER NOT NULL DEFAULT 8080;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS workspace_path TEXT NOT NULL DEFAULT '.';

-- 2. 删除旧表（如果存在）
DROP TABLE IF EXISTS session_model_configs CASCADE;

-- 3. 创建 session_model_configs 表
CREATE TABLE session_model_configs (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    config_json JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 4. 创建索引
CREATE INDEX idx_session_model_configs_session_id ON session_model_configs(session_id);

-- 5. 验证表已创建
SELECT 
    table_name, 
    column_name, 
    data_type 
FROM information_schema.columns 
WHERE table_name = 'session_model_configs' 
ORDER BY ordinal_position;

-- 6. 显示表结构
\d session_model_configs
EOF

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ session_model_configs 表创建成功！"
    echo ""
else
    echo ""
    echo "❌ 创建失败，请检查数据库连接信息"
    echo ""
    exit 1
fi


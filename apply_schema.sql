-- 应用到 PostgreSQL 数据库的 Schema 更改

-- 1. 添加 projects 表的缺失字段（如果不存在）
ALTER TABLE projects ADD COLUMN IF NOT EXISTS host TEXT NOT NULL DEFAULT 'localhost';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS port INTEGER NOT NULL DEFAULT 8080;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS workspace_path TEXT NOT NULL DEFAULT '.';

-- 2. 删除旧的 session_model_configs 表（如果存在）
DROP TABLE IF EXISTS session_model_configs CASCADE;

-- 3. 创建新的 session_model_configs 表，使用 JSONB 存储配置
CREATE TABLE session_model_configs (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    config_json JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 4. 创建索引以提高查询性能
CREATE INDEX idx_session_model_configs_session_id ON session_model_configs(session_id);

-- 验证表已创建
SELECT table_name, column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'session_model_configs' 
ORDER BY ordinal_position;


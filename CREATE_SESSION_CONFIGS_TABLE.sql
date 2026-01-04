-- 在 Navicat 中执行此 SQL 来创建 session_model_configs 表

-- 1. 删除旧表（如果存在）
DROP TABLE IF EXISTS session_model_configs CASCADE;

-- 2. 创建 session_model_configs 表
CREATE TABLE session_model_configs (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    config_json JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CONSTRAINT fk_session_model_configs_session 
        FOREIGN KEY (session_id) 
        REFERENCES sessions(id) 
        ON DELETE CASCADE
);

-- 3. 创建索引（提高查询性能）
CREATE INDEX idx_session_model_configs_session_id ON session_model_configs(session_id);

-- 4. 验证表已创建（可选）
SELECT 
    table_name, 
    column_name, 
    data_type,
    is_nullable
FROM information_schema.columns 
WHERE table_name = 'session_model_configs' 
ORDER BY ordinal_position;

-- 5. 查看表结构（可选）
-- \d session_model_configs


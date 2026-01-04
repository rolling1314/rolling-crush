# 配置存储功能完成步骤

## 当前状态
✅ 前端已添加 API Key 和配置输入
✅ 后端代码已准备好存储逻辑
❌ 数据库表还未创建（导致编译失败）
❌ sqlc 还未生成代码

## 需要执行的步骤

### 步骤 1: 连接到 PostgreSQL 数据库

找到你的 PostgreSQL 容器或连接信息，然后执行：

```bash
# 方式 1: 如果使用 Docker
docker exec -i <容器名> psql -U crushuser -d crushdb < /Users/apple/crush/apply_schema.sql

# 方式 2: 直接使用 psql 客户端
psql -h localhost -p 5432 -U crushuser -d crushdb < /Users/apple/crush/apply_schema.sql

# 方式 3: 手动复制粘贴 SQL
psql -h localhost -p 5432 -U crushuser -d crushdb
# 然后粘贴 apply_schema.sql 中的内容
```

### 步骤 2: 验证表已创建

```sql
-- 在 psql 中执行
\dt session_model_configs
\d session_model_configs
```

你应该看到类似这样的输出：
```
                   Table "public.session_model_configs"
   Column    |  Type  | Collation | Nullable | Default 
-------------+--------+-----------+----------+---------
 id          | text   |           | not null | 
 session_id  | text   |           | not null | 
 config_json | jsonb  |           | not null | 
 created_at  | bigint |           | not null | 
 updated_at  | bigint |           | not null | 
```

### 步骤 3: 生成 sqlc 代码

```bash
cd /Users/apple/crush/crush-main
$HOME/go/bin/sqlc generate
```

### 步骤 4: 编译后端

```bash
cd /Users/apple/crush/crush-main
go build .
```

## SQL 文件位置

所有需要的 SQL 已保存在：
- `/Users/apple/crush/apply_schema.sql` - 完整的 schema 更改

## 配置存储格式

配置会以 JSON 格式存储在 `session_model_configs.config_json` 字段中：

```json
{
  "provider": "zai",
  "model": "glm-4.5",
  "api_key": "your-api-key",
  "base_url": "https://api.example.com/v1",
  "max_tokens": 98304,
  "temperature": 0.7,
  "top_p": 0.9,
  "reasoning_effort": "medium",
  "think": false
}
```

## 如果暂时无法应用数据库更改

如果现在无法连接数据库，可以临时注释掉配置保存功能：

1. 编辑 `/Users/apple/crush/crush-main/internal/httpserver/server.go`
2. 找到 `handleCreateSession` 函数中的配置保存逻辑
3. 临时注释掉对 `s.sessionConfigService.Save` 的调用

但这样配置就不会被保存到数据库中。


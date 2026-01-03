# PostgreSQL 数据库设置指南

本文档提供了为 Crush 项目配置和使用 PostgreSQL 数据库的完整指南。

## 目录

1. [安装 PostgreSQL](#1-安装-postgresql)
2. [创建数据库和用户](#2-创建数据库和用户)
3. [配置环境变量](#3-配置环境变量)
4. [运行数据库迁移](#4-运行数据库迁移)
5. [连接信息](#5-连接信息)
6. [故障排除](#6-故障排除)

---

## 1. 安装 PostgreSQL

### macOS (使用 Homebrew)

```bash
brew install postgresql@16
brew services start postgresql@16
```

### Ubuntu/Debian

```bash
sudo apt update
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

### CentOS/RHEL

```bash
sudo yum install postgresql-server postgresql-contrib
sudo postgresql-setup initdb
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

### 使用 Docker

```bash
docker run --name crush-postgres \
  -e POSTGRES_USER=crush \
  -e POSTGRES_PASSWORD=your_secure_password \
  -e POSTGRES_DB=crush \
  -p 5432:5432 \
  -d postgres:16-alpine
```

---

## 2. 创建数据库和用户

### 方法 1: 使用 psql 命令行

连接到 PostgreSQL：

```bash
# macOS/Linux
sudo -u postgres psql

# Docker
docker exec -it crush-postgres psql -U postgres
```

在 psql 中执行以下命令：

```sql
-- 创建用户
CREATE USER crush WITH PASSWORD 'your_secure_password';

-- 创建数据库
CREATE DATABASE crush OWNER crush;

-- 授予权限
GRANT ALL PRIVILEGES ON DATABASE crush TO crush;

-- 连接到 crush 数据库
\c crush

-- 授予 schema 权限 (PostgreSQL 15+)
GRANT ALL ON SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO crush;

-- 设置默认权限
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO crush;
```

退出 psql：

```sql
\q
```

### 方法 2: 使用 SQL 脚本

创建一个名为 `setup_db.sql` 的文件：

```sql
-- setup_db.sql
CREATE USER crush WITH PASSWORD 'your_secure_password';
CREATE DATABASE crush OWNER crush;
GRANT ALL PRIVILEGES ON DATABASE crush TO crush;

\c crush

GRANT ALL ON SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO crush;
```

执行脚本：

```bash
sudo -u postgres psql -f setup_db.sql
```

---

## 3. 配置环境变量

Crush 通过环境变量配置数据库连接。请设置以下环境变量：

### 必需的环境变量

```bash
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="crush"
export POSTGRES_PASSWORD="your_secure_password"
export POSTGRES_DB="crush"
export POSTGRES_SSLMODE="disable"  # 本地开发使用 "disable"，生产环境使用 "require"
```

### 推荐配置方法

#### 方法 1: 使用 .env 文件 (推荐用于开发)

在项目根目录创建 `.env` 文件：

```bash
# .env
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=crush
POSTGRES_PASSWORD=your_secure_password
POSTGRES_DB=crush
POSTGRES_SSLMODE=disable
```

**注意**: 确保 `.env` 文件已添加到 `.gitignore`，不要提交到版本控制！

#### 方法 2: 使用 Shell 配置文件 (推荐用于永久配置)

在 `~/.bashrc`, `~/.zshrc` 或 `~/.bash_profile` 中添加：

```bash
# PostgreSQL Configuration for Crush
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="crush"
export POSTGRES_PASSWORD="your_secure_password"
export POSTGRES_DB="crush"
export POSTGRES_SSLMODE="disable"
```

然后重新加载配置：

```bash
source ~/.zshrc  # 或 source ~/.bashrc
```

#### 方法 3: 临时设置 (用于测试)

```bash
POSTGRES_HOST=localhost \
POSTGRES_PORT=5432 \
POSTGRES_USER=crush \
POSTGRES_PASSWORD=your_secure_password \
POSTGRES_DB=crush \
POSTGRES_SSLMODE=disable \
./crush
```

---

## 4. 运行数据库迁移

数据库迁移会在应用启动时自动运行。迁移文件位于：

```
crush-main/internal/db/migrations/
```

包含的迁移：

1. `20250424200609_initial.sql` - 创建基础表（sessions, files, messages）
2. `20250515105448_add_summary_message_id.sql` - 添加摘要消息 ID
3. `20250624000000_add_created_at_indexes.sql` - 添加时间戳索引
4. `20250627000000_add_provider_to_messages.sql` - 添加提供商字段
5. `20250810000000_add_is_summary_message.sql` - 添加摘要消息标记

### 手动运行迁移（如果需要）

如果需要手动运行迁移，可以使用 goose：

```bash
# 安装 goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# 运行迁移
cd crush-main/internal/db
goose postgres "host=localhost port=5432 user=crush password=your_secure_password dbname=crush sslmode=disable" up
```

---

## 5. 连接信息

### 默认连接参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `POSTGRES_HOST` | `localhost` | 数据库主机地址 |
| `POSTGRES_PORT` | `5432` | 数据库端口 |
| `POSTGRES_USER` | `crush` | 数据库用户名 |
| `POSTGRES_PASSWORD` | *必需* | 数据库密码（无默认值） |
| `POSTGRES_DB` | `crush` | 数据库名称 |
| `POSTGRES_SSLMODE` | `disable` | SSL 模式 |

### SSL 模式选项

- `disable` - 不使用 SSL（仅用于本地开发）
- `require` - 需要 SSL 但不验证证书
- `verify-ca` - 需要 SSL 并验证 CA
- `verify-full` - 需要 SSL 并完全验证证书

**生产环境建议**: 使用 `require` 或 `verify-full`

### 连接池配置

代码中的默认连接池配置：

```go
db.SetMaxOpenConns(25)    // 最大打开连接数
db.SetMaxIdleConns(5)     // 最大空闲连接数
db.SetConnMaxLifetime(0)  // 连接最大生命周期（0 表示永不关闭）
```

---

## 6. 故障排除

### 问题 1: 无法连接到数据库

**错误信息**: `failed to connect to database: connection refused`

**解决方案**:

1. 确认 PostgreSQL 服务正在运行：

```bash
# macOS (Homebrew)
brew services list | grep postgresql

# Linux
sudo systemctl status postgresql

# Docker
docker ps | grep postgres
```

2. 检查端口是否被占用：

```bash
lsof -i :5432
```

3. 确认连接参数正确：

```bash
psql -h localhost -p 5432 -U crush -d crush
```

### 问题 2: 密码验证失败

**错误信息**: `password authentication failed for user "crush"`

**解决方案**:

1. 确认密码正确
2. 重置用户密码：

```sql
ALTER USER crush WITH PASSWORD 'new_password';
```

3. 检查 PostgreSQL 配置文件 `pg_hba.conf`：

```bash
# 查找配置文件
sudo -u postgres psql -c "SHOW hba_file;"

# 确保有类似以下的行：
# local   all   all                     md5
# host    all   all   127.0.0.1/32      md5
```

### 问题 3: 权限不足

**错误信息**: `permission denied for schema public`

**解决方案**:

```sql
-- 以 postgres 用户连接
sudo -u postgres psql -d crush

-- 授予权限
GRANT ALL ON SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO crush;
```

### 问题 4: 数据库不存在

**错误信息**: `database "crush" does not exist`

**解决方案**:

```bash
sudo -u postgres createdb -O crush crush
```

或使用 psql：

```sql
CREATE DATABASE crush OWNER crush;
```

### 问题 5: 迁移失败

**错误信息**: `failed to apply migrations`

**解决方案**:

1. 检查迁移历史：

```sql
SELECT * FROM goose_db_version ORDER BY id;
```

2. 如果需要重置迁移（**警告：会删除所有数据**）：

```sql
-- 删除所有表
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO crush;
```

3. 重新运行应用以应用迁移

### 问题 6: 连接数过多

**错误信息**: `too many connections`

**解决方案**:

1. 检查当前连接数：

```sql
SELECT count(*) FROM pg_stat_activity WHERE datname = 'crush';
```

2. 查看最大连接数：

```sql
SHOW max_connections;
```

3. 增加最大连接数（需要重启 PostgreSQL）：

```sql
ALTER SYSTEM SET max_connections = 200;
```

然后重启 PostgreSQL：

```bash
sudo systemctl restart postgresql
```

---

## 生产环境建议

### 1. 安全性

- ✅ 使用强密码（至少 16 个字符，包含大小写字母、数字和特殊字符）
- ✅ 启用 SSL/TLS（`POSTGRES_SSLMODE=require`）
- ✅ 限制数据库访问 IP（在 `pg_hba.conf` 中配置）
- ✅ 定期更新 PostgreSQL 版本
- ✅ 不要在代码中硬编码密码

### 2. 性能优化

- ✅ 调整 `shared_buffers`（建议为系统内存的 25%）
- ✅ 启用连接池（如 PgBouncer）
- ✅ 定期执行 `VACUUM` 和 `ANALYZE`
- ✅ 监控慢查询日志
- ✅ 根据负载调整连接池大小

### 3. 备份策略

```bash
# 每日备份
pg_dump -U crush -d crush -F c -f backup_$(date +%Y%m%d).dump

# 恢复备份
pg_restore -U crush -d crush -c backup_20250103.dump
```

### 4. 监控

监控以下指标：

- 连接数
- 慢查询
- 磁盘使用量
- 缓存命中率
- 锁等待

推荐工具：

- pgAdmin
- Grafana + Prometheus
- pg_stat_statements

---

## 从 SQLite 迁移到 PostgreSQL

如果您之前使用 SQLite，需要迁移数据：

### 方法 1: 使用 pgloader

```bash
# 安装 pgloader
brew install pgloader  # macOS
sudo apt install pgloader  # Ubuntu

# 迁移数据
pgloader \
  .crush/crush.db \
  postgresql://crush:your_secure_password@localhost:5432/crush
```

### 方法 2: 导出导入

```bash
# 1. 从 SQLite 导出数据
sqlite3 .crush/crush.db .dump > dump.sql

# 2. 手动调整 SQL 语法（SQLite 和 PostgreSQL 有差异）
# 3. 导入到 PostgreSQL
psql -U crush -d crush -f dump.sql
```

---

## 快速开始示例

完整的设置流程：

```bash
# 1. 安装 PostgreSQL (如果未安装)
brew install postgresql@16
brew services start postgresql@16

# 2. 创建数据库和用户
sudo -u postgres psql << EOF
CREATE USER crush WITH PASSWORD 'my_secure_password_123';
CREATE DATABASE crush OWNER crush;
GRANT ALL PRIVILEGES ON DATABASE crush TO crush;
\c crush
GRANT ALL ON SCHEMA public TO crush;
EOF

# 3. 设置环境变量
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=crush
export POSTGRES_PASSWORD=my_secure_password_123
export POSTGRES_DB=crush
export POSTGRES_SSLMODE=disable

# 4. 编译并运行应用
cd crush-main
go build .
./crush

# 迁移会自动运行！
```

---

## 联系和支持

如果遇到问题，请：

1. 检查 PostgreSQL 日志：`tail -f /usr/local/var/log/postgresql@16.log`
2. 查看应用日志
3. 参考 [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
4. 提交 Issue 到项目仓库

---

## 附录：Docker Compose 配置

如果使用 Docker Compose，可以创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: crush-postgres
    environment:
      POSTGRES_USER: crush
      POSTGRES_PASSWORD: your_secure_password
      POSTGRES_DB: crush
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U crush"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

启动：

```bash
docker-compose up -d
```

停止：

```bash
docker-compose down
```

---

**最后更新**: 2025-01-03
**数据库版本**: PostgreSQL 16
**项目**: Crush


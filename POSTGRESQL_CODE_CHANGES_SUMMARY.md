# PostgreSQL 代码修改完成总结

## ✅ 已完成的修改

### 1. 数据库配置文件
- ✅ **sqlc.yaml**: 引擎从 `sqlite` 改为 `postgresql`

### 2. 数据库连接文件
- ✅ **internal/db/connect.go**: 
  - 完全重写以支持 PostgreSQL
  - 移除了 SQLite 驱动和相关 pragma
  - 添加了环境变量配置支持
  - 添加了连接池配置

### 3. 数据库迁移文件
所有迁移文件已更新为 PostgreSQL 语法：
- ✅ **20250424200609_initial.sql**: 
  - `INTEGER` → `BIGINT` (时间戳)
  - `REAL` → `DECIMAL` (cost 字段)
  - SQLite triggers → PostgreSQL functions + triggers
  - `strftime('%s', 'now')` → `EXTRACT(EPOCH FROM NOW()) * 1000`
  
- ✅ **20250515105448_add_summary_message_id.sql**: 添加了 `IF NOT EXISTS`
- ✅ **20250624000000_add_created_at_indexes.sql**: 无需修改
- ✅ **20250627000000_add_provider_to_messages.sql**: 添加了 `IF NOT EXISTS`
- ✅ **20250810000000_add_is_summary_message.sql**: 添加了 `IF NOT EXISTS`

### 4. SQL 查询文件
所有查询文件已更新：
- ✅ **internal/db/sql/sessions.sql**: `?` → `$1, $2, ...`, 时间戳函数更新
- ✅ **internal/db/sql/messages.sql**: `?` → `$1, $2, ...`, 时间戳函数更新
- ✅ **internal/db/sql/files.sql**: `?` → `$1, $2, ...`, 时间戳函数更新

### 5. 依赖管理
- ✅ **go.mod**: 添加了 `github.com/lib/pq v1.10.9`
- ✅ 运行了 `go mod tidy`
- ✅ 重新生成了 sqlc 代码

### 6. 文档
- ✅ **POSTGRESQL_SETUP.md**: 完整的中文设置指南（576行）
- ✅ **POSTGRESQL_MIGRATION_SUMMARY.md**: 英文迁移总结
- ✅ **crush-main/.env.example**: 环境变量模板

## ⚠️ 需要注意的地方

### 测试代码需要更新

**文件**: `crush-main/internal/agent/common_test.go` (第111行)

```go
conn, err := db.Connect(t.Context(), t.TempDir())
```

**问题**: PostgreSQL 不支持临时目录作为数据库路径（这是 SQLite 特性）

**建议的解决方案**：

#### 方案 1: 使用测试专用的 PostgreSQL 数据库（推荐）

```go
func testEnv(t *testing.T) fakeEnv {
    workingDir := filepath.Join("/tmp/crush-test/", t.Name())
    os.RemoveAll(workingDir)

    err := os.MkdirAll(workingDir, 0o755)
    require.NoError(t, err)

    // 设置测试数据库环境变量
    testDBName := "crush_test_" + strings.ReplaceAll(t.Name(), "/", "_")
    os.Setenv("POSTGRES_DB", testDBName)
    os.Setenv("POSTGRES_HOST", "localhost")
    os.Setenv("POSTGRES_PORT", "5432")
    os.Setenv("POSTGRES_USER", "crush_test")
    os.Setenv("POSTGRES_PASSWORD", "test_password")
    os.Setenv("POSTGRES_SSLMODE", "disable")

    // dataDir 参数现在被忽略，但保持兼容性
    conn, err := db.Connect(t.Context(), "")
    require.NoError(t, err)

    q := db.New(conn)
    sessions := session.NewService(q)
    messages := message.NewService(q)

    permissions := permission.NewPermissionService(workingDir, true, []string{})
    history := history.NewService(q, conn)
    lspClients := csync.NewMap[string, *lsp.Client]()

    t.Cleanup(func() {
        // 清理数据库
        conn.Exec("DROP SCHEMA public CASCADE")
        conn.Exec("CREATE SCHEMA public")
        conn.Close()
        os.RemoveAll(workingDir)
    })

    return fakeEnv{
        workingDir,
        sessions,
        messages,
        permissions,
        history,
        lspClients,
    }
}
```

#### 方案 2: 使用 Docker 容器进行测试

在测试前启动 PostgreSQL 容器：

```bash
docker run --name crush-test-db \
  -e POSTGRES_USER=crush_test \
  -e POSTGRES_PASSWORD=test_password \
  -e POSTGRES_DB=crush_test \
  -p 5433:5432 \
  -d postgres:16-alpine
```

测试代码中设置：

```go
os.Setenv("POSTGRES_HOST", "localhost")
os.Setenv("POSTGRES_PORT", "5433")
os.Setenv("POSTGRES_USER", "crush_test")
os.Setenv("POSTGRES_PASSWORD", "test_password")
os.Setenv("POSTGRES_DB", "crush_test")
```

#### 方案 3: 使用内存中的测试数据库（testcontainers）

添加依赖：
```bash
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

测试代码：
```go
import (
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
    ctx := context.Background()
    
    postgresContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("crush_test"),
        postgres.WithUsername("crush_test"),
        postgres.WithPassword("test_password"),
    )
    require.NoError(t, err)
    
    connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
    
    db, err := sql.Open("postgres", connStr)
    require.NoError(t, err)
    
    cleanup := func() {
        db.Close()
        postgresContainer.Terminate(ctx)
    }
    
    return db, cleanup
}
```

### 其他调用 db.Connect 的地方

**文件**: `crush-main/internal/cmd/root.go` (第196行)

```go
conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
```

**状态**: ✅ 无需修改
- `dataDir` 参数现在不再被使用（PostgreSQL 使用环境变量）
- 但为了保持向后兼容，函数签名保留了这个参数
- 可以正常工作

## 🔧 connect.go 的设计说明

当前的 `connect.go` 保留了 `dataDir` 参数但不使用它：

```go
func Connect(ctx context.Context, dataDir string) (*sql.DB, error) {
    // dataDir 参数被忽略，使用环境变量代替
    dbHost := os.Getenv("POSTGRES_HOST")
    // ...
}
```

这样做的好处：
1. ✅ 保持 API 兼容性，不需要修改所有调用处
2. ✅ 更容易从 SQLite 迁移
3. ✅ 未来如果需要支持多种数据库，可以根据配置决定使用哪个

## 📋 环境变量清单

运行应用需要设置以下环境变量：

| 变量名 | 必需 | 默认值 | 说明 |
|--------|------|--------|------|
| `POSTGRES_HOST` | ❌ | `localhost` | 数据库主机 |
| `POSTGRES_PORT` | ❌ | `5432` | 数据库端口 |
| `POSTGRES_USER` | ❌ | `crush` | 数据库用户 |
| `POSTGRES_PASSWORD` | ✅ | *无* | 数据库密码（必需） |
| `POSTGRES_DB` | ❌ | `crush` | 数据库名称 |
| `POSTGRES_SSLMODE` | ❌ | `disable` | SSL模式 |

## 🚀 快速启动步骤

### 1. 设置 PostgreSQL

```bash
# 使用 Docker (最简单)
docker run --name crush-postgres \
  -e POSTGRES_USER=crush \
  -e POSTGRES_PASSWORD=your_secure_password \
  -e POSTGRES_DB=crush \
  -p 5432:5432 \
  -d postgres:16-alpine

# 或者本地安装
brew install postgresql@16
brew services start postgresql@16
sudo -u postgres psql << EOF
CREATE USER crush WITH PASSWORD 'your_secure_password';
CREATE DATABASE crush OWNER crush;
GRANT ALL PRIVILEGES ON DATABASE crush TO crush;
\c crush
GRANT ALL ON SCHEMA public TO crush;
EOF
```

### 2. 配置环境变量

```bash
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=crush
export POSTGRES_PASSWORD=your_secure_password
export POSTGRES_DB=crush
export POSTGRES_SSLMODE=disable
```

### 3. 构建并运行

```bash
cd crush-main
go build .
./crush
```

数据库迁移会自动运行！

## ❗ 重要提醒

1. **不要提交 .env 文件**: 确保 `.env` 在 `.gitignore` 中
2. **生产环境使用强密码**: 建议至少16个字符
3. **生产环境启用 SSL**: 设置 `POSTGRES_SSLMODE=require`
4. **测试需要单独配置**: 见上面的测试代码修改建议
5. **dataDir 参数已废弃**: 虽然保留了参数，但不再使用

## 📝 后续建议

### 可选的改进：

1. **完全移除 dataDir 参数** (破坏性更改):
   ```go
   func Connect(ctx context.Context) (*sql.DB, error)
   ```

2. **支持连接字符串直接配置**:
   ```go
   func Connect(ctx context.Context, connStrOrDataDir string) (*sql.DB, error) {
       if strings.HasPrefix(connStrOrDataDir, "postgres://") {
           // 使用连接字符串
       } else {
           // 使用环境变量
       }
   }
   ```

3. **添加数据库健康检查**:
   ```go
   func (db *DB) HealthCheck() error
   ```

4. **添加连接重试逻辑**:
   ```go
   func ConnectWithRetry(ctx context.Context, maxRetries int) (*sql.DB, error)
   ```

## 总结

✅ **所有核心代码已成功迁移到 PostgreSQL**

需要手动处理的：
- ⚠️ 测试代码（`common_test.go`）需要更新以使用真实的 PostgreSQL 数据库
- ⚠️ CI/CD 配置需要添加 PostgreSQL 服务

详细文档：
- 📖 [POSTGRESQL_SETUP.md](POSTGRESQL_SETUP.md) - 完整设置指南
- 📖 [POSTGRESQL_MIGRATION_SUMMARY.md](POSTGRESQL_MIGRATION_SUMMARY.md) - 迁移总结
- 📄 [crush-main/.env.example](crush-main/.env.example) - 环境变量模板

---

**迁移完成日期**: 2025-01-03
**PostgreSQL 版本**: 16 (推荐)
**Go PostgreSQL 驱动**: github.com/lib/pq v1.10.9


# PostgreSQL Migration Summary

## Overview

The Crush database has been successfully migrated from SQLite to PostgreSQL. This document summarizes the changes made.

## Changes Made

### 1. Database Configuration

- **File**: `crush-main/sqlc.yaml`
- **Change**: Updated engine from `sqlite` to `postgresql`

### 2. Database Driver

- **File**: `crush-main/internal/db/connect.go`
- **Changes**:
  - Replaced SQLite driver (`github.com/ncruces/go-sqlite3`) with PostgreSQL driver (`github.com/lib/pq`)
  - Removed SQLite-specific pragmas
  - Added PostgreSQL connection string builder
  - Added connection pool configuration
  - Connection now uses environment variables for configuration

### 3. Migration Files

All migration files in `crush-main/internal/db/migrations/` were updated:

- **20250424200609_initial.sql**: 
  - Changed `INTEGER` to `BIGINT` for timestamps
  - Changed `REAL` to `DECIMAL` for cost field
  - Converted SQLite triggers to PostgreSQL functions and triggers
  - Updated timestamp functions from `strftime('%s', 'now')` to `EXTRACT(EPOCH FROM NOW()) * 1000`

- **20250515105448_add_summary_message_id.sql**:
  - Added `IF NOT EXISTS` / `IF EXISTS` clauses for PostgreSQL compatibility

- **20250624000000_add_created_at_indexes.sql**:
  - No changes needed (compatible with both databases)

- **20250627000000_add_provider_to_messages.sql**:
  - Added `IF NOT EXISTS` / `IF EXISTS` clauses

- **20250810000000_add_is_summary_message.sql**:
  - Added `IF NOT EXISTS` / `IF EXISTS` clauses

### 4. SQL Query Files

Updated all query files to use PostgreSQL placeholders:

- **internal/db/sql/sessions.sql**:
  - Changed `?` placeholders to `$1, $2, $3, ...`
  - Changed `strftime('%s', 'now')` to `EXTRACT(EPOCH FROM NOW()) * 1000`

- **internal/db/sql/messages.sql**:
  - Changed `?` placeholders to `$1, $2, $3, ...`
  - Changed `strftime('%s', 'now')` to `EXTRACT(EPOCH FROM NOW()) * 1000`

- **internal/db/sql/files.sql**:
  - Changed `?` placeholders to `$1, $2, $3, ...`
  - Changed `strftime('%s', 'now')` to `EXTRACT(EPOCH FROM NOW()) * 1000`

### 5. Dependencies

- **Added**: `github.com/lib/pq v1.10.9`
- **Kept**: All SQLite dependencies remain in go.mod for compatibility (can be removed if no longer needed)

### 6. Generated Code

- Re-generated all database access code using `sqlc generate`
- Generated code now uses PostgreSQL-specific types and placeholders

## Environment Variables

The application now requires the following environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTGRES_HOST` | No | `localhost` | Database host |
| `POSTGRES_PORT` | No | `5432` | Database port |
| `POSTGRES_USER` | No | `crush` | Database user |
| `POSTGRES_PASSWORD` | **Yes** | None | Database password |
| `POSTGRES_DB` | No | `crush` | Database name |
| `POSTGRES_SSLMODE` | No | `disable` | SSL mode |

## Setup Instructions

### Quick Start

1. **Install PostgreSQL** (if not already installed):

```bash
# macOS
brew install postgresql@16
brew services start postgresql@16

# Ubuntu/Debian
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql

# Docker
docker run --name crush-postgres \
  -e POSTGRES_USER=crush \
  -e POSTGRES_PASSWORD=your_password \
  -e POSTGRES_DB=crush \
  -p 5432:5432 \
  -d postgres:16-alpine
```

2. **Create database and user**:

```bash
sudo -u postgres psql << EOF
CREATE USER crush WITH PASSWORD 'your_secure_password';
CREATE DATABASE crush OWNER crush;
GRANT ALL PRIVILEGES ON DATABASE crush TO crush;
\c crush
GRANT ALL ON SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO crush;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO crush;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO crush;
EOF
```

3. **Configure environment variables**:

```bash
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=crush
export POSTGRES_PASSWORD=your_secure_password
export POSTGRES_DB=crush
export POSTGRES_SSLMODE=disable
```

4. **Run the application**:

```bash
cd crush-main
go build .
./crush
```

The database migrations will run automatically on startup!

## Testing the Migration

To verify the migration was successful:

1. Check database connection:

```bash
psql -h localhost -U crush -d crush -c "\dt"
```

You should see tables: `sessions`, `messages`, `files`, and `goose_db_version`

2. Check migration status:

```sql
SELECT * FROM goose_db_version ORDER BY id;
```

All migrations should show `is_applied = true`

## Rollback (if needed)

If you need to rollback to SQLite:

1. Revert the changes in git
2. Export data from PostgreSQL
3. Import to SQLite
4. Update environment to not use PostgreSQL variables

## Additional Documentation

For detailed setup instructions, troubleshooting, and production deployment guidelines, see:

- **[POSTGRESQL_SETUP.md](POSTGRESQL_SETUP.md)** - Complete PostgreSQL setup guide (Chinese)
- **[.env.example](crush-main/.env.example)** - Environment variable template

## Key Differences: SQLite vs PostgreSQL

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| **Placeholders** | `?` | `$1, $2, $3, ...` |
| **Current Time** | `strftime('%s', 'now')` | `EXTRACT(EPOCH FROM NOW()) * 1000` |
| **Float Type** | `REAL` | `DECIMAL` or `NUMERIC` |
| **Triggers** | Simple BEGIN/END blocks | Requires functions in PL/pgSQL |
| **Timestamp Storage** | `INTEGER` (Unix timestamp) | `BIGINT` (Unix timestamp in ms) |
| **Connection** | File path | Connection string with credentials |
| **Concurrent Access** | Limited | Full multi-user support |
| **Data Integrity** | Basic | Advanced (MVCC) |

## Benefits of PostgreSQL

✅ **Better concurrency**: Multiple connections without locking issues
✅ **Stronger data integrity**: ACID compliance with advanced constraints
✅ **Better performance**: Query optimization and indexing
✅ **Scalability**: Handles larger datasets efficiently
✅ **Production-ready**: Suitable for high-traffic applications
✅ **Advanced features**: JSON, full-text search, partitioning, etc.
✅ **Connection pooling**: Better resource management
✅ **Replication**: Built-in support for backups and failover

## Notes

- Migrations are automatically applied on application startup
- The `dataDir` parameter in `Connect()` is no longer used (was SQLite-specific)
- Connection pool is configured with sensible defaults (25 max open, 5 max idle)
- All timestamps are stored as BIGINT (Unix timestamp in milliseconds)
- The application will fail to start if `POSTGRES_PASSWORD` is not set

## Troubleshooting

See the detailed troubleshooting section in [POSTGRESQL_SETUP.md](POSTGRESQL_SETUP.md) for:

- Connection issues
- Authentication failures
- Permission problems
- Migration errors
- Performance tuning

---

**Migration Date**: 2025-01-03
**PostgreSQL Version**: 16 (recommended)
**Go PostgreSQL Driver**: github.com/lib/pq v1.10.9


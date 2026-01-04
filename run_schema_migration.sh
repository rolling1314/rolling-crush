#!/bin/bash

echo "=========================================="
echo "  PostgreSQL Schema 应用工具"
echo "=========================================="
echo ""
echo "请确保 PostgreSQL 数据库正在运行"
echo ""

# 检查 apply_schema.sql 是否存在
if [ ! -f "/Users/apple/crush/apply_schema.sql" ]; then
    echo "❌ 错误: apply_schema.sql 文件不存在"
    exit 1
fi

# 提示用户输入连接信息
read -p "PostgreSQL 主机 (默认: localhost): " PG_HOST
PG_HOST=${PG_HOST:-localhost}

read -p "PostgreSQL 端口 (默认: 5432): " PG_PORT
PG_PORT=${PG_PORT:-5432}

read -p "数据库用户名 (默认: crushuser): " PG_USER
PG_USER=${PG_USER:-crushuser}

read -p "数据库名称 (默认: crushdb): " PG_DB
PG_DB=${PG_DB:-crushdb}

echo ""
echo "正在连接到 PostgreSQL: $PG_USER@$PG_HOST:$PG_PORT/$PG_DB"
echo ""

# 应用 schema
PGPASSWORD=$POSTGRES_PASSWORD psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -f /Users/apple/crush/apply_schema.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Schema 应用成功！"
    echo ""
    echo "接下来运行："
    echo "  cd /Users/apple/crush/crush-main"
    echo "  \$HOME/go/bin/sqlc generate"
    echo "  go build ."
else
    echo ""
    echo "❌ Schema 应用失败"
    echo ""
    echo "请检查数据库连接信息和权限"
fi


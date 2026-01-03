# Docker PostgreSQL 命令

## 启动 PostgreSQL 容器

```bash
docker run --name crush-postgres \
  -e POSTGRES_USER=crush \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=crush \
  -p 5432:5432 \
  -v crush-postgres-data:/var/lib/postgresql/data \
  -d postgres:16-alpine
```

## 常用管理命令

```bash
# 查看容器状态
docker ps -f name=crush-postgres

# 停止容器
docker stop crush-postgres

# 启动已存在的容器
docker start crush-postgres

# 重启容器
docker restart crush-postgres

# 查看日志
docker logs crush-postgres

# 进入数据库
docker exec -it crush-postgres psql -U crush -d crush

# 删除容器（会保留数据卷）
docker rm -f crush-postgres

# 删除容器和数据（危险！会删除所有数据）
docker rm -f crush-postgres
docker volume rm crush-postgres-data
```

## 连接信息

- **主机**: localhost
- **端口**: 5432
- **数据库**: crush
- **用户名**: crush
- **密码**: 123456
- **连接字符串**: `postgres://crush:123456@localhost:5432/crush?sslmode=disable`

## 快速启动 Crush

```bash
# 1. 启动数据库（如果未启动）
docker start crush-postgres || docker run --name crush-postgres \
  -e POSTGRES_USER=crush \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=crush \
  -p 5432:5432 \
  -v crush-postgres-data:/var/lib/postgresql/data \
  -d postgres:16-alpine

# 2. 等待几秒让数据库启动
sleep 3

# 3. 启动 Crush
cd crush-main
go run .
```

代码已默认使用用户名 `crush` 和密码 `123456`，无需设置环境变量。


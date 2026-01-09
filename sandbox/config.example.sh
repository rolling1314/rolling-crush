#!/bin/bash
# PostgreSQL 数据库配置示例（与 crush-main 保持一致）
# 复制此文件为 config.sh 并修改配置，然后 source config.sh

export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=crush
export POSTGRES_PASSWORD=123456
export POSTGRES_DB=crush
export POSTGRES_SSLMODE=disable

# 沙箱服务配置
export SANDBOX_HOST=0.0.0.0
export SANDBOX_PORT=8888

# 测试配置（可选）
export TEST_SESSION_ID=test-session-123
# export REAL_SESSION_ID=<真实会话ID>

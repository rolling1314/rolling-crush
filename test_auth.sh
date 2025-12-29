#!/bin/bash

# Crush JWT 认证系统测试脚本

echo "🧪 Crush JWT 认证系统测试"
echo "================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试计数器
PASSED=0
FAILED=0

# 测试函数
test_endpoint() {
    local name=$1
    local url=$2
    local method=$3
    local data=$4
    local expected_code=$5
    
    echo -n "测试: $name ... "
    
    if [ "$method" == "POST" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST "$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" "$url" 2>/dev/null)
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" == "$expected_code" ]; then
        echo -e "${GREEN}✓ 通过${NC} (HTTP $http_code)"
        PASSED=$((PASSED + 1))
        if [ ! -z "$body" ]; then
            echo "   响应: $body" | head -c 100
            echo ""
        fi
        return 0
    else
        echo -e "${RED}✗ 失败${NC} (期望 $expected_code, 实际 $http_code)"
        FAILED=$((FAILED + 1))
        if [ ! -z "$body" ]; then
            echo "   响应: $body"
        fi
        return 1
    fi
}

echo "1️⃣  检查服务器状态"
echo "-------------------"

# 测试 HTTP 服务器健康检查
test_endpoint "HTTP 服务器健康检查" \
    "http://localhost:8081/health" \
    "GET" \
    "" \
    "200"

echo ""
echo "2️⃣  测试登录功能"
echo "-------------------"

# 测试成功登录
echo -n "测试: 管理员登录 ... "
login_response=$(curl -s -X POST http://localhost:8081/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

if echo "$login_response" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
    
    # 提取 token
    TOKEN=$(echo "$login_response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    if [ ! -z "$TOKEN" ]; then
        echo "   Token 已获取: ${TOKEN:0:30}..."
    fi
else
    echo -e "${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
    echo "   响应: $login_response"
fi

# 测试错误的密码
echo -n "测试: 错误密码登录 ... "
error_response=$(curl -s -X POST http://localhost:8081/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"wrongpassword"}' 2>/dev/null)

if echo "$error_response" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ 通过${NC} (正确拒绝)"
    PASSED=$((PASSED + 1))
else
    echo -e "${RED}✗ 失败${NC} (应该拒绝错误密码)"
    FAILED=$((FAILED + 1))
fi

# 测试不存在的用户
echo -n "测试: 不存在的用户 ... "
notfound_response=$(curl -s -X POST http://localhost:8081/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"nonexistent","password":"password"}' 2>/dev/null)

if echo "$notfound_response" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ 通过${NC} (正确拒绝)"
    PASSED=$((PASSED + 1))
else
    echo -e "${RED}✗ 失败${NC} (应该拒绝不存在的用户)"
    FAILED=$((FAILED + 1))
fi

echo ""
echo "3️⃣  测试 Token 验证"
echo "-------------------"

if [ ! -z "$TOKEN" ]; then
    # 测试有效 token
    echo -n "测试: 有效 Token 验证 ... "
    verify_response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $TOKEN" \
        http://localhost:8081/api/auth/verify 2>/dev/null)
    
    http_code=$(echo "$verify_response" | tail -n1)
    if [ "$http_code" == "200" ]; then
        echo -e "${GREEN}✓ 通过${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗ 失败${NC} (HTTP $http_code)"
        FAILED=$((FAILED + 1))
    fi
    
    # 测试无效 token
    echo -n "测试: 无效 Token 验证 ... "
    invalid_response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer invalid_token_here" \
        http://localhost:8081/api/auth/verify 2>/dev/null)
    
    http_code=$(echo "$invalid_response" | tail -n1)
    if [ "$http_code" == "401" ]; then
        echo -e "${GREEN}✓ 通过${NC} (正确拒绝)"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗ 失败${NC} (应该返回 401)"
        FAILED=$((FAILED + 1))
    fi
    
    # 测试没有 token
    echo -n "测试: 缺少 Token ... "
    notoken_response=$(curl -s -w "\n%{http_code}" \
        http://localhost:8081/api/auth/verify 2>/dev/null)
    
    http_code=$(echo "$notoken_response" | tail -n1)
    if [ "$http_code" == "401" ]; then
        echo -e "${GREEN}✓ 通过${NC} (正确拒绝)"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗ 失败${NC} (应该返回 401)"
        FAILED=$((FAILED + 1))
    fi
else
    echo -e "${YELLOW}⚠ 跳过 Token 验证测试（未获取到 Token）${NC}"
fi

echo ""
echo "4️⃣  测试 WebSocket 连接"
echo "-------------------"

if [ ! -z "$TOKEN" ]; then
    echo -n "测试: WebSocket 连接（带有效 Token）... "
    
    # 使用 websocat 或 wscat 测试 WebSocket（如果安装了）
    if command -v websocat &> /dev/null; then
        # 尝试连接 WebSocket（超时 2 秒）
        timeout 2 websocat "ws://localhost:8080/ws?token=$TOKEN" <<< '{"content":"test"}' &> /dev/null
        if [ $? -eq 124 ] || [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ 通过${NC} (连接成功)"
            PASSED=$((PASSED + 1))
        else
            echo -e "${RED}✗ 失败${NC}"
            FAILED=$((FAILED + 1))
        fi
    elif command -v wscat &> /dev/null; then
        # 尝试使用 wscat
        timeout 2 wscat -c "ws://localhost:8080/ws?token=$TOKEN" &> /dev/null
        if [ $? -eq 124 ] || [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ 通过${NC} (连接成功)"
            PASSED=$((PASSED + 1))
        else
            echo -e "${RED}✗ 失败${NC}"
            FAILED=$((FAILED + 1))
        fi
    else
        echo -e "${YELLOW}⚠ 跳过${NC} (需要安装 websocat 或 wscat)"
        echo "   安装: brew install websocat  或  npm install -g wscat"
    fi
else
    echo -e "${YELLOW}⚠ 跳过 WebSocket 测试（未获取到 Token）${NC}"
fi

echo ""
echo "================================"
echo "📊 测试结果汇总"
echo "================================"
echo -e "通过: ${GREEN}$PASSED${NC}"
echo -e "失败: ${RED}$FAILED${NC}"
echo -e "总计: $((PASSED + FAILED))"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}🎉 所有测试通过！系统运行正常。${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}❌ 有测试失败。请检查服务器日志。${NC}"
    echo ""
    echo "故障排查提示："
    echo "1. 确认后端服务器正在运行: go run main.go"
    echo "2. 检查端口 8080 和 8081 是否被占用"
    echo "3. 查看后端日志输出"
    exit 1
fi


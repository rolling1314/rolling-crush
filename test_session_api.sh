#!/bin/bash

# Test script to verify session API returns context_window

echo "=== Testing Session API Response ==="
echo ""

# Replace with your actual project ID and JWT token
PROJECT_ID="${1:-your-project-id}"
JWT_TOKEN="${2:-your-jwt-token}"

if [ "$PROJECT_ID" = "your-project-id" ] || [ "$JWT_TOKEN" = "your-jwt-token" ]; then
    echo "Usage: $0 <project_id> <jwt_token>"
    echo ""
    echo "Example: $0 abc123 eyJhbGc..."
    exit 1
fi

echo "Testing: GET /api/projects/$PROJECT_ID/sessions"
echo "---"

curl -s -H "Authorization: Bearer $JWT_TOKEN" \
    "http://localhost:8080/api/projects/$PROJECT_ID/sessions" | jq '.'

echo ""
echo "=== Check if context_window field exists in the response ==="
echo "If you see context_window: 0, it means the session doesn't have model config yet"
echo "If you see context_window: <number>, the API is working correctly"

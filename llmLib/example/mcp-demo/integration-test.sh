#!/bin/bash
# MCP 集成测试脚本

set -e

cd "$(dirname "$0")/../.."

echo "=== MCP 集成测试 ==="
echo ""

# 1. 构建
echo "步骤 1: 构建..."
go build -o /tmp/mcp-server example/mcp-demo/server/main.go
go build -o /tmp/mcp-integration example/mcp-demo/simple-integration.go
echo "✓ 构建完成"
echo ""

# 2. 启动 Server（后台）
echo "步骤 2: 启动 Server..."
/tmp/mcp-server &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"
sleep 2  # 等待 Server 启动

# 3. 运行集成示例
echo ""
echo "步骤 3: 运行集成示例..."
timeout 5 /tmp/mcp-integration 2>&1 || true

# 4. 清理
echo ""
echo "步骤 4: 清理..."
kill $SERVER_PID 2>/dev/null || true
rm -f /tmp/mcp-server /tmp/mcp-integration
echo "✓ 清理完成"

echo ""
echo "=== 测试完成 ==="

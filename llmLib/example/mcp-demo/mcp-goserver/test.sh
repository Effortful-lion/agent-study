#!/bin/bash

# 测试 mcp-go Server
# 与手写版进行对比测试

set -e

echo "========================================"
echo "  mcp-go Server 测试"
echo "========================================"
echo ""

# 设置变量
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 编译 Server
echo "1. 编译 Server..."
go build -o /tmp/mcpgo-server .
echo "   ✓ 编译成功"
echo ""

# 测试 initialize
echo "2. 测试 initialize 方法..."
REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"mcpgo-test","version":"1.0.0"},"capabilities":{"roots":{"listChanged":true}}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/mcpgo-server 2>/dev/null)
echo "   响应: $RESPONSE"
echo ""

# 测试 tools/list
echo "3. 测试 tools/list 方法..."
REQUEST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":null}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/mcpgo-server 2>/dev/null)
echo "   响应: $RESPONSE"
echo ""

# 验证返回的工具
TOOL_COUNT=$(echo "$RESPONSE" | grep -o '"name":"[^"]*"' | wc -l | tr -d ' ')
echo "   ✓ tools/list 返回了 $TOOL_COUNT 个工具"
echo ""

# 测试 tools/call (get_time)
echo "4. 测试 tools/call 方法 (get_time)..."
REQUEST='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_time","arguments":{"timezone":"Asia/Shanghai"}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/mcpgo-server 2>/dev/null)
echo "   响应: $RESPONSE"
echo ""

# 测试 tools/call (calc)
echo "5. 测试 tools/call 方法 (calc)..."
REQUEST='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"calc","arguments":{"expr":"2+3"}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/mcpgo-server 2>/dev/null)
echo "   响应: $RESPONSE"
echo ""

echo "========================================"
echo "  mcp-go Server 测试完成"
echo "========================================"
echo ""

echo "下一步："
echo "  1. 使用 MCP Inspector 测试："
echo "     npx @modelcontextprotocol/inspector go run main.go"
echo ""
echo "  2. 对比手写版和 mcp-go 版："
echo "     cat COMPARISON.md"
echo ""

# 清理
rm -f /tmp/mcpgo-server

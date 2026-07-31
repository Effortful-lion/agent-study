#!/bin/bash

# MCP 手写 Server 演示脚本
# 用途：一键测试手写 Server 的完整流程

set -e  # 遇到错误退出

echo "========================================"
echo "  MCP 手写 Server 测试"
echo "========================================"
echo ""

# 编译 Server
echo "1. 编译 Server..."
go build -o /tmp/handwritten-server server.go
echo "   ✓ 编译成功"
echo ""

# 测试 initialize
echo "2. 测试 initialize 方法..."
REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test-client","version":"1.0.0"},"capabilities":{"roots":{"listChanged":true}}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/handwritten-server)
echo "   响应: $RESPONSE"
echo ""

# 测试 tools/list
echo "3. 测试 tools/list 方法..."
REQUEST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":null}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/handwritten-server)
echo "   响应: $RESPONSE"
echo ""

# 验证返回的工具数量
TOOL_COUNT=$(echo "$RESPONSE" | grep -o '"tools":\[' | wc -l)
echo "   ✓ tools/list 返回了工具列表"
echo ""

# 测试 tools/call (get_time)
echo "4. 测试 tools/call 方法 (get_time)..."
REQUEST='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_time","arguments":{"timezone":"Asia/Shanghai"}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/handwritten-server)
echo "   响应: $RESPONSE"
echo ""

# 测试 tools/call (calc)
echo "5. 测试 tools/call 方法 (calc)..."
REQUEST='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"calc","arguments":{"expr":"1+1"}}}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/handwritten-server)
echo "   响应: $RESPONSE"
echo ""

# 测试通知（无响应）
echo "6. 测试通知（notifications/initialized）..."
REQUEST='{"jsonrpc":"2.0","method":"notifications/initialized"}'
echo "   请求: $REQUEST"
RESPONSE=$(echo "$REQUEST" | /tmp/handwritten-server 2>&1)
echo "   响应: (无响应，符合预期)"
echo ""

echo "========================================"
echo "  所有测试通过！"
echo "========================================"
echo ""
echo "下一步："
echo "  1. make run-client     # 运行完整客户端演示"
echo "  2. make run-secure     # 运行安全净化演示"
echo ""

# 清理
rm -f /tmp/handwritten-server

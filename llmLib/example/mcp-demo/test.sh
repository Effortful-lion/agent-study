#!/bin/bash
# MCP 演示测试脚本

set -e

echo "=== MCP 演示测试 ==="
echo ""

# 检查参数
if [ "$1" == "build" ]; then
    echo "步骤 1: 构建所有组件..."
    cd "$(dirname "$0")/../.."

    go build -o bin/server example/mcp-demo/server/main.go
    echo "✓ Server 构建成功: bin/server"

    go build -o bin/client example/mcp-demo/client/main.go
    echo "✓ Client 构建成功: bin/client"

    go build -o bin/integration example/mcp-demo/simple-integration.go
    echo "✓ 集成示例构建成功: bin/integration"

    echo ""
    echo "运行方式:"
    echo "  ./bin/server              # 启动 Server"
    echo "  ./bin/client ./bin/server # 启动 Client"
    echo "  ./bin/integration         # 运行集成示例"

elif [ "$1" == "test" ]; then
    echo "步骤 1: 构建..."
    cd "$(dirname "$0")/../.."
    go build ./mcp/... > /dev/null
    go build ./example/mcp-demo/... > /dev/null
    echo "✓ 构建成功"

    echo ""
    echo "步骤 2: 运行集成示例..."
    go run example/mcp-demo/simple-integration.go

elif [ "$1" == "server" ]; then
    echo "启动 Server..."
    cd "$(dirname "$0")/../.."
    go run example/mcp-demo/server/main.go

elif [ "$1" == "client" ]; then
    if [ -z "$2" ]; then
        echo "用法: $0 client <server-command>"
        echo "示例: $0 client 'go run server/main.go'"
        exit 1
    fi
    echo "启动 Client..."
    cd "$(dirname "$0")/../.."
    go run example/mcp-demo/client/main.go $2

else
    echo "用法: $0 {build|test|server|client}"
    echo ""
    echo "命令:"
    echo "  build              - 构建所有组件"
    echo "  test               - 运行集成测试"
    echo "  server             - 启动 Server"
    echo "  client <command>   - 启动 Client"
    echo ""
    echo "示例:"
    echo "  $0 build"
    echo "  $0 test"
    echo "  $0 server"
    echo "  $0 client 'go run server/main.go'"
    exit 1
fi

# MCP 手写 Server 练习

# 编译手写 Server
build-server:
	go build -o server server.go

# 运行客户端演示
run-client:
	go run client.go

# 运行安全演示
run-secure:
	go run secure_demo.go

# 启动手写 Server（前台）
start-server:
	go run server.go

# 启动手写 Server（后台）
start-server-bg:
	go run server.go > server.log 2>&1 &
	@echo "Server PID: $$!"

# 停止 Server
stop-server:
	@pkill -f "go run server.go" || true

# 测试完整流程
test: build-server
	@echo "=== 测试手写 Server ==="
	@echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"test","version":"1.0.0"},"capabilities":{}}}' | ./server
	@echo ""
	@echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":null}' | ./server
	@echo ""
	@echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_time","arguments":{"timezone":"UTC"}}}' | ./server

# 代码检查
lint:
	gofmt -d server.go client.go secure_demo.go
	go vet server.go client.go secure_demo.go

# 清理
clean:
	rm -f server
	rm -f *.log

# 帮助
help:
	@echo "可用的 make 命令："
	@echo "  make build-server   - 编译手写 Server"
	@echo "  make start-server   - 启动手写 Server（前台）"
	@echo "  make run-client     - 运行客户端完整演示"
	@echo "  make run-secure     - 运行安全净化演示"
	@echo "  make test           - 测试手写 Server"
	@echo "  make lint           - 代码检查和格式化"
	@echo "  make clean          - 清理生成的文件"
	@echo ""
	@echo "使用步骤："
	@echo "  1. make start-server     # 终端 1：启动 Server"
	@echo "  2. make run-client       # 终端 2：运行完整演示"
	@echo "  3. make run-secure       # 终端 2：运行安全演示"
	@echo ""
	@echo "或者使用测试命令："
	@echo "  make test               # 一键测试 Server"

.PHONY: build-server run-client run-secure start-server start-server-bg stop-server test lint clean help

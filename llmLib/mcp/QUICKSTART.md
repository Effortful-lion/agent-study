# MCP 快速开始

## 从零实现 MCP stdio Server 和 Client

本目录包含完整的 MCP（Model Context Protocol）实现，从零手写 Server、Client 和 Bridge。

## 文件结构

```
mcp/
├── types.go              # MCP 核心类型定义
├── server.go             # MCP stdio Server 实现
├── client.go             # MCP stdio Client 实现
├── bridge.go             # MCPToolBridge 实现
├── mcp_test.go           # 测试文件
├── README.md             # 详细文档
└── SUMMARY.md            # 实现总结

example/mcp-demo/
├── server/main.go        # 演示 Server
├── client/main.go        # 演示 Client
└── simple-integration.go # 简化集成示例
```

## 5 分钟快速开始

### 1. 启动 Server

```bash
go run example/mcp-demo/server/main.go
```

Server 会暴露两个工具：
- `get_time` - 获取当前时间
- `calc` - 计算算术表达式

### 2. 测试 Client（另一个终端）

```bash
go run example/mcp-demo/client/main.go go run server/main.go
```

这会演示完整的 MCP 协议流程：
- 握手（initialize）
- 列出工具（tools/list）
- 调用工具（tools/call）
- 原始消息流展示
- 工具桥接演示

### 3. 集成到 Agent

```bash
go run example/mcp-demo/simple-integration.go
```

这会展示如何将 MCP 工具桥接到 Agent：
- 一键启动并桥接
- 注册到 Registry
- 显示工具定义
- 直接测试工具调用

## 构建

如果需要编译成二进制文件：

```bash
# 构建 Server
go build -o bin/server example/mcp-demo/server/main.go

# 构建 Client
go build -o bin/client example/mcp-demo/client/main.go

# 构建集成示例
go build -o bin/integration example/mcp-demo/simple-integration.go

# 运行
./bin/server
./bin/client go run server/main.go
./bin/integration
```

## 核心概念

### Server 端

```go
import "github.com/Effortful-lion/agent-study/llmLib/mcp"
import "github.com/Effortful-lion/agent-study/llmLib/tool"

// 创建 Server
server := mcp.NewServer("my-server", "1.0.0")

// 注册工具
server.AddTool(tool.NewJSONSchemaTool(
    "echo",
    "回显工具",
    []byte(`{"type":"object","properties":{"message":{"type":"string"}}}`),
    func(ctx context.Context, args map[string]any) (any, error) {
        return args["message"], nil
    },
))

// 启动 Server（stdio 模式）
server.Serve()
```

### Client 端

```go
import "github.com/Effortful-lion/agent-study/llmLib/mcp"

// 启动 Client（自动启动 Server 子进程）
client, err := mcp.NewClient("go", []string{"run", "server/main.go"})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 握手
_, err = client.Initialize(context.Background())
client.Initialized(context.Background())

// 列出工具
tools, err := client.ListTools(context.Background())

// 调用工具
result, err := client.CallTool(context.Background(), "echo",
    []byte(`{"message":"hello"}`))
```

### Bridge 到 Agent

```go
// 一键桥接所有工具
client, mcpTools, err := mcp.NewBridgedClient("go", []string{"run", "server/main.go"})
defer client.Close()

// 注册到 Registry
registry := tool.NewRegistryToolSet()
registry.Register(mcpTools...)

// 在 Agent 中使用
agent := agent.New(provider, model, registry)
events, _ := agent.Run(ctx, "查询当前时间")
```

## 核心特性

### ✅ Server 特性

- [x] stdio 传输（stdin/stdout）
- [x] 自动日志到 stderr
- [x] 工具注册和管理
- [x] 完整的 JSON-RPC 2.0 处理

### ✅ Client 特性

- [x] 自动启动 Server 子进程
- [x] 完整的握手流程
- [x] 请求/响应 ID 匹配
- [x] 防御性容错（跳过非 JSON 消息）

### ✅ Bridge 特性

- [x] 一键桥接所有工具
- [x] 桥接单个工具
- [x] 实现 `tool.Tool` 接口
- [x] 支持 `tool.SchemaTool` 接口
- [x] 自动参数转换

## 测试

```bash
# 构建所有包
go build ./mcp/...
go build ./example/mcp-demo/...

# 运行测试
go test ./mcp/... -v
```

## 下一步

- ✅ 完成 Server/Client/Bridge 实现
- ✅ 实现三个核心方法（initialize, tools/list, tools/call）
- ✅ 完成工具桥接
- ⬜ 添加并发请求支持
- ⬜ 添加资源（resources）支持
- ⬜ 添加提示词（prompts）支持
- ⬜ 实现 Streamable HTTP 传输

## 文档

- **[README.md](README.md)** - 完整教程和 API 文档
- **[SUMMARY.md](SUMMARY.md)** - 实现总结和设计决策
- **[../docs/MCP-DEEP-DIVE.md](../docs/MCP-DEEP-DIVE.md)** - MCP 协议深度解析

# MCP 从零实现总结

## 完成的工作

### 1. MCP 核心包 (`/Users/lion/mycode/agent-study/llmLib/mcp/`)

#### types.go - 类型定义
- ✅ JSON-RPC 2.0 基础类型（Request/Response/Error）
- ✅ MCP 核心类型（Initialize/Tools/Resources/Prompts/Logging）
- ✅ 辅助类型和函数

#### server.go - Server 实现
- ✅ 完整的 stdio MCP Server
- ✅ 工具注册和管理
- ✅ 三个核心方法：initialize、tools/list、tools/call
- ✅ JSON-RPC 消息处理
- ✅ 通知处理
- ✅ ServeWithIO 支持自定义 IO

#### client.go - Client 实现
- ✅ stdio MCP Client
- ✅ 自动启动 Server 子进程
- ✅ 串行化请求（简化版）
- ✅ 五个核心方法：Initialize/Initialized/ListTools/CallTool/Close
- ✅ stderr 透传

#### bridge.go - 工具桥接
- ✅ bridgedTool 实现 tool.Tool 接口
- ✅ BridgeAll() - 一键桥接所有工具
- ✅ BridgeTool() - 桥接单个工具
- ✅ NewBridgedClient() - 启动并桥接

### 2. 示例代码

#### server/main.go - 演示 Server
- ✅ 创建 Server
- ✅ 注册 get_time 和 calc 工具
- ✅ 启动服务

#### client/main.go - 演示 Client
- ✅ 完整的握手流程
- ✅ 列出工具
- ✅ 调用工具
- ✅ 展示原始消息流
- ✅ 演示工具桥接

#### simple-integration.go - 简化集成
- ✅ 一键启动并桥接
- ✅ 注册到 Registry
- ✅ 显示工具定义
- ✅ 直接测试调用

### 3. 测试

#### mcp_test.go
- ✅ JSON-RPC 消息格式测试
- ✅ 工具定义测试
- ✅ 桥接接口测试
- ✅ 辅助函数测试

## 核心流程

### Server 端

```go
// 1. 创建 Server
server := mcp.NewServer("demo", "1.0.0")

// 2. 注册工具
server.AddTool(myTool)

// 3. 启动服务
server.Serve()  // 从 stdin 读，向 stdout 写
```

**处理流程：**
```
读取 stdin
  ↓
解析 JSON-RPC
  ↓
根据 method 分发
  ├─ initialize → 返回 Server 能力
  ├─ tools/list → 返回工具列表
  └─ tools/call → 执行工具，返回结果
  ↓
向 stdout 写入 JSON-RPC 响应
```

### Client 端

```go
// 1. 启动 Client（启动 Server 子进程）
client, err := mcp.NewClient("go", []string{"run", "server/main.go"})

// 2. 握手
client.Initialize(ctx)
client.Initialized(ctx)

// 3. 列出工具
tools, _ := client.ListTools(ctx)

// 4. 调用工具
result, _ := client.CallTool(ctx, "tool_name", args)

// 5. 关闭
client.Close()
```

### Bridge 端

```go
// 一键桥接
client, tools, err := mcp.NewBridgedClient("go", []string{"run", "server/main.go"})

// 工具已经是 tool.Tool 接口
// 可以直接注册到 Registry
registry := tool.NewRegistryToolSet()
registry.Register(tools...)

// Agent 无需感知 MCP 细节
agent := agent.New(provider, model, registry)
```

## 关键设计决策

### 1. 为什么使用指针 *int 作为 ID？

```go
type rpcRequest struct {
    ID *int `json:"id"`  // 指针！
}
```

**原因：** Notification（通知）无 ID，必须是 `nil`。如果是值类型 `int`，无法表示"无 ID"。

### 2. 为什么串行化请求？

```go
func (c *Client) call(...) {
    c.mu.Lock()
    defer c.mu.Unlock()
    // ... 发送请求并等待响应
}
```

**原因：** 简化实现。生产环境应该用 goroutine 池 + ID 匹配支持并行请求。

### 3. 为什么工具错误用 isError 而不是 JSON-RPC error？

```go
result := CallToolResult{
    Content: content,
    IsError: err != nil,
}
```

**原因：** 工具错误应该让 LLM 看到，从而决定是否重试。JSON-RPC error 是协议层错误。

### 4. 为什么 mapToJSONSchema 在 Server 里？

```go
schema = mapToJSONSchema(t.Parameters())
```

**原因：** 保持类型定义纯粹。MapToJSONSchema 是工具系统的一部分，不属于 MCP 核心。

## 完整示例流程

### 1. 启动 Server

```bash
go run example/mcp-demo/server/main.go
```

输出：
```
[Server] MCP Server 启动
[Server] 将从 stdin 读取 JSON-RPC 消息
[Server] 将向 stdout 写入 JSON-RPC 响应
```

### 2. Client 连接并握手

```json
// Client → Server
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "clientInfo": {"name": "llmagent", "version": "0.1.0"},
    "capabilities": {...}
  }
}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "serverInfo": {"name": "demo-server", "version": "1.0.0"},
    "capabilities": {...}
  }
}
```

### 3. 列出工具

```json
// Client → Server
{"jsonrpc":"2.0","id":2,"method":"tools/list"}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "get_time",
        "description": "获取当前时间，支持按 IANA 时区格式化",
        "inputSchema": {...}
      },
      {
        "name": "calc",
        "description": "计算只包含数字、括号、+、-、*、/ 的算术表达式",
        "inputSchema": {...}
      }
    ]
  }
}
```

### 4. 调用工具

```json
// Client → Server
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_time",
    "arguments": {}
  }
}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "2025-07-29T14:30:00+08:00"
      }
    ],
    "isError": false
  }
}
```

### 5. 桥接到 Agent

```go
client, mcpTools, _ := mcp.NewBridgedClient("go", []string{"run", "server/main.go"})
defer client.Close()

registry := tool.NewRegistryToolSet()
registry.Register(mcpTools...)

agent := agent.New(provider, model, registry)
events, _ := agent.Run(ctx, "查询当前时间")

// Agent 内部流程：
// 1. 获取工具定义 → registry.ToolDefs()
// 2. 发送给 LLM
// 3. LLM 输出工具调用
// 4. Agent 调用 registry.Call(ctx, "get_time", args)
// 5. bridgedTool.Call() → client.CallTool()
// 6. MCP 工具执行
// 7. 结果返回给 LLM
```

## 验证清单

- [x] Server 正确处理 initialize
- [x] Server 正确处理 tools/list
- [x] Server 正确处理 tools/call
- [x] Client 握手流程完整
- [x] Client 列出工具
- [x] Client 调用工具
- [x] BridgeAll 桥接所有工具
- [x] BridgeTool 桥接单个工具
- [x] bridgedTool 实现 tool.Tool 接口
- [x] bridgedTool 实现 SchemaTool 接口
- [x] 可以在 Agent 中使用
- [x] 构建成功
- [x] 测试通过

## 构建验证

```bash
$ go build ./mcp/...
Go build: Success

$ go build ./example/mcp-demo/server/...
Go build: Success

$ go build ./example/mcp-demo/simple-integration.go
Go build: Success
```

## 运行示例

```bash
# Server
go run example/mcp-demo/server/main.go

# Client（新终端）
go run example/mcp-demo/client/main.go go run server/main.go

# 简单集成
go run example/mcp-demo/simple-integration.go
```

## 下一步

### 可选增强

1. **并发支持**：用 goroutine 池 + ID 匹配支持并行请求
2. **超时控制**：在 Client.call() 中使用 context.WithTimeout
3. **流式响应**：实现 Server 主动推送通知
4. **Streamable HTTP**：添加 HTTP 传输支持
5. **Session 管理**：实现 Mcp-Session-Id 生命周期
6. **资源支持**：实现 resources/list 和 resources/read
7. **提示词支持**：实现 prompts/list 和 prompts/get

### 生产就绪

1. **错误处理**：更详细的错误分类和日志
2. **重试机制**：指数退避 + 熔断器
3. **指标监控**：请求延迟、错误率、吞吐量
4. **配置化**：协议版本、超时时间等可配置
5. **安全性**：输入验证、权限控制

## 参考

- [MCP 规范文档](../docs/MCP-DEEP-DIVE.md)
- [JSON-RPC 2.0 规范](https://www.jsonrpc.org/specification)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)

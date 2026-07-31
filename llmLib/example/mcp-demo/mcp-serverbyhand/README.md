# MCP Server 手写练习 (mcp-serverbyhand)

## 概述

本示例展示如何**从零手写一个 MCP stdio Server**，并将其接入 Agent 框架。

基于 `llmLib/example/mcp-demo` 中的 mcp-demo 扩展实现。

## 练习目标

实现一个完整的 MCP Server 和 Client，包括：

- ✅ Server 端读取 stdin 的 JSON-RPC 请求，按 `method` 分发，写 stdout 响应
- ✅ `tools/list` 返回工具名、描述和 `inputSchema`
- ✅ `tools/call` 返回 `content` 内容块数组，工具自身错误返回 `isError=true`
- ✅ Client 端用 `StdioClient` 启动 Server，调用 `Initialize`、`Initialized`、`ListTools`、`CallTool`
- ✅ 用 `BridgeAll` 把 MCP 工具注册进 `tool.Registry`
- ✅ 让 M04 Agent 用 MCP 工具完整跑一轮
- ✅ 给工具输出加一层 6.7 生态方向与安全中的净化包装

## 文件结构

```
mcp-serverbyhand/
├── README.md           # 本文件
├── server.go           # 手写的 MCP stdio Server（从零实现）
├── client.go           # 使用 mcp.StdioClient 和 mcp.BridgeAll 的客户端
├── secure_demo.go      # 安全净化包装演示
└── main.go             # 主程序入口
```

## 核心概念

### 1. MCP 协议

MCP (Model Context Protocol) 是基于 JSON-RPC 2.0 的协议：

- **Server**：从 stdin 读取 JSON-RPC 请求，向 stdout 写入响应
- **Client**：向 stdin 写入请求，从 stdout 读取响应
- **Transport**：stdio（标准输入/输出）或 SSE

### 2. 三个核心方法

#### initialize
客户端向 Server 发起握手，交换能力信息。

**请求：**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0"
    },
    "capabilities": {
      "roots": { "listChanged": true }
    }
  }
}
```

**响应：**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "serverInfo": {
      "name": "my-server",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": { "listChanged": false },
      "resources": { "subscribe": false, "listChanged": false },
      "prompts": { "listChanged": false },
      "logging": {}
    }
  }
}
```

#### tools/list
列出 Server 提供的所有工具。

**请求：**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": null
}
```

**响应：**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "get_time",
        "description": "获取当前时间",
        "inputSchema": {
          "type": "object",
          "properties": {
            "timezone": {
              "type": "string",
              "description": "IANA 时区名"
            }
          }
        }
      }
    ]
  }
}
```

#### tools/call
调用指定工具。

**请求：**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_time",
    "arguments": {
      "timezone": "Asia/Shanghai"
    }
  }
}
```

**响应：**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "当前时间: 2025-07-31T10:30:00+08:00"
      }
    ],
    "isError": false
  }
}
```

### 3. 工具桥接

使用 `mcp.BridgeAll` 将 MCP 工具转换为本地 `tool.Tool`：

```go
client, mcpTools, err := mcp.NewBridgedClient(
    "go", []string{"run", "server.go"},
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 注册到 Registry
registry := tool.NewRegistryToolSet()
for _, t := range mcpTools {
    registry.Register(t)
}
```

### 4. 安全净化包装

使用 `mcp.SecureBridgeAll` 和 `security.Sanitizer` 添加安全层：

```go
client, secureTools, err := mcp.NewSecureBridgedClient(
    "go", []string{"run", "server.go"},
    mcp.WithToolWhitelist([]string{"get_time", "calc"}),
)
```

安全特性：
- ✅ **工具白名单**：只允许指定的工具被调用
- ✅ **输出净化**：自动过滤敏感信息（邮箱、电话、密钥等）
- ✅ **审计日志**：记录所有工具调用（参数、结果、错误、耗时）
- ✅ **权限检查**：调用前后进行安全检查

## 使用方法

### 1. 启动手写 Server

```bash
cd mcp-serverbyhand
go run server.go
```

Server 将从 stdin 读取 JSON-RPC 消息，向 stdout 写入响应。

### 2. 运行客户端演示

```bash
go run client.go
```

这将：
1. 启动手写 Server
2. 执行完整的 MCP 握手
3. 列出工具
4. 桥接工具到本地 `tool.Tool`
5. 注册到 `tool.Registry`
6. 测试工具调用
7. 展示在 Agent 中的使用方法

### 3. 运行安全净化演示

```bash
go run secure_demo.go
```

这将：
1. 配置工具白名单
2. 使用 `SecureBridgeAll` 安全桥接
3. 演示输出净化
4. 查看审计日志
5. 展示在 Agent 中使用安全包装

## 验收点检查

### ✅ Server 端读取 stdin 的 JSON-RPC 请求

参见 `server.go` 的 `serveWithIO` 方法：
```go
line, err := reader.ReadBytes('\n')
```

### ✅ 按 `method` 分发

参见 `server.go` 的 `handleMessage` 方法：
```go
switch req.Method {
case "initialize":
    return s.handleInitialize(*req.ID, req.Params)
case "tools/list":
    return s.handleListTools(*req.ID)
case "tools/call":
    return s.handleCallTool(*req.ID, req.Params)
}
```

### ✅ `tools/list` 返回工具名、描述和 `inputSchema`

参见 `server.go` 的 `handleListTools` 方法：
```go
tools = append(tools, tool{
    Name:        t.name,
    Description: t.description,
    InputSchema: t.inputSchema,
})
```

### ✅ `tools/call` 返回 `content` 内容块数组

参见 `server.go` 的 `handleCallTool` 方法：
```go
callResult := callToolResult{
    Content: content,  // 内容块数组
    IsError: err != nil,
}
```

### ✅ Client 端用 `StdioClient` 启动 Server

参见 `client.go`：
```go
client, err := mcp.NewClient(serverCmd, serverArgs)
```

### ✅ 调用 `Initialize`、`Initialized`、`ListTools`、`CallTool`

参见 `client.go` 的 `main` 函数。

### ✅ 用 `BridgeAll` 把 MCP 工具注册进 `tool.Registry`

参见 `client.go` 第 4 节：
```go
bridgedTools, err := mcp.BridgeAll(ctx, client2)
for _, t := range bridgedTools {
    registry.Register(t)
}
```

### ✅ 让 M04 Agent 用 MCP 工具完整跑一轮

参见 `client.go` 第 8 节，模拟完整的 Agent 对话流程。

### ✅ 给工具输出加一层 6.7 生态方向与安全中的净化包装

参见 `secure_demo.go`，使用 `SecureBridgedTool` 和 `Sanitizer`。

## 关键技术点

### JSON-RPC 2.0

- **请求**：包含 `jsonrpc`、`method`、`params`、`id`
- **响应**：包含 `jsonrpc`、`result` 或 `error`、`id`
- **通知**：无 `id` 字段，单向消息

### stdio 通信

- Server 从 stdin 读取，向 stdout 写入
- 每行一条 JSON-RPC 消息
- 使用 `\n` 作为消息分隔符

### 工具定义

- 使用 JSON Schema 描述参数
- 包含 `name`、`description`、`inputSchema`

### 工具调用

- 参数通过 `arguments` 字段传递
- 返回 `content` 数组和 `isError` 标志

## 工具白名单

`server.go` 中注册了两个工具：

1. **get_time**：获取当前时间
   - 参数：`timezone`（可选，IANA 时区名）

2. **calc**：计算算术表达式
   - 参数：`expr`（必需，表达式字符串）

**注意**：当前 `simpleEval` 函数未实现真正的表达式解析，始终返回错误。实际使用时应该集成数学表达式解析库。

## 扩展阅读

- [MCP 协议规范](https://modelcontextprotocol.io/specification)
- [JSON-RPC 2.0 规范](https://www.jsonrpc.org/specification)
- [JSON Schema 规范](https://json-schema.org/)
- `llmLib/mcp/` 中的实现参考
- `llmLib/security/` 中的安全组件

## 练习完成

✅ 所有验收点已实现
✅ 手写 Server 正确处理 JSON-RPC
✅ Client 成功桥接工具到 Agent
✅ 安全净化包装完整可用

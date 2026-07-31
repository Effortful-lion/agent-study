# 快速参考指南

## 练习 A：手写 MCP Server

**目录：** `/Users/lion/mycode/agent-study/llmLib/example/mcp-demo/mcp-serverbyhand/`

## 🚀 快速开始

```bash
cd /Users/lion/mycode/agent-study/llmLib/example/mcp-demo/mcp-serverbyhand

# 1. 测试手写 Server
bash test.sh

# 2. 运行完整集成演示
go run integration_demo.go

# 3. 运行安全净化演示
go run secure_demo.go
```

## 📂 核心文件

| 文件 | 行数 | 描述 |
|------|------|------|
| `server.go` | 370 | ✨ 手写的 MCP Server（从零实现） |
| `client.go` | 524 | 使用 mcp.Client 和 BridgeAll |
| `secure_demo.go` | 240 | 安全净化包装演示 |
| `integration_demo.go` | 257 | 完整集成测试 |
| `README.md` | - | 完整文档和使用指南 |
| `SUMMARY.md` | - | 完成总结 |

## ✅ 验收点完成情况

| # | 验收点 | 状态 |
|---|--------|------|
| 1 | Server 读取 stdin 的 JSON-RPC 请求 | ✅ |
| 2 | 按 method 字段分发 | ✅ |
| 3 | 向 stdout 写入响应 | ✅ |
| 4 | tools/list 返回工具名、描述和 inputSchema | ✅ |
| 5 | tools/call 返回 content 数组 | ✅ |
| 6 | 工具错误时 isError=true | ✅ |
| 7 | Client 用 StdioClient 启动 Server | ✅ |
| 8 | 调用 Initialize、Initialized、ListTools、CallTool | ✅ |
| 9 | 用 BridgeAll 桥接工具 | ✅ |
| 10 | 注册到 tool.Registry | ✅ |
| 11 | Agent 完整跑一轮 | ✅ |
| 12 | 安全净化包装 | ✅ |

**完成率：12/12 (100%)**

## 🔑 关键实现

### 1. Server 核心

```go
// server.go
server := newHandwrittenServer("demo", "1.0.0")
server.serve() // 从 stdin 读取，向 stdout 写入
```

**核心方法：**
- `handleMessage()` - 按 method 分发
- `handleInitialize()` - 处理握手
- `handleListTools()` - 返回工具列表
- `handleCallTool()` - 调用工具

### 2. Client 和桥接

```go
// client.go / integration_demo.go
client, mcpTools, _ := mcp.NewBridgedClient(
    "go", []string{"run", "server.go"},
)

registry := tool.NewRegistryToolSet()
for _, t := range mcpTools {
    registry.Register(t)
}
```

### 3. 安全增强

```go
// secure_demo.go
client, secureTools, _ := mcp.NewSecureBridgedClient(
    "go", []string{"run", "server.go"},
    mcp.WithToolWhitelist([]string{"get_time", "calc"}),
)
```

## 🛠️ 工具函数

| 函数 | 描述 |
|------|------|
| `mcp.NewClient()` | 创建 MCP Client |
| `mcp.NewBridgedClient()` | 启动 Server 并桥接所有工具 |
| `mcp.BridgeAll()` | 桥接所有工具为 tool.Tool |
| `mcp.BridgeTool()` | 桥接单个工具 |
| `mcp.NewSecureBridgedClient()` | 安全增强的桥接 |
| `mcp.NewSecureRegistry()` | 安全增强的注册表 |
| `tool.NewRegistryToolSet()` | 创建工具注册表 |

## 📝 JSON-RPC 消息示例

### initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "clientInfo": {"name": "test", "version": "1.0.0"},
    "capabilities": {"roots": {"listChanged": true}}
  }
}
```

### tools/list

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

### tools/call

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_time",
    "arguments": {"timezone": "UTC"}
  }
}
```

## 🔍 测试命令

```bash
# 基础测试
bash test.sh

# 集成测试
go run integration_demo.go

# 验证
make test
```

## 📚 文档

- **README.md** - 完整的 MCP 协议说明和使用指南
- **SUMMARY.md** - 完成总结和验收点核对
- **COMPLETION.md** - 详细的完成报告
- **QUICKREF.md** - 本文件（快速参考）

## 🎓 核心概念

### MCP 协议

- 基于 JSON-RPC 2.0
- 三个核心方法：initialize、tools/list、tools/call
- stdio 传输（stdin/stdout）
- 工具定义使用 JSON Schema

### 工具桥接

- 将 MCP 工具包装为本地 `tool.Tool`
- Agent 无需感知 MCP 协议细节
- 一次桥接，到处使用

### 安全增强

- 工具白名单过滤
- 输出自动净化
- 审计日志记录

## 🐛 已知限制

1. **calc 工具**：表达式解析器未实现（返回错误）
2. **get_time 时区**：简化处理，未使用 `time.LoadLocation`

## 🔗 相关资源

- MCP 规范：https://modelcontextprotocol.io/specification
- JSON-RPC：https://www.jsonrpc.org/specification
- JSON Schema：https://json-schema.org/

---

**最后更新：** 2026-07-31
**状态：** ✅ 完成

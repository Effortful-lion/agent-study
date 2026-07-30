# MCP 学习路径：层层递进

> 📌 **注意：** 此文档同时存在于两个位置：
> - `mcp/LEARNING-PATH.md` - 包内版本（便于开发时参考）
> - `docs/MCP-LEARNING-PATH.md` - 项目文档版本（便于集中阅读）
>
> 建议阅读项目文档版本：`docs/MCP-LEARNING-PATH.md`

## 📚 学习路径总览

我们从零开始，一步步深入 MCP 的世界：

```
Level 1: Server 端
   ↓
Level 2: Client ↔ Server 通信
   ↓
Level 3: Bridge 桥接
   ↓
Level 4: Agent 集成
```

---

## Level 1: 单独 Server

**目标：** 理解 MCP Server 的基本结构和工具注册

**文件：** `server/main.go`

**运行：**
```bash
go run example/mcp-demo/server/main.go
```

**输出：**
```
[Server] MCP Server 启动
[Server] 将从 stdin 读取 JSON-RPC 消息
[Server] 将向 stdout 写入 JSON-RPC 响应
```

**核心代码：**
```go
// 1. 创建 Server
server := mcp.NewServer("demo-server", "1.0.0")

// 2. 注册工具
server.AddTool(tool.NewJSONSchemaTool(
    "get_time", "获取当前时间", schema, handler,
))

// 3. 启动 Server（从 stdin 读，向 stdout 写）
server.Serve()
```

**学到的内容：**
- ✅ MCP Server 是什么
- ✅ 如何注册工具
- ✅ stdio 传输的基本概念
- ✅ JSON-RPC 消息的发送/接收

---

## Level 2: Client ↔ Server 完整通信

**目标：** 理解完整的 MCP 协议流程

**文件：** `client/main.go`

**运行：**
```bash
# 方式1：一条命令（自动启动 Server）
go run example/mcp-demo/client/main.go go run server/main.go

# 方式2：手动启动（两个终端）
# 终端1：go run example/mcp-demo/server/main.go
# 终端2：go run example/mcp-demo/client/main.go go run server/main.go
```

**输出示例：**
```
=== MCP 演示 Client ===

========== 1. 握手阶段 ==========
Server 信息:
  名称: demo-server
  版本: 1.0.0
  协议版本: 2025-11-25
✓ 握手完成

========== 2. 列出工具 ==========
可用工具 (共 2 个):
  - get_time: 获取当前时间
  - calc: 计算算术表达式

========== 3. 调用工具 ==========
5.1 调用 get_time:
  结果: 2025-07-29T14:30:00+08:00

5.3 调用 calc (12 * (3 + 4)):
  结果: 12*(3+4) = 84

========== 4. 原始消息流演示 ==========
→ 请求:
  {"jsonrpc":"2.0","id":1,"method":"initialize",...}
← 响应:
  {"jsonrpc":"2.0","id":1,"result":{...}}

========== 5. 桥接工具演示 ==========
成功桥接 2 个工具:
  - get_time
  - calc
```

**核心代码：**
```go
// 1. 启动 Client（自动启动 Server 子进程）
client, err := mcp.NewClient("go", []string{"run", "server/main.go"})
defer client.Close()

// 2. 握手
serverInfo, err := client.Initialize(ctx)
client.Initialized(ctx)

// 3. 列出工具
tools, err := client.ListTools(ctx)

// 4. 调用工具
result, err := client.CallTool(ctx, "get_time", nil)

// 5. 展示原始消息流
demonstrateRawMessages(...)

// 6. 演示桥接
demonstrateBridging(...)
```

**学到的内容：**
- ✅ Client 如何启动 Server
- ✅ 完整的握手流程（initialize → initialized）
- ✅ 列出工具（tools/list）
- ✅ 调用工具（tools/call）
- ✅ 原始 JSON-RPC 消息格式
- ✅ 工具桥接的基本概念

---

## Level 3: Bridge 桥接

**目标：** 理解如何将 MCP 工具桥接到本地

**文件：** `integrated/main.go`

**运行：**
```bash
go run example/mcp-demo/integrated/main.go
```

**输出示例：**
```
=== MCP 工具桥接演示 ===

步骤 1: 启动 MCP Server...
✓ 成功桥接 2 个 MCP 工具

步骤 2: 注册工具到 Registry...
  ✓ get_time: 获取当前时间
  ✓ calc: 计算算术表达式

步骤 3: 工具定义 (供 LLM 使用)...
工具名称: get_time
工具描述: 获取当前时间
参数 Schema: {"type":"object","properties":{...}}

步骤 4: 测试工具调用...
调用 get_time:
  ✓ 结果: 2025-07-29T14:30:00+08:00

调用 calc:
  ✓ 结果: 12*(3+4) = 84

步骤 5: Agent 集成说明...
```

**核心代码：**
```go
// 1. 一键桥接所有工具
client, mcpTools, err := mcp.NewBridgedClient(
    "go", []string{"run", "server/main.go"},
)
defer client.Close()

// 2. 注册到 Registry
registry := tool.NewRegistryToolSet()
for _, mcpTool := range mcpTools {
    registry.Register(mcpTool)
}

// 3. 测试工具调用
result, err := registry.Call(ctx, "get_time", args)

// 4. 在 Agent 中使用
agent := agent.New(provider, model, registry)
events, _ := agent.Run(ctx, "查询当前时间")
```

**学到的内容：**
- ✅ BridgeAll() 如何工作
- ✅ 如何将 MCP 工具转换为本地 tool.Tool
- ✅ 如何注册到 Registry
- ✅ 如何通过 Registry 调用 MCP 工具
- ✅ Agent 如何集成 MCP 工具

---

## Level 4: Agent 完整集成（概念演示）

**目标：** 理解完整的 Agent + MCP 架构

**文件：** `integrated/main.go` 中的 Agent 集成说明

**架构图：**
```
┌─────────────────────────────────────────────────────┐
│                    Agent                            │
│  ┌───────────────────────────────────────────────┐ │
│  │  1. 获取工具定义                               │ │
│  │     registry.ToolDefs()                        │ │
│  │         ↓                                      │ │
│  │  2. 发送给 LLM                                │ │
│  │     LLM 决定调用 get_time                      │ │
│  │         ↓                                      │ │
│  │  3. Agent 调用工具                             │ │
│  │     registry.Call(ctx, "get_time", args)      │ │
│  │         ↓                                      │ │
│  │  4. Registry 路由                              │ │
│  │     → bridgedTool.Call()                       │ │
│  │         ↓                                      │ │
│  │  5. MCP Client 调用 Server                     │ │
│  │     client.CallTool(ctx, "get_time", args)    │ │
│  │         ↓                                      │ │
│  │  6. 结果返回给 LLM                             │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

**学到的内容：**
- ✅ Agent 如何使用工具
- ✅ Registry 的路由机制
- ✅ LLM 如何决定调用工具
- ✅ 完整的 Agent-MCP 集成架构

---

## 📊 递进关系对比

| 级别 | 文件 | 复杂度 | 核心概念 | 运行难度 |
|------|------|--------|---------|---------|
| **Level 1** | `server/main.go` | ⭐ | Server 工具注册 | ⭐ 极易 |
| **Level 2** | `client/main.go` | ⭐⭐⭐ | 完整协议流程 | ⭐⭐ 简单 |
| **Level 3** | `integrated/main.go` | ⭐⭐ | Bridge 桥接 | ⭐⭐ 简单 |
| **Level 4** | Agent 集成说明 | ⭐⭐⭐⭐ | 完整架构 | ⭐⭐⭐ 中等 |

---

## 🎯 学习建议

### 路径 1：循序渐进（推荐新手）

```bash
# Step 1: 启动 Server，观察日志
go run example/mcp-demo/server/main.go

# Step 2: 运行 Client 演示，看完整流程
go run example/mcp-demo/client/main.go go run server/main.go

# Step 3: 运行集成示例，理解桥接
go run example/mcp-demo/integrated/main.go

# Step 4: 阅读源码，深入理解
cat mcp/server.go
cat mcp/client.go
cat mcp/bridge.go
```

### 路径 2：快速实践（有经验者）

```bash
# 直接运行集成示例
go run example/mcp-demo/integrated/main.go

# 查看关键代码
cat example/mcp-demo/integrated/main.go
```

### 路径 3：深入原理（想深入理解）

```bash
# 1. 阅读协议深度解析
cat docs/MCP-DEEP-DIVE.md

# 2. 逐行阅读实现
cat mcp/types.go
cat mcp/server.go
cat mcp/client.go
cat mcp/bridge.go

# 3. 运行测试
go test ./mcp/... -v
```

---

## 💡 关键递进点

### 1. Server → Client

**Server 端：** 工具注册、JSON-RPC 处理
**Client 端：** 协议通信、握手流程

**新增概念：**
- JSON-RPC 消息格式
- 请求/响应 ID 匹配
- stdin/stdout 通信

### 2. Client → Bridge

**Client 端：** 通信细节
**Bridge 端：** 接口转换

**新增概念：**
- tool.Tool 接口
- SchemaTool 接口
- 参数转换（JSON ↔ map）

### 3. Bridge → Agent

**Bridge 端：** 工具包装
**Agent 端：** 工具使用

**新增概念：**
- Registry 路由
- ToolDefs 生成
- Agent 工具调用循环

---

## 🗺️ 知识地图

```
MCP 协议
├── 三角色模型
│   ├── Host（AI 应用）
│   ├── Client（协议层）
│   └── Server（能力提供者）
│
├── 四类原语
│   ├── Tools（工具）
│   ├── Resources（资源）
│   ├── Prompts（提示词）
│   └── Logging（日志）
│
├── 会话生命周期
│   ├── 握手（initialize）
│   ├── 操作（tools/list, tools/call）
│   └── 关闭（Close）
│
└── 传输方式
    ├── stdio（标准输入输出）
    └── Streamable HTTP（远程服务）

我们的实现（当前）
├── stdio Server（Level 1）
├── stdio Client（Level 2）
├── Bridge（Level 3）
└── Agent 集成（Level 4）

扩展方向（未来）
├── Resources 支持
├── Prompts 支持
├── Streamable HTTP
├── Session 管理
└── 并发请求
```

---

## ✅ 每层验证清单

### Level 1: Server

- [ ] Server 能启动
- [ ] Server 等待 stdin 输入
- [ ] 日志输出到 stderr
- [ ] 可以注册工具
- [ ] 工具能被执行

**验证命令：**
```bash
go run example/mcp-demo/server/main.go
# 观察输出：Server 启动日志
```

### Level 2: Client

- [ ] Client 能启动 Server
- [ ] 握手成功
- [ ] 能列出工具
- [ ] 能调用工具
- [ ] 能展示原始消息

**验证命令：**
```bash
go run example/mcp-demo/client/main.go go run server/main.go
# 观察完整的协议流程
```

### Level 3: Bridge

- [ ] 能一键桥接所有工具
- [ ] MCP 工具变成 tool.Tool
- [ ] 能通过 Registry 调用
- [ ] 工具定义正确

**验证命令：**
```bash
go run example/mcp-demo/integrated/main.go
# 观察工具定义和调用结果
```

### Level 4: Agent（概念）

- [ ] 理解完整架构
- [ ] 知道如何集成
- [ ] 了解工具调用流程

**验证方式：**
- 阅读代码注释
- 理解架构图
- 尝试自己实现

---

## 🚀 实战建议

### 1. 先跑通示例

```bash
# 按照 Level 1 → Level 2 → Level 3 的顺序
# 每个都成功运行，看输出
```

### 2. 修改代码

```bash
# Level 1: 添加新工具
# 编辑 server/main.go，添加 "echo" 工具

# Level 2: 修改参数
# 编辑 client/main.go，测试不同的参数

# Level 3: 添加本地工具
# 编辑 integrated/main.go，混合本地和 MCP 工具
```

### 3. 调试输出

```bash
# 在 server.go 添加日志
fmt.Fprintln(os.Stderr, "[Server] 处理工具: ", p.Name)

# 在 client.go 添加日志
fmt.Fprintf(os.Stderr, "[Client] 发送请求: %s\n", method)

# 在 bridge.go 添加日志
fmt.Fprintf(os.Stderr, "[Bridge] 调用工具: %s\n", t.def.Name)
```

### 4. 连接真实 MCP Server

```bash
# 测试真实的 MCP Server
go run example/mcp-demo/client/main.go npx @modelcontextprotocol/server-filesystem /tmp

# 或
go run example/mcp-demo/client/main.go python db_server.py
```

---

## 📖 文档对应关系

| 级别 | 运行命令 | 查看文档 |
|------|---------|---------|
| Level 1 | `go run server/main.go` | `mcp/README.md#server-端` |
| Level 2 | `go run client/main.go go run server/main.go` | `mcp/README.md#client-端` |
| Level 3 | `go run integrated/main.go` | `mcp/README.md#bridge-到-agent` |
| Level 4 | 阅读源码 | `mcp/README.md#完整链路` |

---

## 🎓 学习成果

完成这四层后，你将掌握：

1. ✅ MCP 协议的核心概念
2. ✅ JSON-RPC 2.0 消息格式
3. ✅ stdio 传输机制
4. ✅ Server/Client/Bridge 全链路
5. ✅ 工具桥接到 Agent
6. ✅ 完整的 MCP 集成方案

**下一步：**
- 实现 Resources 支持
- 实现 Prompts 支持
- 添加 Streamable HTTP
- 研究真实 MCP Server（filesystem、git、database）

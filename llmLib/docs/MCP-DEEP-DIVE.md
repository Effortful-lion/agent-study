# MCP（Model Context Protocol）深度解析

## 目录

1. [MCP 是什么](#1-mcp-是什么)
2. [架构概览](#2-架构概览)
3. [核心原语](#3-核心原语)
4. [JSON-RPC 消息格式](#4-json-rpc-消息格式)
5. [会话生命周期](#5-会话生命周期)
6. [传输方式](#6-传输方式)
7. [协议对比](#7-协议对比)
8. [实战示例](#8-实战示例)

***

## 1. MCP 是什么

### 1.1 定义

**MCP（Model Context Protocol）** 是一个开放的、模型无关的协议，由 Anthropic 于 2024 年 11 月开源。

它定义"AI 应用 ↔ 外部能力"之间的标准对话方式，把工具调用/文档读取/提示词模板从"每对组合一套代码"变成"一次对接、各处复用"。

**类比：AI 应用的 USB-C 接口**

就像 USB-C 统一了设备连接方式，MCP 统一了 AI 应用与外部工具/数据的对接方式。

### 1.2 发展历程

- **2024-11**: Anthropic 开源 MCP
- **2025-03**: 引入 Streamable HTTP 传输（替代旧 SSE 模式）
- **2025-11-25**: 当前稳定版规范
- **2026+**: 持续演进，已被 OpenAI、Google、JetBrains、Cursor 等采纳

### 1.3 为什么需要 MCP

**问题：** 过去每接一个工具都要写一套专属对接代码，换个 Agent 框架又要重写。

```
GitHub API → 写一套对接代码 → Agent 框架 A
GitHub API → 再写一套 → Agent 框架 B
飞书 API   → 再写一套 → Agent 框架 A
飞书 API   → 再写一套 → Agent 框架 B
...
```

**MCP 解决：** N 个工具 × M 个客户端 = N+M，而不是 N×M

```
GitHub MCP Server → MCP Client → Agent 框架 A
GitHub MCP Server → MCP Client → Agent 框架 B
飞书 MCP Server   → MCP Client → Agent 框架 A
飞书 MCP Server   → MCP Client → Agent 框架 B
```

一次实现，到处复用。

***

## 2. 架构概览

### 2.1 三角色模型

```
┌─────────────┐
│    Host      │  ← AI 应用（Claude Desktop、Cursor、你的 Agent）
│  (AI App)    │
└──────┬───────┘
       │
       │ 1:N（一个 Host 可以连多个 Client）
       │
┌──────▼──────────────────────────────┐
│         Client                        │
│  ┌──────────────────────────────┐    │
│  │  • 协议状态管理               │    │
│  │  • 握手、能力协商             │    │
│  │  • 消息收发                   │    │
│  └──────────────────────────────┘    │
└──────┬───────────────────────────────┘
       │
       │ 1:1（一个 Client 连一个 Server）
       │
┌──────▼───────────────────────────────┐
│         Server                        │
│  ┌──────────────────────────────┐    │
│  │  • 暴露能力                   │    │
│  │  • 处理工具调用               │    │
│  │  • 返回结果                   │    │
│  └──────────────────────────────┘    │
└───────────────────────────────────────┘
```

### 2.2 角色详解

#### Host（主机）

**职责：**

- 持有 LLM、UI、用户意图
- 管理多个 Client 连接
- 决定何时调用哪些工具
- 处理用户交互

**示例：**

- Claude Desktop
- Cursor IDE
- 你自己写的 Agent

#### Client（客户端）

**职责：**

- 一对一连接一个 Server
- 管理协议状态（握手、能力协商）
- 消息收发
- 请求/响应匹配

**关键点：**

- Host 想接 N 个 Server 就开 N 个 Client
- Client 是 Host 内部的代码
- 每个 Client 只负责一个 Server

#### Server（服务端）

**职责：**

- 暴露具体能力（工具、资源、提示词）
- 执行工具调用
- 返回结果

**形式：**

- 本地子进程（文件系统、shell、git）
- 远程 HTTP 服务（GitHub、数据库、SaaS）

### 2.3 通信模式

```
Host 可以同时连接多个 Server：

┌─────────────┐
│   Host      │
│             │
│  ┌────────┐ │
│  │Client 1│─┼─→ Server 1（文件系统）
│  └────────┘ │
│             │
│  ┌────────┐ │
│  │Client 2│─┼─→ Server 2（数据库）
│  └────────┘ │
│             │
│  ┌────────┐ │
│  │Client 3│─┼─→ Server 3（GitHub）
│  └────────┘ │
└─────────────┘
```

***

## 3. 核心原语

MCP 通过四类原语向 Host 暴露能力：

### 3.1 Tools（工具）

**定义：** 可调用动作，让模型执行查询、写入、发送、计算等操作。

**适用场景：**

- 查询数据库
- 读写文件
- 执行计算
- 发送 HTTP 请求
- 运行命令

**方法：**

| 方法                                 | 方向              | 作用       |
| ---------------------------------- | --------------- | -------- |
| `tools/list`                       | Client → Server | 列出可用工具   |
| `tools/call`                       | Client → Server | 调用某个工具   |
| `notifications/tools/list_changed` | Server → Client | 通知工具列表变化 |

**工具声明示例：**

```json
{
  "name": "get_weather",
  "description": "查询指定城市的天气",
  "inputSchema": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "城市名，如 北京"
      },
      "days": {
        "type": "number",
        "description": "预报天数，默认 1"
      }
    },
    "required": ["city"]
  }
}
```

**工具调用示例：**

```json
{
  "name": "get_weather",
  "arguments": {
    "city": "北京",
    "days": 3
  }
}
```

**工具返回示例：**

```json
{
  "content": [
    {
      "type": "text",
      "text": "北京未来3天：晴，15-25℃"
    }
  ],
  "isError": false
}
```

### 3.2 Prompts（提示词）

**定义：** 可复用模板，更像 slash command 或对话起手式，通常由用户触发。

**与 Tools 的区别：**

- **Tools**：LLM 在循环里按需调用
- **Prompts**：用户或 Host UI 选择触发

**适用场景：**

- 代码审查模板
- 文档生成起手式
- 分析报告框架

**方法：**

| 方法                                   | 方向              | 作用                |
| ------------------------------------ | --------------- | ----------------- |
| `prompts/list`                       | Client → Server | 列出模板              |
| `prompts/get`                        | Client → Server | 获取填好参数后的 messages |
| `notifications/prompts/list_changed` | Server → Client | 通知模板列表变化          |

**Prompt 示例：**

```json
{
  "name": "review-pr",
  "description": "审查 Pull Request",
  "arguments": [
    {
      "name": "pr_number",
      "type": "number",
      "required": true,
      "description": "PR 编号"
    }
  ]
}
```

### 3.3 Resources（资源）

**定义：** 可读取数据，适合文件、文档、URL、数据库行、监控指标等。

**与 Tools 的区别：**

- **Tools**：LLM 决定何时调用
- **Resources**：用户或 Host UI 选择某个 URI

**适用场景：**

- 读取知识库文档
- 访问数据库记录
- 获取监控指标
- 读取配置文件

**方法：**

| 方法                                     | 方向              | 作用       |
| -------------------------------------- | --------------- | -------- |
| `resources/list`                       | Client → Server | 列出资源     |
| `resources/read`                       | Client → Server | 读取资源内容   |
| `resources/subscribe`                  | Client → Server | 订阅资源变化   |
| `resources/unsubscribe`                | Client → Server | 取消订阅     |
| `notifications/resources/updated`      | Server → Client | 资源更新通知   |
| `notifications/resources/list_changed` | Server → Client | 资源列表变化通知 |

**Resource 示例：**

```json
{
  "uri": "file:///docs/api.md",
  "name": "API 文档",
  "mimeType": "text/markdown",
  "description": "API 接口文档"
}
```

### 3.4 Logging（日志）

**定义：** Server 到 Client 的诊断流，用于推送结构化日志。

**适用场景：**

- 长任务进度
- 异常处理
- 调试信息

**方法：**

| 方法                      | 方向              | 作用     |
| ----------------------- | --------------- | ------ |
| `logging/setLevel`      | Client → Server | 设置日志级别 |
| `notifications/message` | Server → Client | 推送日志消息 |

**日志消息示例：**

```json
{
  "level": "info",
  "data": "开始索引文档...",
  "logger": "indexer"
}
```

### 3.5 核心原语对比

| 原语            | 谁决定何时取  | 典型用例     | 返回类型         |
| ------------- | ------- | -------- | ------------ |
| **Tools**     | LLM 按需  | 查询、计算、写入 | 任意（工具定义）     |
| **Prompts**   | 用户触发    | 模板、起手式   | messages 数组  |
| **Resources** | 用户/Host | 文件、数据、文档 | text/content |
| **Logging**   | Server  | 进度、诊断    | 结构化日志        |

**核心思维模型：**

- **Tools** = 模型主动发现并调用的动作
- **Prompts** = 用户选择的对话模板
- **Resources** = 用户选择的可读取数据
- **Logging** = Server 的诊断通道

### 3.6 扩展原语

除核心四类外，规范还定义了一些扩展能力：

| 能力               | 说明                      | 成熟度 |
| ---------------- | ----------------------- | --- |
| **Sampling**     | Server 请求 Client 采样 LLM | 稳定  |
| **Roots**        | 文件系统根目录协商               | 稳定  |
| **Elicitation**  | Server 请求用户输入           | 实验性 |
| **Completion**   | 自动补全建议                  | 实验性 |
| **Progress**     | 进度通知                    | 稳定  |
| **Cancellation** | 取消请求                    | 稳定  |

**建议：** Tools-centric 的 Agent 工作流中，优先关注 Tools、Resources、Prompts 和 Logging。

***

## 4. JSON-RPC 消息格式

### 4.1 什么是 JSON-RPC 2.0

MCP 使用 **JSON-RPC 2.0** 作为消息信封格式。它是一种轻量级的远程过程调用（RPC）协议。

**核心特点：**

- 基于 JSON
- 简单、人类可读
- 跨语言方便
- 不支持二进制格式（牺牲性能换取简单性）

### 4.2 三种消息类型

#### 类型 1：Request（请求）

**特征：** 有 `id`，期待响应

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {
      "city": "北京"
    }
  }
}
```

**字段说明：**

- `jsonrpc`：固定值 "2.0"
- `id`：请求 ID（数字或字符串），用于匹配响应
- `method`：要调用的方法名
- `params`：参数对象（可选）

#### 类型 2：Response（响应）

**特征：** 回填同一个 `id`，带 `result` 或 `error`

**成功响应：**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "北京：晴，15-25℃"
      }
    ],
    "isError": false
  }
}
```

**错误响应：**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "details": "缺少必需的参数：city"
    }
  }
}
```

**字段说明：**

- `jsonrpc`：固定值 "2.0"
- `id`：对应的请求 ID
- `result`：成功结果（与 error 二选一）
- `error`：错误信息（与 result 二选一）

#### 类型 3：Notification（通知）

**特征：** 无 `id`，单向消息，不期待响应

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/resources/updated",
  "params": {
    "uri": "file:///docs/api.md"
  }
}
```

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/message",
  "params": {
    "level": "info",
    "data": "索引完成，共处理 42 个文件"
  }
}
```

**字段说明：**

- `jsonrpc`：固定值 "2.0"
- `method`：通知方法名
- `params`：参数（可选）
- **无** **`id`** **字段**：这是与 Request 的关键区别

### 4.3 JSON-RPC 标准错误码

| Code     | 含义               | 说明        |
| -------- | ---------------- | --------- |
| `-32700` | Parse error      | JSON 解析失败 |
| `-32600` | Invalid Request  | 请求格式错误    |
| `-32601` | Method not found | 方法不存在     |
| `-32602` | Invalid params   | 参数无效      |
| `-32603` | Internal error   | 服务器内部错误   |

**重要原则：**

- JSON-RPC error 用于**协议层错误**
- 工具自身错误应返回 `isError=true` 的 result，让模型看到错误内容

**示例：**

```json
// ❌ 不好的做法：SQL 语法错误返回 JSON-RPC error
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {"code": -32603, "message": "SQL 语法错误"}
}

// ✅ 好的做法：工具返回 isError=true
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{"type": "text", "text": "ERROR: 语法错误 near 'SELCT'"}],
    "isError": true
  }
}
```

### 4.4 消息流程示例

```
Client                                 Server
  |                                      |
  |--- Request: tools/list ------------->|
  |   {id: 1, method: "tools/list"}      |
  |                                      |
  |<-- Response: 工具列表 ----------------|
  |   {id: 1, result: {tools: [...]}}    |
  |                                      |
  |--- Request: tools/call ------------->|
  |   {id: 2, method: "tools/call",      |
  |    params: {name: "get_weather"}}    |
  |                                      |
  |<-- Response: 工具结果 -----------------|
  |   {id: 2, result: {content: [...]}}  |
  |                                      |
  |--- Notification: progress ---------->|
  |   {method: "notifications/progress"} |
  |   (无 id，不期待响应)                 |
```

***

## 5. 会话生命周期

### 5.1 三阶段模型

```
┌─────────────┐
│  1. 握手     │
│  Initialize  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  2. 操作     │
│  工具调用等   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  3. 关闭     │
│  清理资源     │
└─────────────┘
```

### 5.2 阶段 1：握手（Initialize）

**目的：** Client 和 Server 交换协议版本、能力声明

**流程：**

```
Client                               Server
  |                                      |
  |--- Request: initialize ------------->|
  |   {                                  |
  |     protocolVersion: "2025-11-25",   |
  |     clientInfo: {...},               |
  |     capabilities: {...}              |
  |   }                                  |
  |                                      |
  |<-- Response: 初始化完成 ----------------|
  |   {                                  |
  |     protocolVersion: "2025-11-25",   |
  |     serverInfo: {...},               |
  |     capabilities: {...}              |
  |   }                                  |
  |                                      |
  |--- Notification: initialized ------->|
  |   (通知握手完成)                      |
```

**详细示例：**

**Client → Server:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "clientInfo": {
      "name": "llmagent",
      "version": "0.1.0"
    },
    "capabilities": {
      "roots": {
        "listChanged": true
      },
      "sampling": {}
    }
  }
}
```

**Server → Client:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "serverInfo": {
      "name": "git-server",
      "version": "0.3.0"
    },
    "capabilities": {
      "tools": {
        "listChanged": true
      },
      "resources": {
        "subscribe": true,
        "listChanged": true
      },
      "prompts": {
        "listChanged": false
      },
      "logging": {}
    }
  }
}
```

**Client → Server (Notification):**

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}
```

**关键点：**

- `protocolVersion`：双方协商的协议版本
- `capabilities`：声明支持的能力，老 Client 遇到新 Server 仍能工作
- `notifications/initialized`：通知 Server 握手完成，此后可以开始正常操作

### 5.3 阶段 2：操作（正常交互）

握手完成后，进入正常操作阶段。Client 和 Server 根据握手时声明的能力进行交互。

**典型操作序列：**

```
1. 发现工具
   Client → Server: tools/list
   Server → Client: 工具列表

2. 调用工具
   Client → Server: tools/call
   Server → Client: 工具结果

3. 读取资源
   Client → Server: resources/read
   Server → Client: 资源内容

4. 获取提示词
   Client → Server: prompts/get
   Server → Client: 填充后的 messages

5. 接收通知
   Server → Client: notifications/message (日志)
   Server → Client: notifications/progress (进度)
```

**完整示例：**

```json
// 1. 列工具
Client → Server:
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}

Server → Client:
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
          "properties": {}
        }
      }
    ]
  }
}

// 2. 调用工具
Client → Server:
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_time",
    "arguments": {}
  }
}

Server → Client:
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

// 3. Server 推送进度通知
Server → Client:
{
  "jsonrpc": "2.0",
  "method": "notifications/progress",
  "params": {
    "progressToken": "abc123",
    "progress": 50,
    "total": 100,
    "message": "处理中..."
  }
}
```

### 5.4 阶段 3：关闭（Cleanup）

**目的：** 清理资源，优雅关闭连接

**关闭方式：**

**stdio 传输：**

```
Client 关闭 stdin → Server 收到 EOF → Server 退出
```

**Streamable HTTP 传输：**

```
Client → Server: DELETE /mcp
Server → Client: 204 No Content
```

**超时关闭：**

- Streamable HTTP 支持空闲超时
- Server 可以配置会话超时时间

### 5.5 取消机制

MCP 支持取消正在进行的请求，但**尽力而为（best-effort）**。

**取消通知：**

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/cancelled",
  "params": {
    "requestId": 42,
    "reason": "user aborted"
  }
}
```

**关键点：**

- Server 收到取消通知后应尽快停止
- 但协议**无法强制**它一定停止
- 取消是通知，不是请求，不需要响应

***

## 6. 传输方式

MCP 协议与传输正交。同一套 JSON-RPC 消息可以走 stdio，也可以走 Streamable HTTP。

### 6.1 stdio（标准输入输出）

#### 什么是 stdio

**stdio** 是最常用、最简单的传输方式。Client 将 Server 作为**子进程启动**，通过 stdin 发送请求，通过 stdout 接收响应。

**数据流：**

```
Client ──stdin──→ Server (JSON-RPC 请求，一行一条)
Client ←─stdout── Server (JSON-RPC 响应/通知，一行一条)
                  Server (日志一律走 stderr)
```

#### 特点

✅ **优点：**

- 最简单，无需网络
- 适合本地工具（文件系统、shell、git、本地数据库）
- 自动进程管理（Client 退出时 Server 也随之退出）
- 无网络开销

❌ **缺点：**

- 只能本地通信
- 无法跨机器
- 无法多 Client 共享一个 Server

#### 消息格式

**每行一条 JSON-RPC 消息（换行符分隔）：**

```
Client → Server:
{"jsonrpc":"2.0","id":1,"method":"tools/list"}\n
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{...}}\n

Server → Client:
{"jsonrpc":"2.0","id":1,"result":{...}}\n
{"jsonrpc":"2.0","id":2,"result":{...}}\n
```

**关键规则：**

- **Server 的 stdout 只能输出合法的 MCP 消息**
- **所有日志必须走 stderr**
- 否则 Client 的 JSON-RPC 解析会被污染

#### 典型配置

**Claude Desktop 配置示例：**

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/me/projects"]
    },
    "git": {
      "command": "uvx",
      "args": ["mcp-server-git", "--repository", "/Users/me/repo"]
    }
  }
}
```

#### stdio 常见坑

**1. Server 日志打到 stdout**

```go
// ❌ 错误：日志污染 stdout
fmt.Println("Server starting...")  // 这会破坏 JSON-RPC 解析

// ✅ 正确：日志走 stderr
fmt.Fprintln(os.Stderr, "Server starting...")
```

**2. Client 不读取 stdin**

```go
// ❌ 错误：Server 收不到请求
cmd.Stdin = nil

// ✅ 正确：建立管道
stdin, _ := cmd.StdinPipe()
```

**3. 不处理 EOF**

```go
// Server 必须处理 EOF（Client 关闭连接）
for {
    line, err := reader.ReadBytes('\n')
    if err != nil {
        return  // EOF，正常退出
    }
    // ... 处理请求
}
```

### 6.2 Streamable HTTP

#### 什么是 Streamable HTTP

**Streamable HTTP** 是 MCP 2025-03 规范引入的新传输方式，用于远程服务。

**核心：** 单个 HTTP 端点 (`/mcp`) 处理所有交互

#### 核心设计

**两种响应模式：**

```
Client 发 POST /mcp          Server 可以选择：
─────────────────             ──────────────────────────
 简单请求                →    同步返回 JSON
                          →    OR
 复杂请求                →    开 SSE 流式推送
                          →    progress/log → final response
```

**两种读取方式：**

```
Client 需要接收通知:
─────────────────
 方式 1：               方式 2：
 POST /mcp              GET /mcp (空)
 → 响应中携带通知         → text/event-stream
                        → 持续接收 Server 推送
```

#### HTTP 方法

| 操作            | 方法            | Body        | 响应                      |
| ------------- | ------------- | ----------- | ----------------------- |
| Client 发请求/通知 | `POST /mcp`   | JSON-RPC 消息 | 普通 JSON 或 SSE 流         |
| Client 监听通知   | `GET /mcp`    | (空)         | `text/event-stream` SSE |
| 关闭会话          | `DELETE /mcp` | (空)         | `204 No Content`        |

#### POST /mcp 详细说明

**请求：**

```
POST /mcp
Content-Type: application/json
Accept: application/json, text/event-stream
Mcp-Session-Id: xyz (可选，会话 ID)

{JSON-RPC 消息}
```

**响应方式 1：同步 JSON（简单请求）**

```
HTTP/1.1 200 OK
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"result":{...}}
```

**响应方式 2：SSE 流（复杂请求）**

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Mcp-Session-Id: xyz

event: message
data: {"progress":30,"message":"处理中..."}

event: message
data: {"progress":60,"message":"继续处理..."}

event: message
data: {"jsonrpc":"2.0","id":1,"result":{...}}
```

#### GET /mcp 详细说明

**请求：**

```
GET /mcp
Accept: text/event-stream
Mcp-Session-Id: xyz (可选)

(空 Body)
```

**响应：**

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: message
data: {"jsonrpc":"2.0","method":"notifications/resources/updated",...}

event: message
data: {"jsonrpc":"2.0","method":"notifications/progress",...}
```

#### 会话管理

**Mcp-Session-Id Header：**

- Server 在 `initialize` 响应中下发（如果选择有状态会话）
- Client 后续所有请求都带上此 Header
- Server 重启后 session 失效，Client 重新握手

**示例：**

```json
// Server 在 initialize 响应中下发
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "serverInfo": {"name": "api-server", "version": "1.0.0"},
    "capabilities": {...}
  }
}

// 响应 Headers
Mcp-Session-Id: abc123-def456-ghi789
```

```http
// Client 后续请求带上 session ID
POST /mcp HTTP/1.1
Content-Type: application/json
Mcp-Session-Id: abc123-def456-ghi789

{...}
```

#### 鉴权

**Streamable HTTP 是 OAuth 2.0 资源服务器：**

- 支持 Bearer token
- 支持 Resource Indicators（RFC 8707）
- 跨网络时鉴权是**必需项**

**示例：**

```http
POST /mcp HTTP/1.1
Content-Type: application/json
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...

{...}
```

**对比 stdio：**

- stdio 不需要鉴权（本地子进程，环境变量传 token）
- HTTP 跨网络，鉴权是必需项

#### 适用场景

✅ **适合 Streamable HTTP：**

- 远程 MCP 服务（SaaS、企业公共服务）
- 多 Host 共享
- 云上托管的 MCP Server
- 需要鉴权的场景

❌ **不适合 Streamable HTTP：**

- 本地工具（文件系统、shell）
- 单进程 Host
- 无网络环境

### 6.3 传输方式对比

| 维度           | stdio            | Streamable HTTP       |
| ------------ | ---------------- | --------------------- |
| **适用场景**     | 本地工具             | 远程服务                  |
| **通信方式**     | 子进程 stdin/stdout | HTTP POST/GET         |
| **消息格式**     | 换行符分隔的 JSON      | JSON body 或 SSE       |
| **鉴权**       | 不需要（本地）          | 必需（OAuth 2.0）         |
| **会话管理**     | 进程生命周期           | Mcp-Session-Id Header |
| **多 Client** | 不支持（1:1）         | 支持（共享 Server）         |
| **跨网络**      | 不支持              | 支持                    |
| **复杂度**      | 低                | 中                     |
| **性能**       | 高（无网络开销）         | 中（HTTP 开销）            |
| **调试难度**     | 低                | 中（需抓包）                |

### 6.4 如何选择

```
本地工具 / 桌面集成 / 单进程 Host
        ↓
      使用 stdio

远程服务 / 多 Host 共享 / 公网鉴权
        ↓
  使用 Streamable HTTP
```

**决策树：**

```
是否需要跨网络？
├─ 否 → stdio
└─ 是 → 是否需要多 Host 共享？
        ├─ 否 → 仍可 stdio（通过 SSH 隧道）
        └─ 是 → Streamable HTTP
```

***

## 7. 协议对比

### 7.1 MCP vs OpenAPI

| 维度       | MCP                     | OpenAPI          |
| -------- | ----------------------- | ---------------- |
| **形态**   | JSON-RPC 2.0            | HTTP + JSON/YAML |
| **类型**   | 双向 RPC + 通知             | 单向请求响应           |
| **能力发现** | 协议内置 `*/list`           | 靠 spec 文件        |
| **能力演进** | capabilities 协商         | 版本号 + 文档         |
| **传输**   | stdio / Streamable HTTP | HTTP             |
| **适用场景** | AI 应用连接外部能力             | 通用 Web API       |

**关键区别：**

- MCP 是为"AI 应用连接外部工具和上下文"专门设计的
- OpenAPI 是通用 Web API 描述规范
- MCP 内置能力发现和协商，OpenAPI 需要额外工具

### 7.2 MCP vs gRPC

| 维度        | MCP          | gRPC              |
| --------- | ------------ | ----------------- |
| **形态**    | JSON-RPC 2.0 | HTTP/2 + Protobuf |
| **类型**    | 双向 RPC + 通知  | 双向 RPC + 流        |
| **类型系统**  | 动态 JSON      | 静态 Protobuf       |
| **人类可读性** | 高（JSON）      | 低（二进制）            |
| **跨语言**   | 高（JSON）      | 高（Protobuf）       |
| **性能**    | 中（JSON 解析）   | 高（二进制编码）          |
| **适用场景**  | AI 应用工具对接    | 微服务 RPC           |

**关键区别：**

- MCP 强调人类可读性和跨语言简便性
- gRPC 强调性能和强类型
- MCP 内置通知机制，gRPC 需要流

### 7.3 MCP vs 传统插件

| 维度       | MCP           | 传统插件     |
| -------- | ------------- | -------- |
| **协议**   | 标准化（JSON-RPC） | 各家自创     |
| **能力发现** | 协议内置          | 离线文档     |
| **跨框架**  | 是（一次对接）       | 否（每框架一套） |
| **生态系统** | 统一            | 碎片化      |
| **成熟度**  | 快速成长          | 成熟但分裂    |

**关键区别：**

- MCP 是行业标准，传统插件是各产品自定义
- MCP 一次实现，到处复用

***

## 8. 实战示例

### 8.1 完整握手流程

```json
// ========== 1. Client 发起握手 ==========

// Client → Server
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "clientInfo": {
      "name": "my-agent",
      "version": "1.0.0"
    },
    "capabilities": {
      "roots": {
        "listChanged": true
      }
    }
  }
}

// ========== 2. Server 回应 ==========

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "serverInfo": {
      "name": "weather-server",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {
        "listChanged": true
      },
      "resources": {
        "subscribe": true,
        "listChanged": true
      }
    }
  }
}

// ========== 3. Client 通知握手完成 ==========

// Client → Server（通知，无 id）
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}

// ========== 握手完成，进入操作阶段 ==========
```

### 8.2 工具调用完整流程

```json
// ========== 1. 列出工具 ==========

// Client → Server
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "get_weather",
        "description": "查询城市天气",
        "inputSchema": {
          "type": "object",
          "properties": {
            "city": {"type": "string"}
          },
          "required": ["city"]
        }
      }
    ]
  }
}

// ========== 2. 调用工具 ==========

// Client → Server
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {
      "city": "北京"
    }
  }
}

// Server → Client（成功）
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "北京：晴，15-25℃，湿度 45%"
      }
    ],
    "isError": false
  }
}

// Server → Client（工具错误）
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "错误：无效的城市名称"
      }
    ],
    "isError": true
  }
}
```

### 8.3 资源订阅流程

```json
// ========== 1. 订阅资源 ==========

// Client → Server
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/subscribe",
  "params": {
    "uri": "file:///docs/api.md"
  }
}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {}
}

// ========== 2. 资源更新通知 ==========

// Server → Client（异步通知）
{
  "jsonrpc": "2.0",
  "method": "notifications/resources/updated",
  "params": {
    "uri": "file:///docs/api.md"
  }
}

// ========== 3. 读取更新后的资源 ==========

// Client → Server
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "resources/read",
  "params": {
    "uri": "file:///docs/api.md"
  }
}

// Server → Client
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "contents": [
      {
        "uri": "file:///docs/api.md",
        "mimeType": "text/markdown",
        "text": "# API 文档\n\n..."
      }
    ]
  }
}
```

### 8.4 stdio vs Streamable HTTP 对比

#### stdio 示例

```
# Server 代码（Go）
func main() {
    r := bufio.NewReader(os.Stdin)
    w := os.Stdout

    for {
        line, err := r.ReadBytes('\n')
        if err != nil {
            return  // EOF，退出
        }

        var req struct {
            ID     *int            `json:"id"`
            Method string          `json:"method"`
            Params json.RawMessage `json:"params"`
        }
        json.Unmarshal(line, &req)

        switch req.Method {
        case "initialize":
            reply(w, req.ID, map[string]any{...})
        case "tools/list":
            reply(w, req.ID, map[string]any{...})
        case "tools/call":
            reply(w, req.ID, map[string]any{...})
        }
    }
}

func reply(w io.Writer, id *int, result any) {
    if id == nil {
        return  // 通知，不回复
    }
    data, _ := json.Marshal(map[string]any{
        "jsonrpc": "2.0",
        "id": *id,
        "result": result,
    })
    fmt.Fprintf(w, "%s\n", data)
}
```

**启动方式：**

```bash
# 直接作为子进程启动
./my-mcp-server

# 或通过 npm/npx
npx @modelcontextprotocol/server-filesystem /path
```

#### Streamable HTTP 示例

**HTTP 请求：**

```http
POST /mcp HTTP/1.1
Host: api.example.com
Content-Type: application/json
Accept: application/json, text/event-stream
Authorization: Bearer <token>
Mcp-Session-Id: abc123

{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {"city": "北京"}
  }
}
```

**同步响应：**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{"type": "text", "text": "北京：晴"}],
    "isError": false
  }
}
```

**流式响应（SSE）：**

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Mcp-Session-Id: abc123

event: message
data: {"progress":30,"message":"查询天气..."}

event: message
data: {"progress":60,"message":"处理结果..."}

event: message
data: {"jsonrpc":"2.0","id":1,"result":{...}}
```

***

## 9. 关键设计理念

### 9.1 能力协商优先

**而不是单纯版本号：**

- `initialize` 时双方互报 `capabilities`
- 老 Client 遇到新 Server，只要新能力不破坏老能力，就仍能工作
- 向前兼容

**示例：**

```
Server 支持: tools, resources, prompts, logging
Client 支持: tools, resources

协商结果：使用 tools 和 resources，忽略 prompts 和 logging
```

### 9.2 单向通知是一等公民

**取消、进度、日志、列表变化**都可以自然表达，不需要轮询。

**优势：**

- 实时性
- 减少轮询开销
- 表达力强

### 9.3 协议不规定执行环境

**Server 可以用任何语言/框架实现：**

- Go
- Python
- Node.js
- Rust
- 本地进程或云服务

**Client 可以用任何语言/框架：**

- 只要实现 JSON-RPC 2.0 和传输层

### 9.4 工具结果和协议错误分离

**工具错误 →** **`isError=true`** **的 result**

- SQL 语法错误
- 参数验证失败
- 业务逻辑错误

**协议错误 → JSON-RPC error**

- 方法不存在
- 参数格式错误
- 解析失败

***

## 10. 总结

### MCP 的核心价值

1. **标准化**：统一的协议规范
2. **解耦**：工具实现与 Agent 框架分离
3. **复用**：一次实现，到处使用
4. **生态**：共享工具和资源

### 三要素

1. **三角色**：Host / Client / Server
2. **四原语**：Tools / Prompts / Resources / Logging
3. **两传输**：stdio / Streamable HTTP

### 五阶段

1. **握手**：交换能力声明
2. **发现**：列出可用工具/资源/提示词
3. **调用**：执行工具/读取资源
4. **通知**：进度、日志、更新
5. **关闭**：清理资源

### 设计原则

- **中立抽象**：JSON-RPC 2.0 信封
- **能力协商**：向前兼容
- **通知优先**：取消、进度、日志
- **传输正交**：同一套消息，多种传输

***

## 参考资源

- [MCP 官方规范](https://modelcontextprotocol.io/)
- [MCP GitHub](https://github.com/modelcontextprotocol)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- [MCP Inspector](https://github.com/modelcontextprotocol/inspector)


# KP01 — 基础类型（core 包）

## 标题和概述

`core` 包是整个 llmLib 的**类型基石**，定义了跨 Provider、Agent、Router 等模块共享的核心数据结构。它承担以下职责：

- 描述 LLM 调用所需的连接配置（`LLMConfig`）和函数式选项（`ChatOption`）
- 定义对话消息模型（`Message`、`Role`、`ToolCall`）及其 JSON 序列化规则
- 规范请求/响应结构（`ChatRequest`、`ChatResponse`、`Usage`、`StreamChunk`）
- 提供工具定义（`ToolDef`、`ToolFunction`）供上游协议和工具系统共享
- 实现辅助工具：Token 估算、JSON 安全序列化、通道处理、Provider 常量

## 核心概念

### 1. LLMConfig — 连接配置

`LLMConfig` 是一次模型调用的最小配置单元，包含 **BaseURL**、**APIKey**、**Model** 三个必备字段，以及用于路由层成本估算的 `InputPricePerMillion` / `OutputPricePerMillion` 和延迟排序用的 `LatencyMS`。

### 2. ChatOption — 函数式选项

`ChatOption` 是 `func(*LLMConfig)` 类型，通过 `WithModel`、`WithBaseURL`、`WithAPIKey`、`WithInputPrice`、`WithOutputPrice`、`WithLatencyMS` 等高阶函数实现配置的灵活补丁式注入。

### 3. Role — 消息角色枚举

四种角色：`user`、`system`、`assistant`、`tool`。`Message.MarshalJSON()` 根据 Role 自动适配不同上游协议（如 tool 角色需要附加 `tool_call_id`）。

### 4. Message / ToolCall — 消息与工具调用

`Message` 支持两种扩展字段：
- `ToolCalls []ToolCall`：Assistant 消息携带的结构化工具调用
- `ToolCallID`：Tool 角色消息关联的调用 ID

`ToolCall` 的 `Args` 字段使用 `json.RawMessage`，在 `MarshalJSON` 时会转换为 OpenAI 协议要求的字符串格式。

### 5. 请求 / 响应结构

- `ChatRequest`：`Model` + `Messages` + `Stream` + `Tools`
- `ChatResponse`：`Content` + `ToolCalls` + `FinishReason` + `InputTokens` / `OutputTokens`
- `Usage`：独立的 Token 用量统计
- `StreamChunk`：流式响应的增量片段，包含 `Content`、`ToolCalls` 和 `Err`

### 6. Token 估算与预算

- `EstimateTokens(s)`：按 ASCII/中文字符比例粗估 token 数（ASCII 约 1/4 token，CJK 约 2/3 token）
- `Budget`：为系统提示词、工具定义、历史对话、检索内容分配 token 预算

### 7. 通道处理工具

- `Process[T]`：通用 channel 消费循环，支持 context 取消和错误短路
- `Collect[T]`：基于 `Process` 的便捷聚合函数，将 channel 内容收集为切片

### 8. Provider 常量

`constants.go` 定义了 8 个 Provider 名称常量和 3 个通用环境变量键；`p_*.go` 文件为每个 Provider 提供专属的 API Key、BaseURL、默认模型名常量。

## 类型/函数清单

### 配置相关
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `LLMConfig` | `core/config.go` | 模型调用连接配置 |
| `ChatOption` | `core/chat_option.go` | 函数式选项类型 (`func(*LLMConfig)`) |
| `WithModel(model)` | `core/chat_option.go` | 指定模型名称 |
| `WithBaseURL(baseURL)` | `core/chat_option.go` | 覆盖默认接口地址 |
| `WithAPIKey(apiKey)` | `core/chat_option.go` | 注入 API Key |
| `WithInputPrice(price)` | `core/chat_option.go` | 设置输入 token 单价 |
| `WithOutputPrice(price)` | `core/chat_option.go` | 设置输出 token 单价 |
| `WithLatencyMS(latency)` | `core/chat_option.go` | 设置预估延迟 |

### 消息相关
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Role` (type) | `core/role.go` | 消息角色枚举 |
| `User` / `System` / `Assistant` / `ToolRole` | `core/role.go` | 角色常量 |
| `Message` | `core/message.go` | 消息结构（含 ToolCalls、ToolCallID） |
| `ToolCall` | `core/message.go` | 工具调用结构（ID、Name、Args、Result） |
| `openaiToolCall` | `core/message.go` | OpenAI 兼容序列化格式 |
| `Message.MarshalJSON()` | `core/message.go` | 自定义序列化（按 Role 适配协议） |
| `NewMessage(role, content)` | `core/message_new.go` | 通用消息构造器 |
| `NewUserMessage(content)` | `core/message_new.go` | 用户消息快捷构造 |
| `NewSystemMessage(content)` | `core/message_new.go` | 系统消息快捷构造 |
| `NewAssistantMessage(content)` | `core/message_new.go` | 助手消息快捷构造 |

### 请求/响应相关
| 类型 | 源文件 | 说明 |
|---|---|---|
| `ChatRequest` | `core/req_resp.go` | 聊天请求体 |
| `ChatResponse` | `core/req_resp.go` | 聊天响应体 |
| `Usage` | `core/req_resp.go` | Token 用量统计 |
| `StreamChunk` | `core/stream.go` | 流式响应增量片段 |

### 工具定义相关
| 类型 | 源文件 | 说明 |
|---|---|---|
| `ToolDef` | `core/tool_def.go` | 工具定义（Type + Function） |
| `ToolFunction` | `core/tool_def.go` | 工具函数详情（Name、Description、Parameters） |

### 辅助工具
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `EstimateTokens(s)` | `core/token.go` | Token 粗估 |
| `Budget` | `core/token.go` | Token 预算结构 |
| `SafeJSON(v)` | `core/safejson.go` | 安全 JSON 序列化 |
| `Process[T](ctx, ch, handler)` | `core/channel.go` | 通用 channel 消费循环 |
| `Collect[T](ctx, ch)` | `core/channel.go` | Channel 聚合为切片 |

### 常量
| 常量组 | 源文件 | 说明 |
|---|---|---|
| `API_KEY` / `BASE_URL` / `MODEL` | `core/constants.go` | 通用环境变量键 |
| `ProviderOpenAI` ~ `ProviderQwen` | `core/constants.go` | 8 个 Provider 名称常量 |
| `*_API_KEY` / `*_BASE_URL` / `*_MODEL_ENV` / `*_BASEURL` / `*_DEFAULT_MODEL` | `core/p_*.go` | 各 Provider 专属常量（8 个文件） |

## 使用示例

```go
// 1. 构造消息
messages := []core.Message{
    core.NewSystemMessage("你是一个有用的助手"),
    core.NewUserMessage("你好"),
}

// 2. 配置调用参数
cfg := core.LLMConfig{
    BaseURL:   core.DEEPSEEK_BASEURL,
    APIKey:    os.Getenv(core.DEEPSEEK_API_KEY),
    Model:     core.DEEPSEEK_DEFAULT_MODEL,
}

// 3. 使用 ChatOption 灵活覆盖
ctx := context.Background()
// 通过 WithModel 等选项补丁式修改
cfg2 := cfg
core.WithModel("deepseek-reasoner")(&cfg2)

// 4. Token 估算与预算
budget := core.Budget{
    Total:        128000,
    SystemPrompt: 2000,
    Tools:        5000,
    History:      50000,
    Retrieved:    20000,
}
estimated := core.EstimateTokens("你好，世界！Hello, World!")

// 5. StreamChunk 通道消费
ch := make(chan core.StreamChunk)
go func() {
    ch <- core.StreamChunk{Content: "你好"}
    ch <- core.StreamChunk{Content: "世界"}
    close(ch)
}()
result, err := core.Collect[core.StreamChunk](ctx, ch)

// 6. SafeJSON 安全序列化
jsonStr := core.SafeJSON(map[string]any{"key": "value"})
```

## 关联知识点

- **KP02-Provider体系**：`Provider` 接口接收 `LLMConfig` 和 `[]Message`，返回 `ChatResponse` 或 `StreamChunk` channel
- **KP03-错误处理**：`StreamChunk.Err` 字段携带的错误由 `errutil` 包进行分类和重试策略决策
- **KP04-工具系统**：`ToolDef` / `ToolFunction` 由 `tool.BuildToolDefs()` 从工具注册表构建，注入 `ChatRequest.Tools`
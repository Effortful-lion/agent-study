# KP02 — Provider 体系（provider 包）

## 标题和概述

`provider` 包是 llmLib 的**多模型适配层**，通过统一的 `Provider` / `ToolCallProvider` 接口抽象不同 LLM 服务商的协议差异。它实现了：

- 8 个主流 LLM 服务商的 Provider 实现（DeepSeek、豆包、Claude、OpenAI、智谱、通义、Kimi、Qwen）
- OpenAI 兼容协议的同步/流式调用（覆盖除 Claude 外的所有 Provider）
- Claude 原生消息协议的同步/流式调用
- SSE（Server-Sent Events）解析器，统一处理流式响应
- `transport` 包提供可复用的 HTTP 客户端基础设施

## 核心概念

### 1. Provider 接口

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (*core.ChatResponse, error)
    ChatStream(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (<-chan core.StreamChunk, error)
}
```

这是所有 Provider 的最小接口，覆盖同步（`Chat`）和流式（`ChatStream`）两种调用模式。

### 2. ToolCallProvider 接口

```go
type ToolCallProvider interface {
    Provider
    ChatWithTools(ctx, cfg, messages, tools) (*core.ChatResponse, error)
    ChatStreamWithTools(ctx, cfg, messages, tools) (<-chan core.StreamChunk, error)
}
```

扩展了工具调用能力，接受 `[]core.ToolDef` 参数，允许模型在生成过程中调用外部工具。**所有 8 个 Provider 均实现此接口**。

### 3. Provider 注册与工厂

`NewProvider(name string)` 通过 switch-case 工厂方法按名称创建 Provider 实例，支持 8 种 Provider 名称常量。这是整个库的 Provider 唯一入口点。

### 4. OpenAI 兼容协议

`OpenAIChat` / `OpenAIChatStream` 是 7 个 Provider（除 Claude 外）共享的底层实现。它们：
- 将 `ChatRequest` 序列化为 JSON
- POST 到 `{BaseURL}/chat/completions`
- 携带 `Authorization: Bearer {APIKey}` 和 `Accept: application/json`（或 `text/event-stream`）
- 解析响应并映射为统一的 `ChatResponse` / `StreamChunk`

### 5. Claude 原生协议

`ClaudeChat` / `ClaudeChatStream` 使用 Claude 专有协议：
- 端点：`{BaseURL}/v1/messages`
- 鉴权：`x-api-key` 头部
- 工具定义使用 `input_schema` 而非 `parameters`
- 响应内容为 `content` 数组（支持 `text` 和 `tool_use` 两种类型）

### 6. SSE 解析器

`ParseSSE(r io.Reader, onData)` 是一个通用的 SSE 事件解析器：
- 使用 `bufio.Scanner` 逐行读取，支持最大 1MB 的单行缓冲
- 累积多行 `data:` 字段，空行触发事件分发
- 跳过注释行（以 `:` 开头）
- 将拼装后的 data 段回调给上层处理函数

### 7. 增量解析

- `parseOpenAIDelta(data)`：从 OpenAI SSE `data` 负载中提取文本增量和 `[DONE]` 标记
- `parseOpenAIDeltaWithTools(data)`：扩展版，同时提取工具调用增量
- `normalizeArgs(args)`：处理工具参数的 JSON 字符串↔对象互转

### 8. Transport 传输层

`transport.NewClient()` 创建带 120s 默认超时的 HTTP 客户端，支持 `WithTimeout`、`WithTLSConfig`、`WithTransport` 三种可选配置。

## 类型/函数清单

### 接口
| 接口 | 源文件 | 说明 |
|---|---|---|
| `Provider` | `provider/provider.go` | 基础 Provider 接口（Name / Chat / ChatStream） |
| `ToolCallProvider` | `provider/provider.go` | 扩展工具调用的 Provider 接口 |

### Provider 实现
| 结构体 | 源文件 | 协议 |
|---|---|---|
| `DeepSeekProvider` | `provider/provider.go` | OpenAI 兼容 |
| `DoubaoProvider` | `provider/provider.go` | OpenAI 兼容 |
| `ClaudeProvider` | `provider/provider.go` | Claude 原生 |
| `OpenAIProvider` | `provider/provider.go` | OpenAI 兼容 |
| `ZhipuProvider` | `provider/provider.go` | OpenAI 兼容 |
| `TongyiProvider` | `provider/provider.go` | OpenAI 兼容 |
| `KimiProvider` | `provider/provider.go` | OpenAI 兼容 |
| `QwenProvider` | `provider/provider.go` | OpenAI 兼容 |

### 工厂函数
| 函数 | 源文件 | 说明 |
|---|---|---|
| `NewProvider(name)` | `provider/provider.go` | 按名称创建 Provider |
| `NewDeepSeekProvider()` | `provider/provider.go` | 创建 DeepSeek Provider |
| `NewDoubaoProvider()` | `provider/provider.go` | 创建豆包 Provider |
| `NewClaudeProvider()` | `provider/provider.go` | 创建 Claude Provider |
| `NewOpenAIProvider()` | `provider/provider.go` | 创建 OpenAI Provider |
| `NewZhipuProvider()` | `provider/provider.go` | 创建智谱 Provider |
| `NewTongyiProvider()` | `provider/provider.go` | 创建通义 Provider |
| `NewKimiProvider()` | `provider/provider.go` | 创建 Kimi Provider |
| `NewQwenProvider()` | `provider/provider.go` | 创建 Qwen Provider |

### OpenAI 协议适配器
| 函数 | 源文件 | 说明 |
|---|---|---|
| `OpenAIChat(ctx, cfg, messages)` | `provider/chat_openai.go` | 同步 OpenAI 兼容聊天 |
| `OpenAIChatWithTools(ctx, cfg, messages, tools)` | `provider/chat_openai.go` | 带工具的同步聊天 |
| `OpenAIChatStream(ctx, cfg, messages)` | `provider/chat_openai.go` | 流式 OpenAI 兼容聊天 |
| `OpenAIChatStreamWithTools(ctx, cfg, messages, tools)` | `provider/chat_openai.go` | 带工具的流式聊天 |
| `normalizeArgs(args)` | `provider/chat_openai.go` | 工具参数规范化 |

### Claude 协议适配器
| 函数 | 源文件 | 说明 |
|---|---|---|
| `ClaudeChat(ctx, cfg, messages)` | `provider/chat_claude.go` | 同步 Claude 聊天 |
| `ClaudeChatWithTools(ctx, cfg, messages, tools)` | `provider/chat_claude.go` | 带工具的同步 Claude 聊天 |
| `ClaudeChatStream(ctx, cfg, messages)` | `provider/chat_claude.go` | 流式 Claude 聊天 |
| `ClaudeChatStreamWithTools(ctx, cfg, messages, tools)` | `provider/chat_claude.go` | 带工具的流式 Claude 聊天 |

### SSE 解析
| 函数 | 源文件 | 说明 |
|---|---|---|
| `ParseSSE(r, onData)` | `provider/parse_sse.go` | 通用 SSE 解析器 |
| `parseOpenAIDelta(data)` | `provider/parse_data.go` | OpenAI 增量解析（仅文本） |
| `parseOpenAIDeltaWithTools(data)` | `provider/parse_data.go` | OpenAI 增量解析（文本+工具） |

### 传输层
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `ClientOption` | `transport/transport.go` | HTTP 客户端配置函数类型 |
| `NewClient(opts...)` | `transport/transport.go` | 创建带默认超时的 HTTP 客户端 |
| `WithTimeout(timeout)` | `transport/transport.go` | 自定义超时 |
| `WithTLSConfig(cfg)` | `transport/transport.go` | 注入 TLS 配置 |
| `WithTransport(transport)` | `transport/transport.go` | 替换 RoundTripper |

## 使用示例

```go
import (
    "context"
    "github.com/Effortful-lion/agent-study/llmLib/core"
    "github.com/Effortful-lion/agent-study/llmLib/provider"
)

// 1. 通过工厂创建 Provider
p, err := provider.NewProvider(core.ProviderDeepSeek)
if err != nil {
    panic(err)
}

// 2. 构造配置（实际使用时应从环境变量读取）
cfg := core.LLMConfig{
    BaseURL: core.DEEPSEEK_BASEURL,
    APIKey:  "your-api-key",
    Model:   core.DEEPSEEK_DEFAULT_MODEL,
}

// 3. 准备消息
messages := []core.Message{
    core.NewSystemMessage("你是一个数学助手"),
    core.NewUserMessage("计算 23 * 47"),
}

ctx := context.Background()

// 4. 同步调用
resp, err := p.Chat(ctx, cfg, messages)
if err != nil {
    // 通过 errutil.ClassifyError 判断错误类型
    // 参考 KP03-错误处理
    panic(err)
}
fmt.Println(resp.Content)

// 5. 流式调用
stream, err := p.ChatStream(ctx, cfg, messages)
if err != nil {
    panic(err)
}
for chunk := range stream {
    if chunk.Err != nil {
        break
    }
    fmt.Print(chunk.Content)
}

// 6. 带工具调用（ToolCallProvider）
if tcp, ok := p.(provider.ToolCallProvider); ok {
    tools := []core.ToolDef{ /* 从 tool 包构建 */ }
    toolResp, err := tcp.ChatWithTools(ctx, cfg, messages, tools)
    // 处理 toolResp.ToolCalls...
}

// 7. 直接使用底层协议函数
resp2, err := provider.OpenAIChat(ctx, cfg, messages)

// 8. 使用 transport 自定义客户端
client := transport.NewClient(
    transport.WithTimeout(60*time.Second),
    transport.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS13}),
)
```

## 关联知识点

- **KP01-基础类型**：Provider 接口消费 `LLMConfig`、`Message`、`ToolDef`，生产 `ChatResponse`、`StreamChunk`
- **KP03-错误处理**：Provider 返回的错误通过 `errutil.ClassifyError` 分类，`RetryWithBackoff` 进行重试
- **KP04-工具系统**：`ToolCallProvider` 接口接收 `[]core.ToolDef`，与 `tool.BuildToolDefs()` 配合使用
- **Router 模块**：`router` 包基于 `Provider` 接口实现多 Provider 路由和策略选择
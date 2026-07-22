# llmlib

一个 LLM 开发的标准库，提供统一的接口来调用各种 AI 服务商的 API。
查看go.doc文档：
1. 浏览器：go install golang.org/x/pkgsite/cmd/pkgsite@latest && pkgsite -http :6060 && 访问：http://localhost:6060
2. 命令行：go doc llmlib

## 特性

- **多服务商支持**: OpenAI、DeepSeek、Doubao、Claude、Zhipu、Tongyi、Kimi
- **两种调用方式**: 同步调用和流式调用
- **统一接口**: 所有服务商使用相同的消息格式和响应结构
- **便捷配置**: 内置默认 API URL，支持自定义配置
- **Token 估算**: 内置字符串 token 数量估算功能
- **自定义客户端**: 支持配置 HTTP 客户端超时、TLS 等

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "your-project/llmLib"
)

func main() {
    apiKey := os.Getenv("DEEPSEEK_API_KEY")
    
    // 同步调用
    resp, err := llmlib.Chat(context.Background(), "deepseek", apiKey, []llmlib.Message{
        llmlib.NewUserMessage("你好"),
    }, llmlib.WithModel("deepseek-chat"))
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Content)
    
    // 流式调用
    stream, err := llmlib.ChatStream(context.Background(), "deepseek", apiKey, []llmlib.Message{
        llmlib.NewUserMessage("讲一个故事"),
    }, llmlib.WithModel("deepseek-chat"))
    if err != nil {
        panic(err)
    }
    for chunk := range stream {
        if chunk.Err != nil {
            panic(chunk.Err)
        }
        fmt.Print(chunk.Content)
    }
}
```

## 安装

```bash
go get your-project/llmLib
```

## 使用方式

### 基础用法

```go
// 创建消息
messages := []llmlib.Message{
    llmlib.NewSystemMessage("你是一个助手"),
    llmlib.NewUserMessage("你好"),
}

// 调用 Chat 接口
resp, err := llmlib.Chat(ctx, "provider-name", apiKey, messages)
```

### 支持的服务商

| 服务商 | providerName | 默认 BaseURL |
|--------|--------------|--------------|
| DeepSeek | `deepseek` | https://api.deepseek.com |
| Doubao | `doubao` | https://ark.cn-beijing.volces.com/api/v3 |
| Claude | `claude` | https://api.anthropic.com |
| OpenAI | `openai` | https://api.openai.com/v1 |
| Zhipu | `zhipu` | https://open.bigmodel.cn/api/paas/v4 |
| Tongyi | `tongyi` | https://dashscope.aliyuncs.com/compatible-mode/v1 |
| Kimi | `kimi` | https://api.moonshot.cn/v1 |

### 选项配置

```go
// 使用选项配置
resp, err := llmlib.Chat(ctx, "deepseek", apiKey, messages,
    llmlib.WithModel("deepseek-chat"),
    llmlib.WithBaseURL("https://custom-api.example.com"),
)
```

### 自定义 HTTP 客户端

```go
client := llmlib.NewClient(
    llmlib.WithTimeout(60*time.Second),
    llmlib.WithTLSConfig(&tls.Config{...}),
)
```

## API 参考

### 核心函数

#### Chat

```go
func Chat(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (*ChatResponse, error)
```

发送同步聊天请求。

**参数**:
- `providerName`: 服务商名称
- `apiKey`: API 密钥
- `messages`: 消息列表
- `opts`: 可选配置项

**返回**:
- `*ChatResponse`: 聊天响应
- `error`: 错误信息

#### ChatStream

```go
func ChatStream(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (<-chan StreamChunk, error)
```

发送流式聊天请求，返回一个 channel 用于接收响应。

### 消息创建函数

```go
llmlib.NewMessage(role, content)      // 创建消息
llmlib.NewUserMessage(content)        // 创建用户消息
llmlib.NewSystemMessage(content)      // 创建系统消息
llmlib.NewAssistantMessage(content)   // 创建助手消息
```

### 选项函数

```go
llmlib.WithModel(model)         // 设置模型名称
llmlib.WithBaseURL(baseURL)     // 设置 API 基础 URL
llmlib.WithAPIKey(apiKey)       // 设置 API 密钥
```

### 数据结构

#### Message

```go
type Message struct {
    Role    Role   // 消息角色
    Content string // 消息内容
}
```

#### Role

```go
const (
    llmlib.User      // 用户角色
    llmlib.System    // 系统角色
    llmlib.Assistant // 助手角色
)
```

#### ChatResponse

```go
type ChatResponse struct {
    Content      string // 回复内容
    InputTokens  int    // 输入 token 数
    OutputTokens int    // 输出 token 数
}
```

#### StreamChunk

```go
type StreamChunk struct {
    Content string // 内容片段
    Err     error  // 错误信息
}
```

## 扩展新服务商

要添加新的服务商，需要：

1. 在 `provider.go` 中创建新的 Provider 结构体并实现 `Provider` 接口
2. 在 `NewProvider` 函数中添加 case
3. 如果协议不同，实现对应的聊天函数
4. 在 `baseurl.go` 中添加默认的 BaseURL

## 开发人员指南

### 代码组织结构

```
llmlib/
├── llmlib.go          # 统一入口，导出核心功能和便捷函数
├── provider.go        # Provider 接口定义及各服务商实现
├── config.go          # LLMConfig 配置结构体
├── chat_openai.go     # OpenAI 风格协议实现（可被多个服务商复用）
├── chat_claude.go     # Claude 风格协议实现
├── req-resp.go        # 请求/响应数据结构
├── message.go         # Message 消息结构
├── role.go            # Role 角色定义
├── stream.go          # 流式响应处理工具
├── baseurl.go         # 默认 API BaseURL 常量
├── token.go           # Token 估算功能
├── transport.go       # HTTP 客户端配置
├── router.go          # 路由调度器、策略和故障转移
├── env.go             # .env 加载和服务批量初始化
├── parse_data.go      # SSE 数据解析工具
└── parse_sse.go       # SSE 协议解析
```

### 模块依赖关系

```
┌─────────────────────────────────────────────────────────────┐
│                      用户入口层                              │
│  llmlib.go (Chat/ChatStream/NewMessage/WithXxx)            │
│  env.go (LoadAll/LoadDotEnv)                               │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                      核心抽象层                              │
│  provider.go (Provider 接口 + 各服务商实现)                  │
│  config.go (LLMConfig)                                     │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                      协议实现层                              │
│  chat_openai.go (OpenAIChat/OpenAIChatStream)              │
│  chat_claude.go (ClaudeChat/ClaudeChatStream)              │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                      基础工具层                              │
│  stream.go (StreamChunk/Process/Collect)                   │
│  token.go (estimateTokens)                                 │
│  transport.go (NewClient/ClientOption)                     │
│  parse_data.go (ParseSSE/parseOpenAIDelta)                 │
└─────────────────────────────────────────────────────────────┘
```

### 内部函数说明

#### router.go 内部函数

| 函数名 | 功能说明 |
|--------|----------|
| `estimateCost(cfg, resp)` | 根据配置的价格和响应的 token 数估算调用成本 |
| `selectStrategy(strategy, services)` | 根据策略选择服务顺序 |
| `strategyDefault(services)` | 默认策略：保持原顺序 |
| `strategyCheapestFirst(services)` | 最便宜优先：按价格升序排列 |
| `strategyLowestLatency(services)` | 最低延迟优先：按延迟升序排列 |
| `sendRouteStreamChunk(ctx, out, chunk)` | 安全发送流式数据块，处理 ctx 取消 |
| `sendRouteStreamErr(ctx, out, err)` | 发送错误到流式通道 |
| `formatRouteErrors(errs)` | 格式化路由错误信息，包含诊断建议 |
| `routeErrorsAsErrors(errs)` | 将 routeError 转换为普通 error 切片 |
| `diagnoseError(err)` | 诊断错误类型并给出建议 |
| `percentile(samples, p)` | 计算分位值 |

#### env.go 内部函数

| 函数名 | 功能说明 |
|--------|----------|
| `loadProviderNames()` | 从环境变量 LLM_PROVIDERS 加载服务商名称列表 |
| `findProviderMeta(name)` | 根据名称查找服务商元信息 |
| `loadProviderConfig(provider)` | 从环境变量加载单个服务商配置 |
| `loadLatencyMS(provider)` | 加载延迟配置（支持环境变量覆盖默认值） |

#### parse_data.go 内部函数

| 函数名 | 功能说明 |
|--------|----------|
| `parseOpenAIDelta(data)` | 解析 OpenAI 风格的 SSE delta 数据 |

#### token.go 内部函数

| 函数名 | 功能说明 |
|--------|----------|
| `estimateTokens(s)` | 估算字符串的 token 数量（英文约4字符/token，中文约1.5~2字符/token） |

#### llmlib.go 内部函数

| 函数名 | 功能说明 |
|--------|----------|
| `getDefaultBaseURL(providerName)` | 根据服务商名称获取默认 BaseURL |

### 内部类型说明

| 类型名 | 说明 |
|--------|------|
| `routeError` | 路由错误，包含服务信息和错误 |
| `providerMeta` | 服务商元信息，包含环境变量名、默认值、价格、延迟等 |

### 添加新服务商步骤

```go
// 1. 在 provider.go 中创建新的 Provider 结构体
type MyProvider struct{}

// 2. 实现 Provider 接口
func NewMyProvider() *MyProvider { return &MyProvider{} }
func (p *MyProvider) Name() string { return "myprovider" }
func (p *MyProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
    return OpenAIChat(ctx, cfg, messages)
}
func (p *MyProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
    return OpenAIChatStream(ctx, cfg, messages)
}

// 3. 在 NewProvider 函数中添加 case
// 4. 在 baseurl.go 中添加默认 BaseURL
// 5. 在 env.go 的 providerMetas 中添加元信息
```

### 开发规范

1. **GoDoc 注释**：所有导出函数和类型必须有 GoDoc 注释
2. **内部函数注释**：内部函数和类型也应该有注释说明用途
3. **Option Pattern**：使用 option pattern 处理可选参数
4. **错误处理**：错误信息应包含上下文，使用 `fmt.Errorf("prefix: %w", err)`
5. **并发安全**：涉及共享状态的操作需要加锁
6. **流式处理**：流式处理需要处理 ctx 取消和 channel 关闭

### 测试建议

- 使用 `go test` 运行单元测试
- 使用 `go doc llmlib` 查看导出 API 文档
- 使用 `go doc -u llmlib` 查看所有文档（包括内部函数）
- 使用 `pkgsite -http :6060` 启动本地文档服务器

### 扩展点

- **Provider 接口**：添加新服务商
- **Strategy**：添加新路由策略
- **ClientOption**：添加新的 HTTP 客户端配置选项
- **LLMConfig**：添加新的配置字段

## 许可证

MIT
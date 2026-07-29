# KP07 — 多 Provider 路由（router 包）

## 标题和概述

`router` 包是 llmLib 的 **多 Provider 智能路由系统**，为应用层提供统一的多 LLM 服务管理能力。它负责：

- 聚合多个 LLM Provider（豆包、DeepSeek、Zhipu、Tongyi、Kimi、Claude、OpenAI、Qwen 等）
- 基于策略（默认/最便宜/最低延迟）自动排序和选择 Provider
- 延迟指标收集（P50/P95 统计），为路由决策提供数据支持
- 环境变量自动加载，支持 `.env` 文件配置
- `RouterAdapter` 适配器模式，使 `Router` 可直接作为 `provider.Provider` 使用

## 核心概念

### 1. Strategy — 路由策略

```go
type Strategy int

const (
    StrategyDefault       Strategy = iota  // 默认：按注册顺序
    StrategyCheapestFirst                  // 优先选择最便宜的 Provider
    StrategyLowestLatency                  // 优先选择最低延迟的 Provider
)
```

| 策略 | 排序依据 | 说明 |
|---|---|---|
| `StrategyDefault` | 注册顺序 | 按 `LoadAll` 返回的顺序排列 |
| `StrategyCheapestFirst` | `InputPricePerMillion + OutputPricePerMillion` | 总成本最低优先 |
| `StrategyLowestLatency` | `LatencyMS` | 配置的延迟最低优先 |

### 2. LLMService — LLM 服务封装

```go
type LLMService struct {
    Provider provider.Provider  // LLM Provider 实例
    Config   core.LLMConfig    // 模型配置（包含定价、延迟等元数据）
}
```

每个 `LLMService` 封装一个 Provider 及其配置，是路由调度的基本单元。

### 3. RouteResult — 路由结果

```go
type RouteResult struct {
    Provider   string              // Provider 名称
    Model      string              // 模型名称
    Response   *core.ChatResponse  // 模型响应
    Cost       float64             // 估算成本（美元）
    Latency    LatencySnapshot     // 延迟快照
    LastErrors []error             // 最后一个 Provider 的错误
}
```

路由结果包含完整的调用信息，包括成本估算和延迟统计，便于监控和日志记录。

### 4. LatencySnapshot — 延迟快照

```go
type LatencySnapshot struct {
    Samples int           // 采样数量
    P50     time.Duration // 50 分位延迟
    P95     time.Duration // 95 分位延迟
}
```

### 5. LatencyMetrics — 延迟指标收集器

```go
type LatencyMetrics struct {
    records []time.Duration  // 延迟记录（内部使用）
}
```

提供两个方法：
- `Record(d)`：记录一次调用延迟
- `Snapshot()`：计算并返回 P50/P95 延迟快照

### 6. Router — 核心路由器

```go
type Router struct {
    services []LLMService    // 可用服务列表
    strategy Strategy        // 路由策略
    metrics  *LatencyMetrics // 延迟指标
}
```

**核心方法**：`Chat(ctx, messages)` 按策略排序后依次尝试各个 Provider，成功即返回，失败则尝试下一个。

### 7. RouterAdapter — 适配器

```go
type RouterAdapter struct {
    router *Router
}
```

使 `Router` 适配 `provider.Provider` 接口：
- `Chat(ctx, cfg, messages)`：委托给 `Router.Chat`，返回 `RouteResult.Response`
- `ChatStream`：返回不支持错误（路由模式下不支持流式）

### 8. 环境变量配置系统

`env.go` 实现了一套基于环境变量的 Provider 自动发现机制：

**支持的 Provider 及其环境变量**：

| Provider | API Key 变量 | Base URL 变量 | Model 变量 | 默认模型 |
|---|---|---|---|---|
| 豆包 (Doubao) | `DOUBAO_API_KEY` | `DOUBAO_BASE_URL` | `DOUBAO_MODEL` | `doubao-pro-32k` |
| DeepSeek | `DEEPSEEK_API_KEY` | `DEEPSEEK_BASE_URL` | `DEEPSEEK_MODEL` | `deepseek-chat` |
| Zhipu | `ZHIPU_API_KEY` | `ZHIPU_BASE_URL` | `ZHIPU_MODEL` | `glm-4` |
| Tongyi | `TONGYI_API_KEY` | `TONGYI_BASE_URL` | `TONGYI_MODEL` | `qwen-max` |
| Kimi | `KIMI_API_KEY` | `KIMI_BASE_URL` | `KIMI_MODEL` | `moonshot-v1-8k` |
| Claude | `CLAUDE_API_KEY` | `CLAUDE_BASE_URL` | `CLAUDE_MODEL` | `claude-3-sonnet` |
| OpenAI | `OPENAI_API_KEY` | `OPENAI_BASE_URL` | `OPENAI_MODEL` | `gpt-4o` |
| Qwen | `QWEN_API_KEY` | `QWEN_BASE_URL` | `QWEN_MODEL` | `qwen-max` |

**路由策略环境变量**：`LLM_ROUTER_STRATEGY=cheapest|latency|default`

### 9. 加载函数

| 函数 | 说明 |
|---|---|
| `LoadAll()` | 从系统环境变量加载所有已配置的 Provider |
| `LoadAllWithEnv(envPath)` | 先加载 `.env` 文件，再从环境变量加载 |
| `LoadDotEnv()` | 加载当前目录的 `.env` 文件 |
| `LoadDotEnvFromPath(path)` | 加载指定路径的 `.env` 文件 |

`loadDotEnvFile` 支持：
- `KEY=value` 格式
- 引号包裹的值（`KEY="value"`）
- 注释行（以 `#` 开头）
- `\r\n` 和 `\n` 换行

### 10. 成本估算

```go
func estimateCost(resp *core.ChatResponse, cfg core.LLMConfig) float64 {
    inputCost := float64(resp.InputTokens) / 1_000_000 * cfg.InputPricePerMillion
    outputCost := float64(resp.OutputTokens) / 1_000_000 * cfg.OutputPricePerMillion
    return inputCost + outputCost
}
```

根据 `core.LLMConfig` 中配置的输入/输出价格（每百万 Token 美元价）估算调用成本。

## 类型/函数清单

### 路由核心类型
| 类型 | 源文件 | 说明 |
|---|---|---|
| `Strategy` | `router/router.go` | 路由策略枚举 |
| `StrategyDefault` | `router/router.go` | 默认策略常量 |
| `StrategyCheapestFirst` | `router/router.go` | 最便宜优先策略常量 |
| `StrategyLowestLatency` | `router/router.go` | 最低延迟策略常量 |
| `LLMService` | `router/router.go` | LLM 服务封装 |
| `RouteResult` | `router/router.go` | 路由结果 |
| `RouteStreamChunk` | `router/router.go` | 流式路由结果块 |
| `LatencySnapshot` | `router/router.go` | 延迟快照 |
| `LatencyMetrics` | `router/router.go` | 延迟指标收集器 |
| `Router` | `router/router.go` | 核心路由器 |
| `RouterAdapter` | `router/router.go` | Provider 适配器 |

### Router 方法
| 函数/方法 | 源文件 | 说明 |
|---|---|---|
| `NewRouter(services, strategy)` | `router/router.go` | 创建路由器 |
| `(*Router).Chat(ctx, messages)` | `router/router.go` | 执行路由调用 |
| `(*Router).sortServices()` | `router/router.go` | 按策略排序服务 |
| `estimateCost(resp, cfg)` | `router/router.go` | 估算调用成本 |

### LatencyMetrics 方法
| 函数/方法 | 源文件 | 说明 |
|---|---|---|
| `NewLatencyMetrics()` | `router/router.go` | 创建延迟指标收集器 |
| `(*LatencyMetrics).Record(d)` | `router/router.go` | 记录延迟 |
| `(*LatencyMetrics).Snapshot()` | `router/router.go` | 获取延迟快照 |

### RouterAdapter 方法
| 函数/方法 | 源文件 | 说明 |
|---|---|---|
| `NewRouterAdapter(r)` | `router/router.go` | 创建适配器 |
| `(*RouterAdapter).Name()` | `router/router.go` | 返回名称 `"router-adapter"` |
| `(*RouterAdapter).Chat(ctx, cfg, messages)` | `router/router.go` | 委托调用 |
| `(*RouterAdapter).ChatStream(ctx, cfg, messages)` | `router/router.go` | 返回不支持 |

### 环境变量加载
| 函数 | 源文件 | 说明 |
|---|---|---|
| `LoadAll()` | `router/env.go` | 从环境加载所有 Provider |
| `LoadAllWithEnv(envPath)` | `router/env.go` | 加载 .env 文件后再加载 Provider |
| `LoadDotEnv()` | `router/env.go` | 加载 `.env` 文件 |
| `LoadDotEnvFromPath(path)` | `router/env.go` | 加载指定 .env 文件 |
| `loadDotEnvFile(path)` | `router/env.go` | 解析并加载 .env 文件（内部） |
| `loadAllFromEnv()` | `router/env.go` | 遍历 providerMetas 加载（内部） |
| `ReadStrategyFromEnv()` | `router/env.go` | 从环境变量读取路由策略 |
| `ProviderConfigHelp()` | `router/env.go` | 返回环境变量配置帮助文本 |
| `splitLines(s)` | `router/env.go` | 按行分割字符串（内部） |
| `trimSpace(s)` | `router/env.go` | 去除首尾空格（内部） |

## 使用示例

### 基础路由使用

```go
import (
    "context"
    "github.com/Effortful-lion/agent-study/llmLib/core"
    "github.com/Effortful-lion/agent-study/llmLib/router"
)

// 1. 从环境变量加载所有 Provider 服务
services, err := router.LoadAll()
if err != nil {
    panic(err)
}

// 2. 按策略创建路由器
r := router.NewRouter(services, router.StrategyCheapestFirst)

// 3. 执行路由调用
ctx := context.Background()
messages := []core.Message{
    {Role: core.System, Content: "你是一个助手"},
    {Role: core.User, Content: "你好"},
}

result, err := r.Chat(ctx, messages)
if err != nil {
    panic(err)
}

fmt.Printf("Provider: %s\n", result.Provider)
fmt.Printf("Model: %s\n", result.Model)
fmt.Printf("Cost: $%.4f\n", result.Cost)
fmt.Printf("P50 Latency: %v\n", result.Latency.P50)
fmt.Printf("P95 Latency: %v\n", result.Latency.P95)
fmt.Printf("Answer: %s\n", result.Response.Content)
```

### 加载 .env 文件

```go
// 方式一：加载当前目录 .env
router.LoadDotEnv()

// 方式二：加载指定路径
router.LoadDotEnvFromPath("/path/to/.env")

// 方式三：一体化加载（先 .env 再 Provider）
services, err := router.LoadAllWithEnv(".env")
```

`.env` 文件示例：

```env
# 路由策略
LLM_ROUTER_STRATEGY=cheapest

# 豆包
DOUBAO_API_KEY=your-doubao-key
DOUBAO_MODEL=doubao-pro-32k

# DeepSeek
DEEPSEEK_API_KEY=your-deepseek-key

# Zhipu
ZHIPU_API_KEY=your-zhipu-key
```

### 使用 RouterAdapter 作为 Provider

```go
// 将 Router 适配为 provider.Provider 接口
adapter := router.NewRouterAdapter(r)

// 现在 adapter 可以用在任何需要 Provider 的地方
// 例如传给 Agent
a := agent.New(adapter, "auto", registry,
    agent.WithAgentAPIKey("dummy"), // 适配器内部自行处理
)
```

### 延迟指标监控

```go
// 创建独立的延迟指标收集器
metrics := router.NewLatencyMetrics()

// 手动记录延迟
metrics.Record(150 * time.Millisecond)
metrics.Record(200 * time.Millisecond)
metrics.Record(180 * time.Millisecond)

// 获取快照
snapshot := metrics.Snapshot()
fmt.Printf("Samples: %d, P50: %v, P95: %v\n",
    snapshot.Samples, snapshot.P50, snapshot.P95)
```

### 读取环境变量中的策略

```go
// 从环境变量读取策略
strategy := router.ReadStrategyFromEnv()
r := router.NewRouter(services, strategy)
```

### 获取配置帮助

```go
help := router.ProviderConfigHelp()
fmt.Println(help)
// 输出所有支持的 Provider 环境变量说明
```

## 关联知识点

- **KP01-基础类型**：`LLMService.Config` 使用 `core.LLMConfig`，`RouteResult.Response` 使用 `core.ChatResponse`，`core.StreamChunk` 用于流式结果
- **KP02-Provider体系**：`LLMService.Provider` 使用 `provider.Provider` 接口，`RouterAdapter` 实现了 `provider.Provider` 的 `Chat` 和 `ChatStream` 方法
- **KP05-Agent运行时**：`RouterAdapter` 可作为 Agent 的 Provider，实现多 Provider 自动切换的 Agent 运行时
- **KP08-命令行参数**：可通过 `command` 包注册 `LLM_ROUTER_STRATEGY` 等参数，实现命令行级别的路由策略选择
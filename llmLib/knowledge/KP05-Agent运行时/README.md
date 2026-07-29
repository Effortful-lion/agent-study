# KP05 — Agent 运行时（agent 包）

## 标题和概述

`agent` 包是 llmLib 的 **Agent 运行时引擎**，实现了基于状态机的 **Think-Act-Observe** 循环架构。它负责：

- `Agent` 结构体作为核心运行时，编排 `Provider`（模型）和 `tool.Registry`（工具）的交互循环
- 四阶段状态机（`Thinking` → `Acting` → `Done`/`Error`）驱动 Agent 的每一步决策
- `State` 完整快照支持 JSON 序列化，配合 `Store`/`FileStore` 实现会话持久化
- `AgentEvent` 事件流机制，将 Agent 内部的每一步操作实时暴露给调用方
- `AgentBudgetConfig` 预算控制系统，防止死循环、超时和成本失控
- 内置 Plan-and-Execute 模式支持任务的拓扑排序和并行执行

## 核心概念

### 1. Agent 结构体

```go
type Agent struct {
    p            provider.Provider      // LLM 提供者
    model        string                  // 模型名称
    apiKey       string                  // API 密钥
    baseURL      string                  // API 基础 URL
    tools        *tool.Registry          // 工具注册表
    systemPrompt string                  // 系统提示词
    budget       AgentBudgetConfig       // 预算配置
    store        Store                   // 状态持久化存储
    sessionID    string                  // 会话 ID
    memory       *State                  // 内存中的状态
}
```

`Agent` 是整个运行时的入口，通过 `New(p, model, registry, opts...)` 创建。所有配置通过 `Option` 函数式选项模式传入。

### 2. Option — 配置函数

```go
type Option func(*Agent)
```

可用的 Option 包括：
- `WithSystemPrompt(prompt)` — 自定义系统提示词
- `WithAgentBudgetConfig(budget)` — 设置预算配置
- `WithAgentAPIKey(apiKey)` — 设置 API 密钥
- `WithAgentBaseURL(baseURL)` — 设置 API 基础 URL
- `WithStore(store, sessionID)` — 启用状态持久化

### 3. 状态机四阶段

| 阶段 | 常量 | 说明 |
|---|---|---|
| `PhaseThinking` | `"thinking"` | 调用模型思考，获取回复和工具调用指令 |
| `PhaseActing` | `"acting"` | 执行模型返回的工具调用，收集结果 |
| `PhaseDone` | `"done"` | Agent 正常完成，输出最终答案 |
| `PhaseError` | `"error"` | Agent 遇到错误，异常终止 |

### 4. Think-Act-Observe 循环

```
PhaseThinking → PhaseActing → PhaseThinking → ... → PhaseDone
     │              │
     └── 失败 → PhaseError
```

- **Think**：调用 `stepThink()` 向模型发送消息（包含系统提示、用户目标、历史对话、工具描述），解析模型回复中的工具调用
- **Act**：调用 `stepAct()` 依次执行每个工具调用，将结果追加到对话历史
- **Observe**：每一步结束时通过 `checkpoint()` 持久化状态，然后回到 `Thinking` 阶段
- **智能重试**：当模型在第一步未调用工具时，自动追加提示要求使用工具；当有未调用的工具时，检查是否需要继续

### 5. State — 运行状态快照

```go
type State struct {
    Goal         string            // 用户目标
    Messages     []core.Message    // 对话历史
    Step         int               // 当前步骤编号
    Phase        Phase             // 当前阶段
    Answer       string            // 最终答案
    Usage        core.Usage        // Token 用量统计
    ActionCounts map[string]int    // 动作重复计数（防死循环）
    StartedAt    time.Time         // 开始时间
    UpdatedAt    time.Time         // 最近更新时间
    Metadata     map[string]string // 元数据（存储中间状态）
    GoalAdded    bool              // 目标是否已添加
}
```

`State` 是一次 Agent 运行的完整快照，支持 JSON 序列化。`Messages` 数组保存了完整的对话历史（系统消息、用户消息、助手消息、工具消息）。

### 6. Store 接口与 FileStore

```go
type Store interface {
    Save(ctx context.Context, sessionID string, st *State) error
    Load(ctx context.Context, sessionID string) (*State, error)
}
```

`FileStore` 是基于文件系统的实现，将会话状态保存为 JSON 文件：
- `NewFileStore(dir)` 创建文件存储
- 文件路径：`dir/<sessionID>.json`
- 使用 `os.WriteFile` / `os.ReadFile` 读写
- 支持会话恢复：`Load` 后可从中断处继续执行

### 7. AgentEvent — 事件系统

```go
type AgentEvent struct {
    Type EventType  // 事件类型
    Text string     // 事件文本内容
    Tool string     // 工具名（工具调用相关事件）
    Args string     // 工具参数
    Step int        // 当前步骤编号
}
```

### 8. EventType 常量

| 常量 | 值 | 说明 |
|---|---|---|
| `EventStepStart` | `"step_start"` | 步骤开始 |
| `EventStepEnd` | `"step_end"` | 步骤结束 |
| `EventModelCall` | `"model_call"` | 模型调用开始 |
| `EventModelResponse` | `"model_response"` | 模型回复内容 |
| `EventToolCall` | `"tool_call"` | 工具调用开始 |
| `EventToolResult` | `"tool_result"` | 工具执行结果 |
| `EventThought` | `"thought"` | 模型思考过程 |
| `EventAnswer` | `"answer"` | 最终答案 |
| `EventError` | `"error"` | 错误事件 |
| `EventDone` | `"done"` | Agent 完成 |

### 9. AgentBudgetConfig — 预算控制

```go
type AgentBudgetConfig struct {
    MaxSteps         int           // 最大步数（0 = 不限制）
    MaxTotalTokens   int           // 最大 Token 总数
    MaxDuration      time.Duration // 最大运行时长
    MaxRetries       int           // 工具调用最大重试次数
    MaxActionRetries int           // 同一动作最大重复次数
}
```

默认配置（`DefaultAgentBudgetConfig()`）：
- `MaxSteps: 10`，`MaxTotalTokens: 100000`，`MaxDuration: 5分钟`
- `MaxRetries: 3`，`MaxActionRetries: 3`

### 10. Plan-and-Execute — 任务规划与执行

`plan_execute.go` 实现了完整的 Plan-and-Execute 架构：

- **`Task`**：包含 `ID`、`Description`、`DependsOn`（依赖列表）、`Action`（执行函数）
- **`Plan`**：任务的有序集合
- **`Levels(plan)`**：对任务进行拓扑排序，返回层级列表（检测重复 ID、不存在的依赖、循环依赖）
- **`Execute(ctx, plan)`**：按拓扑排序的层级并行执行任务，同一层内任务并发执行，依赖结果通过 `map[string]any` 传递

## 类型/函数清单

### Agent 核心
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Agent` | `agent/agent.go` | Agent 运行时主结构体 |
| `Option` | `agent/agent.go` | 配置函数类型 `func(*Agent)` |
| `New(p, model, registry, opts...)` | `agent/agent.go` | 创建 Agent 实例 |
| `WithSystemPrompt(prompt)` | `agent/agent.go` | 设置系统提示词 |
| `WithAgentBudgetConfig(budget)` | `agent/agent.go` | 设置预算配置 |
| `WithAgentAPIKey(apiKey)` | `agent/agent.go` | 设置 API 密钥 |
| `WithAgentBaseURL(baseURL)` | `agent/agent.go` | 设置 API URL |
| `WithStore(store, sessionID)` | `agent/agent.go` | 启用状态持久化 |
| `Agent.Run(ctx, goal)` | `agent/agent.go` | 启动 Agent，返回事件流 |

### 状态机与持久化
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Phase` | `agent/agent_state.go` | 阶段类型（`thinking`/`acting`/`done`/`error`） |
| `State` | `agent/agent_state.go` | 运行状态快照 |
| `Store` | `agent/agent_state.go` | 状态持久化接口 |
| `FileStore` | `agent/agent_state.go` | 文件系统存储实现 |
| `NewFileStore(dir)` | `agent/agent_state.go` | 创建 FileStore |
| `FileStore.Save(ctx, sessionID, state)` | `agent/agent_state.go` | 保存状态到文件 |
| `FileStore.Load(ctx, sessionID)` | `agent/agent_state.go` | 从文件加载状态 |

### 事件系统
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `EventType` | `agent/agent_event.go` | 事件类型 |
| `AgentEvent` | `agent/agent_event.go` | 事件结构体 |
| `EventStepStart` ~ `EventDone` | `agent/agent_event.go` | 10 个事件类型常量 |

### 预算控制
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `AgentBudgetConfig` | `agent/agent_budget.go` | 预算配置结构体 |
| `DefaultAgentBudgetConfig()` | `agent/agent_budget.go` | 默认预算配置 |
| `ShouldStop(state)` | `agent/agent_budget.go` | 判断是否应停止 |
| `ShouldRetry(actionKey, counts)` | `agent/agent_budget.go` | 判断动作是否可重试 |

### Plan-and-Execute
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Task` | `agent/plan_execute.go` | 可执行任务 |
| `Plan` | `agent/plan_execute.go` | 任务集合 |
| `Levels(plan)` | `agent/plan_execute.go` | 拓扑排序分层 |
| `Execute(ctx, plan)` | `agent/plan_execute.go` | 并行执行计划 |

## 使用示例

### 基础 Agent 使用

```go
import (
    "context"
    "github.com/Effortful-lion/agent-study/llmLib/agent"
    "github.com/Effortful-lion/agent-study/llmLib/provider"
    "github.com/Effortful-lion/agent-study/llmLib/tool"
)

// 1. 创建 Provider 和工具注册表
p, _ := provider.NewProvider("doubao")
registry := tool.NewRegistryToolSet()
registry.Register(&tool.CalculatorTool{})
registry.Register(&tool.TimeTool{})

// 2. 创建 Agent
a := agent.New(p, "doubao-pro-32k", registry,
    agent.WithAgentAPIKey("your-api-key"),
    agent.WithAgentBaseURL("https://ark.cn-beijing.volces.com/api/v3"),
)

// 3. 启动 Agent 并消费事件流
ctx := context.Background()
events, err := a.Run(ctx, "现在是几点？计算 123 * 456")
if err != nil {
    panic(err)
}

for event := range events {
    switch event.Type {
    case agent.EventStepStart:
        fmt.Printf("▶ [%d] %s\n", event.Step, event.Text)
    case agent.EventModelResponse:
        fmt.Printf("  💭 %s\n", event.Text)
    case agent.EventToolCall:
        fmt.Printf("  🔧 调用工具: %s(%s)\n", event.Tool, event.Args)
    case agent.EventToolResult:
        fmt.Printf("  📦 结果: %s\n", event.Text)
    case agent.EventAnswer:
        fmt.Printf("  ✅ 答案: %s\n", event.Text)
    case agent.EventDone:
        fmt.Println("🏁 Agent 完成")
    }
}
```

### 带状态持久化的 Agent

```go
// 启用 FileStore 实现会话恢复
store := agent.NewFileStore("./sessions")
sessionID := "user-001"

a := agent.New(p, model, registry,
    agent.WithAgentAPIKey("your-key"),
    agent.WithStore(store, sessionID),
)

// 首次运行
events, _ := a.Run(ctx, "帮我分析这份数据")
for event := range events { /* ... */ }

// 后续恢复（从上次断点继续）
events2, _ := a.Run(ctx, "继续完成未完成的任务")
```

### 自定义预算和系统提示词

```go
a := agent.New(p, model, registry,
    agent.WithAgentAPIKey("your-key"),
    agent.WithSystemPrompt("你是一个专业的数据分析助手，擅长处理结构化数据。"),
    agent.WithAgentBudgetConfig(agent.AgentBudgetConfig{
        MaxSteps:         20,
        MaxTotalTokens:   200000,
        MaxDuration:      10 * time.Minute,
        MaxRetries:       5,
        MaxActionRetries: 4,
    }),
)
```

### Plan-and-Execute 任务编排

```go
plan := agent.Plan{
    Tasks: []agent.Task{
        {
            ID:          "fetch_data",
            Description: "获取数据",
            DependsOn:   nil,
            Action: func(ctx context.Context, inputs map[string]any) (any, error) {
                return fetchData(), nil
            },
        },
        {
            ID:          "analyze",
            Description: "分析数据",
            DependsOn:   []string{"fetch_data"},
            Action: func(ctx context.Context, inputs map[string]any) (any, error) {
                data := inputs["fetch_data"]
                return analyze(data), nil
            },
        },
        {
            ID:          "generate_report",
            Description: "生成报告",
            DependsOn:   []string{"analyze"},
            Action: func(ctx context.Context, inputs map[string]any) (any, error) {
                analysis := inputs["analyze"]
                return generateReport(analysis), nil
            },
        },
    },
}

// 拓扑排序并执行（同层任务并行）
results, err := agent.Execute(ctx, plan)
// results["fetch_data"], results["analyze"], results["generate_report"]
```

## 关联知识点

- **KP01-基础类型**：`State.Messages` 使用 `core.Message` 结构体，`AgentBudgetConfig` 引用 `core.Usage`
- **KP02-Provider体系**：`Agent` 持有 `provider.Provider`，通过 `callModel` 调用 `Chat` 或 `ChatWithTools`
- **KP03-错误处理**：工具执行使用 `errutil.NewAgentError` 创建错误，支持分类和可重试标记
- **KP04-工具系统**：`Agent` 持有 `*tool.Registry`，通过 `parseReActToolCalls` 和 `executeTool` 管理工具调用
- **KP06-Agent模式**：`Agent` 的 Think-Act-Observe 循环是一种有环图模式，可与 Chain/Router 等模式组合使用
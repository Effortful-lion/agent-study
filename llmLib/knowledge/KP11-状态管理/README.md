# KP11 — 状态管理（state 包）

## 标题和概述

`state` 包是 llmLib 的 **通用状态机和工作流执行引擎**，提供了两套互补的执行抽象。它负责：

- `Machine` 有限状态机：管理状态转换、条件分支和回调执行
- `Workflow` 链式工作流：提供 `Do(step1, step2, ...)` 风格的顺序执行 API
- `Step` 可组合的执行单元：支持重试、超时、日志等配置
- `Middleware` 中间件体系：实现横切关注点（重试、超时、panic 恢复、日志）
- 与 llmLib 的关系：Agent 运行时使用 `state.Workflow` 组织 Think-Act-Observe 循环，`pattern` 包使用 `state` 实现 5 大 Agent 范式

## 核心概念

### 1. Phase — 状态标识

```go
type Phase string
```

使用 `string` 类型方便序列化和调试，如 `"thinking"`、`"` acting`"、`"observing"`、`"done"`。

### 2. StepFunc — 步骤执行函数

```go
type StepFunc func(ctx context.Context, state map[string]any) (map[string]any, error)
```

接收当前状态，返回新的状态和可能的错误。是整个包的核心执行单元类型。

### 3. Middleware — 中间件

```go
type Middleware func(next StepFunc) StepFunc
```

包装 `StepFunc` 实现横切关注点，支持中间件嵌套链。

### 4. Transition — 状态转换

```go
type Transition struct {
    From      Phase         // 源状态
    To        Phase         // 目标状态
    Action    StepFunc      // 转换时执行的动作
    Condition ConditionFunc // 条件判断（nil 表示无条件）
}
```

定义一次从 `From` 到 `To` 的状态跳转，可选执行动作和条件判断。

### 5. MachineDef — 状态机定义

```go
type MachineDef struct {
    Initial     Phase        // 初始状态
    Transitions []Transition // 所有状态转换
    OnError     StepFunc     // 错误处理回调
    Middlewares []Middleware // 全局中间件
}
```

状态机的完整配置，`Validate()` 方法校验定义有效性（初始状态非空、至少一个转换、存在从初始状态出发的转换）。

### 6. Machine — 状态机运行时

```go
type Machine struct {
    def     *MachineDef
    state   map[string]any
    phase   Phase
    mws     []Middleware
    onError StepFunc
    history []Transition  // 执行历史
}
```

核心方法：

| 方法 | 说明 |
|---|---|
| `Run(ctx)` | 启动状态机，执行到终态（当前状态无匹配转换）或出错 |
| `RunUntil(ctx, target)` | 执行到指定状态为止 |
| `Phase()` | 返回当前状态 |
| `State()` | 返回当前状态数据 |
| `History()` | 返回执行历史 |

### 7. TransitionBuilder — 流畅转换构建器

```go
T(from).To(to).Do(action).When(condition).Build()
```

提供链式 API 定义状态转换：
- `T(from)`：创建从 `from` 状态出发的构建器
- `To(to)`：指到目标状态（可多次调用定义多个分支）
- `Do(action)`：指定转换时执行的动作
- `When(cond)`：指定条件判断
- `Build()`：返回构建好的转换列表

### 8. Step — 工作流步骤

```go
type Step struct {
    Name        string
    Fn          StepFunc
    MaxRetries  int
    RetryDelay  time.Duration
    MaxDelay    time.Duration
    Timeout     time.Duration
    IsRetryable func(error) bool
}
```

步骤配置选项（`StepOption` 函数式选项模式）：

| 选项 | 说明 |
|---|---|
| `WithRetry(maxRetries, delay)` | 配置重试次数和延迟 |
| `WithRetryPolicy(maxRetries, baseDelay, maxDelay, isRetryable)` | 完整重试策略 |
| `WithTimeout(timeout)` | 配置超时时间 |

### 9. Workflow — 链式工作流

```go
type Workflow struct {
    steps       []*Step
    middlewares []Middleware
    onError     StepFunc
}
```

核心方法：

| 方法 | 说明 |
|---|---|
| `Do(steps...)` | 创建工作流并添加步骤（构造函数风格） |
| `Then(steps...)` | 追加步骤，支持链式调用 |
| `Use(mws...)` | 添加全局中间件 |
| `OnError(fn)` | 设置错误处理回调 |
| `Run(ctx, initialState)` | 按顺序执行所有步骤 |
| `RunWithCallback(ctx, initialState, callback)` | 执行步骤，每步完成后回调 |

### 10. 内置中间件

| 中间件 | 说明 |
|---|---|
| `RetryMiddleware(maxRetries, baseDelay, maxDelay, isRetryable)` | 指数退避重试 |
| `TimeoutMiddleware(timeout)` | 为步骤设置超时 |
| `RecoveryMiddleware()` | 捕获 panic 并恢复 |
| `LogMiddleware(logFn)` | 通过回调函数输出日志 |

## 类型/函数清单

### 基础类型
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Phase` | `state/state.go` | 状态标识类型（`string`） |
| `StepFunc` | `state/state.go` | 步骤执行函数类型 |
| `ConditionFunc` | `state/state.go` | 条件判断函数类型 |
| `Middleware` | `state/state.go` | 中间件函数类型 |
| `Transition` | `state/state.go` | 状态转换定义 |
| `TransitionBuilder` | `state/state.go` | 转换构建器 |
| `MachineDef` | `state/state.go` | 状态机定义 |
| `Machine` | `state/state.go` | 状态机运行时 |

### 状态机方法
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `T(from)` | `state/state.go` | 创建转换构建器 |
| `(*TransitionBuilder).To(to)` | `state/state.go` | 指定目标状态 |
| `(*TransitionBuilder).Do(action)` | `state/state.go` | 指定动作 |
| `(*TransitionBuilder).When(cond)` | `state/state.go` | 指定条件 |
| `(*TransitionBuilder).Build()` | `state/state.go` | 构建转换列表 |
| `(*MachineDef).Validate()` | `state/state.go` | 校验定义有效性 |
| `NewMachine(def, initialState)` | `state/state.go` | 创建状态机实例 |
| `(*Machine).Phase()` | `state/state.go` | 获取当前状态 |
| `(*Machine).State()` | `state/state.go` | 获取当前状态数据 |
| `(*Machine).History()` | `state/state.go` | 获取执行历史 |
| `(*Machine).Run(ctx)` | `state/state.go` | 运行到终态 |
| `(*Machine).RunUntil(ctx, target)` | `state/state.go` | 运行到指定状态 |

### 内置中间件
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `RetryMiddleware(maxRetries, baseDelay, maxDelay, isRetryable)` | `state/state.go` | 重试中间件 |
| `TimeoutMiddleware(timeout)` | `state/state.go` | 超时中间件 |
| `RecoveryMiddleware()` | `state/state.go` | panic 恢复中间件 |
| `LogMiddleware(logFn)` | `state/state.go` | 日志中间件 |

### 工作流类型
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Step` | `state/workflow.go` | 工作流步骤 |
| `StepOption` | `state/workflow.go` | 步骤配置选项（函数式） |
| `WithRetry(maxRetries, delay)` | `state/workflow.go` | 配置重试 |
| `WithRetryPolicy(maxRetries, baseDelay, maxDelay, isRetryable)` | `state/workflow.go` | 完整重试策略 |
| `WithTimeout(timeout)` | `state/workflow.go` | 配置超时 |
| `NewStep(name, fn, opts...)` | `state/workflow.go` | 创建步骤 |
| `Workflow` | `state/workflow.go` | 链式工作流 |
| `Do(steps...)` | `state/workflow.go` | 创建工作流 |
| `(*Workflow).Then(steps...)` | `state/workflow.go` | 追加步骤 |
| `(*Workflow).Use(mws...)` | `state/workflow.go` | 添加中间件 |
| `(*Workflow).OnError(fn)` | `state/workflow.go` | 设置错误处理 |
| `(*Workflow).Run(ctx, initialState)` | `state/workflow.go` | 执行工作流 |
| `(*Workflow).RunWithCallback(ctx, initialState, callback)` | `state/workflow.go` | 带回调执行 |

## 使用示例

### 有限状态机示例：订单状态流转

```go
import (
    "context"
    "fmt"
    "github.com/Effortful-lion/agent-study/llmLib/state"
)

func main() {
    // 1. 定义状态转换
    transitions := []state.Transition{
        {From: "created", To: "paid", Action: payAction},
        {From: "paid", To: "shipped", Action: shipAction},
        {From: "paid", To: "cancelled", Condition: func(s map[string]any) bool {
            return s["cancel"] == true
        }, Action: cancelAction},
        {From: "shipped", To: "completed", Action: completeAction},
        {From: "shipped", To: "refunded", Condition: func(s map[string]any) bool {
            return s["refund"] == true
        }, Action: refundAction},
    }

    // 2. 构建状态机定义
    def := &state.MachineDef{
        Initial:     "created",
        Transitions: transitions,
        OnError: func(ctx context.Context, s map[string]any) (map[string]any, error) {
            fmt.Printf("状态转换出错: %v\n", s["_error"])
            s["status"] = "error"
            return s, nil
        },
        Middlewares: []state.Middleware{
            state.RecoveryMiddleware(),
            state.LogMiddleware(func(phase state.Phase, msg string) {
                fmt.Printf("[状态:%s] %s\n", phase, msg)
            }),
        },
    }

    // 3. 创建并运行状态机
    m, err := state.NewMachine(def, map[string]any{"order_id": "12345"})
    if err != nil {
        panic(err)
    }

    err = m.Run(context.Background())
    if err != nil {
        fmt.Printf("运行出错: %v\n", err)
    }

    fmt.Printf("最终状态: %s\n", m.Phase())
    fmt.Printf("执行历史: %d 步\n", len(m.History()))
}

func payAction(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("执行支付...")
    s["status"] = "paid"
    return s, nil
}

func shipAction(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("执行发货...")
    s["status"] = "shipped"
    return s, nil
}

func cancelAction(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("取消订单...")
    s["status"] = "cancelled"
    return s, nil
}

func completeAction(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("订单完成...")
    s["status"] = "completed"
    return s, nil
}

func refundAction(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("退款处理...")
    s["status"] = "refunded"
    return s, nil
}
```

### 使用 TransitionBuilder 构建

```go
transitions := state.T("created").
    To("paid").Do(payAction).
    To("cancelled").When(func(s map[string]any) bool {
        return s["cancel"] == true
    }).Do(cancelAction).
    Build()
```

### 链式工作流示例：数据处理管道

```go
import (
    "context"
    "fmt"
    "time"
    "github.com/Effortful-lion/agent-study/llmLib/state"
)

func main() {
    // 1. 创建步骤
    fetchStep := state.NewStep("获取数据", fetchData,
        state.WithRetry(3, time.Second),
        state.WithTimeout(30*time.Second),
    )

    transformStep := state.NewStep("转换数据", transformData,
        state.WithTimeout(10*time.Second),
    )

    validateStep := state.NewStep("校验数据", validateData)

    saveStep := state.NewStep("保存结果", saveData,
        state.WithRetry(2, 500*time.Millisecond),
    )

    // 2. 组装工作流
    workflow := state.Do(fetchStep).
        Then(transformStep, validateStep).
        Then(saveStep).
        Use(state.RecoveryMiddleware()).
        OnError(func(ctx context.Context, s map[string]any) (map[string]any, error) {
            fmt.Printf("工作流出错: %v\n", s["_error"])
            s["status"] = "failed"
            return s, nil
        })

    // 3. 执行
    result, err := workflow.Run(context.Background(), map[string]any{
        "source": "api.example.com",
        "dataset": "sales_2024",
    })

    if err != nil {
        fmt.Printf("执行失败: %v\n", err)
    } else {
        fmt.Printf("执行成功: %v\n", result)
    }
}

func fetchData(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Printf("从 %s 获取 %s 数据...\n", s["source"], s["dataset"])
    s["raw_data"] = []string{"record1", "record2", "record3"}
    return s, nil
}

func transformData(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("转换数据格式...")
    s["transformed"] = true
    return s, nil
}

func validateData(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("校验数据完整性...")
    return s, nil
}

func saveData(ctx context.Context, s map[string]any) (map[string]any, error) {
    fmt.Println("保存到数据库...")
    s["saved"] = true
    return s, nil
}
```

### 带回调执行（SSE 推送场景）

```go
workflow := state.Do(step1, step2, step3)

result, err := workflow.RunWithCallback(ctx, nil,
    func(stepName string, state map[string]any, err error) error {
        if err != nil {
            fmt.Printf("步骤 %s 失败: %v\n", stepName, err)
        } else {
            fmt.Printf("步骤 %s 完成\n", stepName)
        }
        // 返回错误可中止工作流
        return nil
    })
```

### 使用 RunUntil 执行到指定状态

```go
m, _ := state.NewMachine(def, initialState)

// 只执行到 "paid" 状态
err := m.RunUntil(context.Background(), "paid")
if err != nil {
    fmt.Printf("执行失败: %v\n", err)
}
fmt.Printf("当前状态: %s\n", m.Phase())
```

## 关联知识点

- **KP05-Agent运行时**：Agent 运行时的 Think-Act-Observe 循环基于 `state.Workflow` 实现，每个步骤对应一次 LLM 调用或工具执行
- **KP06-Agent模式**：`pattern` 包中的 5 大 Agent 范式（ReAct、Plan-and-Execute 等）使用 `state` 包的状态机和工作流作为执行引擎
- **KP10-信号处理**：`SignalContext` 提供的 Context 可直接用于 `Machine.Run()` 和 `Workflow.Run()`，实现信号触发的优雅中止
- **KP09-日志系统**：可通过 `LogMiddleware` 将状态机/工作流的执行日志接入 `lg` 包，实现统一的日志管理
- **KP04-工具系统**：工作流中的 `StepFunc` 可调用 `tool.ToolSet` 中的工具，实现工具编排
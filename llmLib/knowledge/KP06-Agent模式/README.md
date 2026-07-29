# KP06 — Agent 模式（pattern 包）

## 标题和概述

`pattern` 包是 llmLib 的 **Agent 工作模式库**，实现了 AI Agent 的 5 大经典工作范式（参考 Anthropic "Building Effective Agents"）。每个模式都是 `Pattern` 接口的一个实现，内部使用 `state.Workflow` 作为执行引擎。

- **Chain（链式）**：最简单的顺序执行，A → B → C → D
- **Router（路由）**：根据输入条件选择不同的执行分支
- **Parallel（并行）**：多个步骤同时执行，结果聚合
- **Orchestrator（编排者）**：动态规划 + 子任务分发 + 结果合成
- **Evaluator-Optimizer（评估器-优化器）**：生成 → 评估 → 反馈 → 改进的迭代循环

## 核心概念

### 1. Pattern 接口

```go
type Pattern interface {
    Name() string                                                              // 模式名称
    Description() string                                                       // 模式描述
    Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) // 执行模式
}
```

所有 5 种模式都实现此接口，统一了执行入口。`Execute` 返回一个事件通道，调用方可以实时消费执行过程中的事件。

### 2. Event — 事件结构

```go
type Event struct {
    Type    string  // 事件类型：step_start, step_end, tool_call, tool_result, answer, error, done
    Step    int     // 当前步骤编号
    Name    string  // 步骤名称
    Content string  // 步骤内容 / 答案
    Tool    string  // 工具名（tool_call/tool_result 事件）
    Args    string  // 工具参数（tool_call 事件）
    Error   error   // 错误信息（error 事件）
}
```

事件类型与 Agent 的事件系统对齐，便于统一处理。

### 3. Chain — 链式模式

链式模式按顺序执行一系列步骤，上一步的输出作为下一步的输入：

```
Step1 → Step2 → Step3 → ... → StepN
```

**适用场景**：
- 多步推理（先分析问题，再查资料，最后写答案）
- 数据流水线（提取 → 转换 → 加载）
- 固定的多步流程

**核心特性**：
- `AddStep(step)` 添加步骤，支持链式调用
- `OnError(fn)` 设置错误处理回调
- 内部使用 `state.Do(steps...).RunWithCallback()` 作为执行引擎

### 4. Router — 路由模式

路由模式根据输入条件选择不同的执行分支：

```
输入 → 路由函数 → 分支A / 分支B / ... → 执行
```

**适用场景**：
- 根据问题类型选择不同的处理流程
- 意图识别 + 分支处理
- 基于规则的流程分发

**核心特性**：
- 构造时传入 `routerFn func(ctx, state) (string, error)` 路由决策函数
- `Branch(routeKey, steps...)` 注册路由分支
- `Default(steps...)` 设置默认分支
- 未匹配路由时自动使用默认分支

### 5. Parallel — 并行模式

并行模式多个步骤同时执行，最后聚合结果：

```
    ┌── Step1 ──┐
    ├── Step2 ──┤ → Merge → 结果
    └── Step3 ──┘
```

**适用场景**：
- 同时查询多个数据源
- 同时调用多个工具获取不同维度的信息
- 并行处理独立子任务

**核心特性**：
- `AddStep(step)` 添加并行步骤
- `Merge(fn)` 设置结果聚合函数
- 每个步骤自动应用 `RetryMiddleware` 和 `TimeoutMiddleware`（如果配置了的话）
- 使用 `sync.WaitGroup` 等待所有步骤完成

### 6. Orchestrator — 编排者模式

编排者模式动态分解任务，分发给多个工作者并行执行，最后合成结果：

```
Plan（分解任务）→ Worker（并行执行子任务）→ Synthesizer（合成结果）
```

**适用场景**：
- 复杂的多步骤任务（先规划再执行）
- 需要动态决策子任务的场景
- 类似 Plan-and-Execute 的工作模式

**核心特性**：
- `Planner(fn)` 设置规划步骤，返回包含 `"subtasks"` 键的状态
- `Worker(fn)` 设置工作者步骤，每个子任务作为独立状态传入
- `Synthesizer(fn)` 设置合成步骤，接收包含 `"sub_results"` 键的状态
- 子任务支持 `[]map[string]any` 和 `[]any` 两种格式

### 7. Evaluator-Optimizer — 评估器-优化器模式

评估器-优化器模式形成生成 → 评估 → 反馈 → 改进的迭代循环：

```
Generate → Evaluate(score >= threshold?) → 是 → 输出答案
                                    ↓ 否
                              反馈改进 → Generate → ...
```

**适用场景**：
- 代码生成 + 审查 + 修改
- 内容生成 + 质量检查 + 润色
- 自我纠正 / Reflexion 模式

**核心特性**：
- `Generator(fn)` 设置生成步骤
- `EvaluatorFn(fn)` 设置评估步骤，需返回包含 `"score"`（float64）和 `"feedback"`（string）的状态
- `MaxIterations(n)` 设置最大迭代次数（默认 3）
- `Threshold(t)` 设置质量阈值（默认 0.8）

## 类型/函数清单

### Pattern 接口与事件
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Pattern` | `pattern/pattern.go` | 所有模式的统一接口 |
| `Event` | `pattern/pattern.go` | 模式执行事件结构体 |
| `emitPatternEvent(out, event)` | `pattern/chain.go` | 安全发送事件（内部函数） |

### Chain 链式模式
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Chain` | `pattern/chain.go` | 链式模式结构体 |
| `NewChain()` | `pattern/chain.go` | 创建链式模式 |
| `(*Chain).AddStep(step)` | `pattern/chain.go` | 添加步骤 |
| `(*Chain).OnError(fn)` | `pattern/chain.go` | 设置错误处理 |
| `(*Chain).Execute(ctx, state)` | `pattern/chain.go` | 执行链式模式 |

### Router 路由模式
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Router` | `pattern/router.go` | 路由模式结构体 |
| `NewRouter(routerFn)` | `pattern/router.go` | 创建路由模式 |
| `(*Router).Branch(key, steps...)` | `pattern/router.go` | 注册分支 |
| `(*Router).Default(steps...)` | `pattern/router.go` | 设置默认分支 |
| `(*Router).Execute(ctx, state)` | `pattern/router.go` | 执行路由模式 |

### Parallel 并行模式
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Parallel` | `pattern/parallel.go` | 并行模式结构体 |
| `NewParallel()` | `pattern/parallel.go` | 创建并行模式 |
| `(*Parallel).AddStep(step)` | `pattern/parallel.go` | 添加并行步骤 |
| `(*Parallel).Merge(fn)` | `pattern/parallel.go` | 设置聚合函数 |
| `(*Parallel).OnError(fn)` | `pattern/parallel.go` | 设置错误处理 |
| `(*Parallel).Execute(ctx, state)` | `pattern/parallel.go` | 执行并行模式 |

### Orchestrator 编排者模式
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Orchestrator` | `pattern/orchestrator.go` | 编排者模式结构体 |
| `NewOrchestrator()` | `pattern/orchestrator.go` | 创建编排者模式 |
| `(*Orchestrator).Planner(fn)` | `pattern/orchestrator.go` | 设置规划步骤 |
| `(*Orchestrator).Worker(fn)` | `pattern/orchestrator.go` | 设置工作者步骤 |
| `(*Orchestrator).Synthesizer(fn)` | `pattern/orchestrator.go` | 设置合成步骤 |
| `(*Orchestrator).OnError(fn)` | `pattern/orchestrator.go` | 设置错误处理 |
| `(*Orchestrator).Execute(ctx, state)` | `pattern/orchestrator.go` | 执行编排者模式 |

### Evaluator-Optimizer 评估器-优化器模式
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `Evaluator` | `pattern/evaluator.go` | 评估器-优化器模式结构体 |
| `NewEvaluator()` | `pattern/evaluator.go` | 创建评估器模式 |
| `(*Evaluator).Generator(fn)` | `pattern/evaluator.go` | 设置生成步骤 |
| `(*Evaluator).EvaluatorFn(fn)` | `pattern/evaluator.go` | 设置评估步骤 |
| `(*Evaluator).MaxIterations(n)` | `pattern/evaluator.go` | 设置最大迭代次数 |
| `(*Evaluator).Threshold(t)` | `pattern/evaluator.go` | 设置质量阈值 |
| `(*Evaluator).OnError(fn)` | `pattern/evaluator.go` | 设置错误处理 |
| `(*Evaluator).Execute(ctx, state)` | `pattern/evaluator.go` | 执行评估器模式 |

## 使用示例

### Chain 链式模式

```go
import (
    "context"
    "github.com/Effortful-lion/agent-study/llmLib/pattern"
    "github.com/Effortful-lion/agent-study/llmLib/state"
)

// 创建链式模式：分析 → 研究 → 写作
chain := pattern.NewChain().
    AddStep(state.NewStep("analyze", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["analysis"] = "分析结果：这是一道数学题"
        return s, nil
    })).
    AddStep(state.NewStep("research", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["research"] = "查询到相关公式"
        return s, nil
    })).
    AddStep(state.NewStep("write", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["answer"] = "最终答案：42"
        return s, nil
    }))

// 执行并消费事件
events, err := chain.Execute(ctx, map[string]any{"query": "计算 6*7"})
if err != nil {
    panic(err)
}
for event := range events {
    fmt.Printf("[%s] %s\n", event.Type, event.Name)
}
```

### Router 路由模式

```go
// 创建路由模式：根据问题类型选择分支
router := pattern.NewRouter(
    func(ctx context.Context, s map[string]any) (string, error) {
        query, _ := s["query"].(string)
        if strings.Contains(query, "计算") || strings.Contains(query, "多少") {
            return "math", nil
        }
        return "general", nil
    },
).
    Branch("math", state.NewStep("calc", mathStepFn)).
    Branch("general", state.NewStep("answer", generalStepFn)).
    Default(state.NewStep("fallback", fallbackStepFn))

events, _ := router.Execute(ctx, map[string]any{"query": "123 * 456 等于多少？"})
```

### Parallel 并行模式

```go
// 并行获取天气和新闻
parallel := pattern.NewParallel().
    AddStep(state.NewStep("weather", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["weather"] = "北京：25°C，晴"
        return s, nil
    })).
    AddStep(state.NewStep("news", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["news"] = "今日要闻：..."
        return s, nil
    })).
    Merge(func(results map[string]any) (map[string]any, error) {
        merged := map[string]any{
            "weather": results["weather"],
            "news":    results["news"],
        }
        return merged, nil
    })

events, _ := parallel.Execute(ctx, map[string]any{"city": "北京"})
```

### Orchestrator 编排者模式

```go
// 编排者：动态分解任务并并行执行
orchestrator := pattern.NewOrchestrator().
    Planner(func(ctx context.Context, s map[string]any) (map[string]any, error) {
        return map[string]any{
            "subtasks": []map[string]any{
                {"task": "获取数据", "type": "fetch"},
                {"task": "分析数据", "type": "analyze"},
                {"task": "生成报告", "type": "report"},
            },
        }, nil
    }).
    Worker(func(ctx context.Context, s map[string]any) (map[string]any, error) {
        task, _ := s["task"].(string)
        result := executeTask(task)
        return map[string]any{"result": result}, nil
    }).
    Synthesizer(func(ctx context.Context, s map[string]any) (map[string]any, error) {
        subResults, _ := s["sub_results"].([]any)
        finalResult := synthesizeResults(subResults)
        return map[string]any{"final": finalResult}, nil
    })

events, _ := orchestrator.Execute(ctx, map[string]any{"goal": "分析销售数据"})
```

### Evaluator-Optimizer 评估器-优化器模式

```go
// 评估器：生成代码并自我改进
evaluator := pattern.NewEvaluator().
    Generator(func(ctx context.Context, s map[string]any) (map[string]any, error) {
        s["code"] = generateCode(s["prompt"])
        return s, nil
    }).
    EvaluatorFn(func(ctx context.Context, s map[string]any) (map[string]any, error) {
        code, _ := s["code"].(string)
        score, feedback := evaluateCode(code)
        return map[string]any{
            "score":    score,
            "feedback": feedback,
        }, nil
    }).
    MaxIterations(5).
    Threshold(0.9)

events, _ := evaluator.Execute(ctx, map[string]any{"prompt": "写一个排序函数"})
```

## 关联知识点

- **KP01-基础类型**：`Pattern.Execute` 使用 `map[string]any` 作为状态载体，`Event` 结构体的字段设计与 `core` 包的类型系统对齐
- **KP05-Agent运行时**：Agent 的 Think-Act-Observe 循环本身就是一种有环图模式；Chain 可在 Agent 前后添加预处理/后处理步骤；Router 可根据用户意图选择不同的 Agent 配置
- **KP04-工具系统**：模式内部的步骤函数可调用 `tool.Registry` 中的工具，工具事件通过 `Event` 通道传递
- **state 包**：所有模式内部使用 `state.Workflow`（`state.Do(steps...)`）作为执行引擎，步骤函数签名为 `state.StepFunc`（`func(ctx, map[string]any) (map[string]any, error)`）
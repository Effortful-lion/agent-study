# llmLib 模块关系思维导图

## 综合思维导图

```mermaid
mindmap
  root((llmlib 开发库))
    基础层
      KP01-基础类型
        core.LLMConfig
        core.Message/Role
        core.ChatRequest/Response
        core.ToolCall/ToolDef
        core.StreamChunk
      KP02-Provider体系
        provider.Provider 接口
        provider.ToolCallProvider
        8种Provider实现
        NewProvider()
      KP03-错误处理
        errutil.AgentError
        errutil.ErrorCategory
        errutil.RetryWithBackoff
        errutil.ClassifyError
    核心层
      KP04-工具系统
        tool.Tool 接口
        tool.Registry 注册表
        tool.JSONSchemaTool
        tool.CalculatorTool/TimeTool
        tool.BuildToolDefs
      KP05-Agent运行时
        agent.Agent 主体
        agent.AgentEvent 事件
        agent.Store/FileStore 存储
        agent.Phase 阶段
        agent.Task/Plan 任务计划
      KP06-Agent模式
        pattern.Chain 链式
        pattern.Evaluator 评估式
        pattern.Parallel 并行式
        pattern.Orchestrator 编排式
        pattern.Router 路由式
      KP07-多Provider路由
        router.Router 路由器
        router.Strategy 策略
        router.RouterAdapter
        router.LLMService
        router.LoadAll/LoadDotEnv
    应用层
      KP08-命令行参数
        command.CommandBuilder
        command.LoadCommands
        command.Register/Parse
      KP09-日志系统
        lg.Logger 日志器
        lg.Entry 日志记录
        lg.Writer 输出接口
        lg.Router 日志路由
        lg.Frame 框架日志
      KP10-信号处理
        signalx.SignalContext
        优雅关闭机制
      KP11-状态管理
        state.Machine 状态机
        state.Workflow 工作流
        state.Step 步骤
        state.Middleware 中间件
```

## 数据流向图

```mermaid
flowchart TD
    subgraph 应用入口["应用入口"]
        App["用户应用"]
    end

    subgraph llmlib["llmlib.go 根入口"]
        direction TB
        Entry["Chat() / ChatStream()<br/>New() Agent 创建"]
    end

    subgraph 基础层["基础层 Foundation"]
        direction LR
        Core["core 包<br/>基础类型"]
        Provider["provider 包<br/>Provider 体系"]
        ErrUtil["errutil 包<br/>错误处理"]
    end

    subgraph 核心层["核心层 Core"]
        direction LR
        Tool["tool 包<br/>工具系统"]
        Agent["agent 包<br/>Agent 运行时"]
        Pattern["pattern 包<br/>Agent 模式"]
        Router["router 包<br/>多Provider路由"]
    end

    subgraph 应用层["应用层 Application"]
        direction LR
        Command["command 包<br/>命令行参数"]
        Logger["lg 包<br/>日志系统"]
        Signal["signalx 包<br/>信号处理"]
        State["state 包<br/>状态管理"]
    end

    App --> Entry
    Entry -->|"类型别名<br/>函数转发"| Core
    Entry -->|"类型别名<br/>函数转发"| Provider
    Entry -->|"类型别名<br/>函数转发"| ErrUtil
    Entry -->|"类型别名<br/>函数转发"| Tool
    Entry -->|"类型别名<br/>函数转发"| Agent
    Entry -->|"类型别名<br/>函数转发"| Pattern
    Entry -->|"类型别名<br/>函数转发"| Router
    Entry -->|"类型别名<br/>函数转发"| Command

    Agent -->|"使用"| Tool
    Agent -->|"使用"| Provider
    Agent -->|"使用"| Core
    Agent -->|"使用"| State
    Agent -->|"使用"| Logger
    Agent -->|"使用"| ErrUtil

    Pattern -->|"使用"| Agent
    Pattern -->|"使用"| State
    Pattern -->|"使用"| Core

    Router -->|"使用"| Provider
    Router -->|"使用"| Core
    Router -->|"适配为"| Provider

    Tool -->|"使用"| Core

    Signal -->|"Context 传递"| Agent
    Signal -->|"Context 传递"| State

    Command -->|"配置"| Agent
    Command -->|"配置"| Router

    Logger -->|"记录"| Agent
    Logger -->|"记录"| State
    Logger -->|"记录"| Router
    Logger -->|"记录"| Tool

    State -->|"执行引擎"| Agent
    State -->|"执行引擎"| Pattern

    subgraph 数据流说明
        direction TB
        N1["Agent 使用 Tool 调用 LLM 工具"]
        N2["Provider 使用 Transport 发送请求"]
        N3["Router 使用 Provider 实现多模型切换"]
        N4["Pattern 使用 State 编排执行流程"]
        N5["Signal 传递 Context 实现优雅关闭"]
        N6["Logger 贯穿所有模块的日志记录"]
        N7["Command 解析参数配置运行时行为"]
    end

    Agent -.-> N1
    Provider -.-> N2
    Router -.-> N3
    Pattern -.-> N4
    Signal -.-> N5
    Logger -.-> N6
    Command -.-> N7
```

## 模块依赖关系图

```mermaid
graph LR
    subgraph 应用层
        Command[command]
        Logger[lg]
        Signal[signalx]
        State[state]
    end

    subgraph 核心层
        Tool[tool]
        Agent[agent]
        Pattern[pattern]
        Router[router]
    end

    subgraph 基础层
        Core[core]
        Provider[provider]
        ErrUtil[errutil]
    end

    Agent --> Tool
    Agent --> Provider
    Agent --> Core
    Agent --> State
    Agent --> ErrUtil

    Pattern --> Agent
    Pattern --> State
    Pattern --> Core

    Router --> Provider
    Router --> Core

    Tool --> Core

    Command --> Agent
    Command --> Router

    Signal --> Agent
    Signal --> State

    Logger --> Agent
    Logger --> State
    Logger --> Tool
    Logger --> Router

    State --> Core

    style Core fill:#e1f5fe,stroke:#01579b
    style Provider fill:#e1f5fe,stroke:#01579b
    style ErrUtil fill:#e1f5fe,stroke:#01579b
    style Tool fill:#fff3e0,stroke:#e65100
    style Agent fill:#fff3e0,stroke:#e65100
    style Pattern fill:#fff3e0,stroke:#e65100
    style Router fill:#fff3e0,stroke:#e65100
    style Command fill:#e8f5e9,stroke:#1b5e20
    style Logger fill:#e8f5e9,stroke:#1b5e20
    style Signal fill:#e8f5e9,stroke:#1b5e20
    style State fill:#e8f5e9,stroke:#1b5e20
```

## 三层架构说明

### 基础层（Foundation Layer）— KP01 ~ KP03

提供所有上层模块的基础类型和抽象，不依赖任何上层模块。

| 模块 | 知识点 | 核心职责 |
|---|---|---|
| `core` | KP01 | 定义 `LLMConfig`、`Message`、`ChatRequest/Response`、`ToolCall`、`StreamChunk` 等核心数据结构 |
| `provider` | KP02 | 定义 `Provider` 接口，实现 8 种 LLM 服务商的 API 调用 |
| `errutil` | KP03 | 定义 `AgentError`、错误分类、重试退避等错误处理基础设施 |

### 核心层（Core Layer）— KP04 ~ KP07

基于基础层构建，提供 Agent 运行时的核心能力。

| 模块 | 知识点 | 核心职责 |
|---|---|---|
| `tool` | KP04 | 定义 `Tool` 接口，提供工具注册表、工具调用范式检测 |
| `agent` | KP05 | Agent 主体，实现 Think-Act-Observe 循环、事件系统、会话存储 |
| `pattern` | KP06 | 5 大 Agent 范式：Chain/Evaluator/Parallel/Orchestrator/Router |
| `router` | KP07 | 多 Provider 智能路由，支持最便宜/最低延迟策略，环境变量配置 |

### 应用层（Application Layer）— KP08 ~ KP11

提供应用开发所需的辅助功能，可与核心层灵活组合。

| 模块 | 知识点 | 核心职责 |
|---|---|---|
| `command` | KP08 | 可扩展的命令行参数解析系统 |
| `lg` | KP09 | 结构化日志系统，支持模块化分流、多输出目标 |
| `signalx` | KP10 | 系统信号处理，优雅关闭机制 |
| `state` | KP11 | 通用状态机和工作流执行引擎 |

## 关键关系说明

| 关系 | 说明 |
|---|---|
| **Agent → Tool** | Agent 运行时通过 `ToolSet` 调用工具，实现 Act 阶段 |
| **Agent → Provider** | Agent 通过 `Provider` 与 LLM API 通信，实现 Think 阶段 |
| **Agent → State** | Agent 使用 `state.Workflow` 组织 Think-Act-Observe 循环 |
| **Pattern → State** | Pattern 使用 `state` 包的状态机实现各种 Agent 范式的编排 |
| **Router → Provider** | `RouterAdapter` 将 Router 适配为 Provider，实现多模型自动切换 |
| **Tool → Core** | 工具的 `ToolDef`、`ToolFunction` 等类型定义在 `core` 包 |
| **Signal → Agent/State** | `SignalContext` 传递的 Context 用于 Agent 和状态机的优雅中止 |
| **Logger → 全局** | `lg` 包贯穿所有模块，提供统一的日志记录能力 |
| **Command → Agent/Router** | 命令行参数用于配置 Agent 和 Router 的运行时行为 |
| **ErrUtil → Agent** | Agent 使用 `errutil` 进行错误分类和重试 |
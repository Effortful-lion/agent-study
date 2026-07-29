# KP10 — 信号处理（signalx 包）

## 标题和概述

`signalx` 包是 llmLib 的 **系统信号处理工具包**，提供了简洁的优雅关闭（Graceful Shutdown）机制。它负责：

- 将操作系统信号（`SIGINT`/Ctrl+C、`SIGTERM`）与 Go 的 `context.Context` 绑定
- 当进程接收到终止信号时，自动取消上下文，触发下游的清理和退出逻辑
- 基于 Go 标准库 `os/signal` 的高级封装，简化信号处理代码
- 为 Agent 运行时、HTTP 服务、工作流执行等场景提供统一的优雅关闭入口

## 核心概念

### 1. SignalContext — 信号上下文

```go
func SignalContext(parent context.Context) (context.Context, context.CancelFunc)
```

`SignalContext` 是包内唯一的函数，封装了 `signal.NotifyContext` 的调用：

- 监听两个信号：`os.Interrupt`（Ctrl+C）和 `syscall.SIGTERM`（`kill` 命令）
- 返回一个可被信号取消的 `context.Context` 和对应的 `context.CancelFunc`
- 当信号到达时，返回的 Context 会被自动取消，`ctx.Done()` channel 关闭
- 调用方也可以手动调用 `CancelFunc` 主动取消

### 2. 信号处理原理

```
进程运行 → 收到 SIGINT/SIGTERM → signal.NotifyContext 捕获 → Context 取消 → 下游 ctx.Done() 触发 → 优雅关闭
```

Go 标准库 `signal.NotifyContext` 的工作流程：
1. 注册对指定信号的监听
2. 信号到达时调用 `cancel()` 取消 Context
3. 所有持有该 Context 的 goroutine 通过 `ctx.Done()` 感知到取消
4. 执行清理逻辑（关闭数据库连接、保存状态、刷新日志等）

### 3. 与 Agent 运行时的集成

在 Agent 运行时中，`SignalContext` 用于实现：
- 用户按 Ctrl+C 时中止当前的 Think-Act-Observe 循环
- 正在进行的 LLM 调用、工具执行被中断
- 已完成的步骤和中间结果被保留
- 日志系统记录 shutdown 事件

## 类型/函数清单

### signalx 包导出
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `SignalContext(parent)` | `signalx/signal.go` | 创建信号绑定的 Context，监听 SIGINT 和 SIGTERM |

### 底层依赖（Go 标准库）
| 类型/函数 | 来源 | 说明 |
|---|---|---|
| `signal.NotifyContext(parent, signals...)` | `os/signal` | 信号通知上下文（底层实现） |
| `os.Interrupt` | `os` | Ctrl+C 信号（跨平台） |
| `syscall.SIGTERM` | `syscall` | `kill` 命令发送的终止信号 |
| `context.Context` | `context` | Go 标准库上下文接口 |
| `context.CancelFunc` | `context` | 取消函数类型 |

## 使用示例

### 基础用法

```go
import (
    "context"
    "fmt"
    "time"
    "github.com/Effortful-lion/agent-study/llmLib/signalx"
)

func main() {
    ctx, cancel := signalx.SignalContext(context.Background())
    defer cancel()

    fmt.Println("服务启动，按 Ctrl+C 退出...")

    // 主循环：监听信号
    for {
        select {
        case <-ctx.Done():
            fmt.Println("收到终止信号，正在优雅关闭...")
            // 执行清理逻辑
            cleanup()
            return
        case <-time.After(time.Second):
            fmt.Println("运行中...")
        }
    }
}

func cleanup() {
    fmt.Println("关闭数据库连接...")
    fmt.Println("保存状态...")
    fmt.Println("刷新日志...")
}
```

### 与 Agent 集成

```go
import (
    "context"
    "fmt"
    "github.com/Effortful-lion/agent-study/llmLib/agent"
    "github.com/Effortful-lion/agent-study/llmLib/provider"
    "github.com/Effortful-lion/agent-study/llmLib/signalx"
    "github.com/Effortful-lion/agent-study/llmLib/tool"
)

func main() {
    ctx, cancel := signalx.SignalContext(context.Background())
    defer cancel()

    p, _ := provider.NewProvider("doubao")
    registry := tool.NewRegistryToolSet()

    a := agent.New(p, "doubao-pro-32k", registry)

    // Agent.Run 内部会检查 ctx.Done()
    // 收到信号时自动中止运行
    events, err := a.Run(ctx, "帮我分析最近的销售数据")
    if err != nil {
        fmt.Printf("运行结束: %v\n", err)
    }

    for event := range events {
        fmt.Print(event.Text)
    }
}
```

### 与 HTTP 服务集成

```go
import (
    "context"
    "fmt"
    "net/http"
    "time"
    "github.com/Effortful-lion/agent-study/llmLib/signalx"
)

func main() {
    ctx, cancel := signalx.SignalContext(context.Background())
    defer cancel()

    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })

    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    // 在 goroutine 中启动 HTTP 服务
    go func() {
        fmt.Println("HTTP 服务启动在 :8080")
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            fmt.Printf("服务异常退出: %v\n", err)
        }
    }()

    // 等待信号
    <-ctx.Done()
    fmt.Println("正在关闭 HTTP 服务...")

    // 优雅关闭 HTTP 服务（5 秒超时）
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        fmt.Printf("服务关闭超时: %v\n", err)
    } else {
        fmt.Println("HTTP 服务已优雅关闭")
    }
}
```

### 与工作流/状态机集成

```go
import (
    "context"
    "fmt"
    "github.com/Effortful-lion/agent-study/llmLib/signalx"
    "github.com/Effortful-lion/agent-study/llmLib/state"
)

func main() {
    ctx, cancel := signalx.SignalContext(context.Background())
    defer cancel()

    step1 := state.NewStep("获取数据", func(ctx context.Context, s map[string]any) (map[string]any, error) {
        // 检查是否被取消
        select {
        case <-ctx.Done():
            return s, ctx.Err()
        default:
        }
        // 执行数据获取
        return s, nil
    }, state.WithTimeout(30*time.Second))

    step2 := state.NewStep("处理数据", processFn)
    step3 := state.NewStep("保存结果", saveFn)

    workflow := state.Do(step1, step2, step3)

    // 执行工作流，信号到达时自动中止
    finalState, err := workflow.Run(ctx, nil)
    if err != nil {
        fmt.Printf("工作流结束: %v\n", err)
    }
    fmt.Printf("最终状态: %v\n", finalState)
}
```

### 多级信号处理

```go
func main() {
    // 父 Context 先超时，信号取消作为后备
    parentCtx, parentCancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer parentCancel()

    ctx, cancel := signalx.SignalContext(parentCtx)
    defer cancel()

    // ctx 会在以下任一情况被取消：
    // 1. 收到 SIGINT/SIGTERM
    // 2. 父 Context 超时（10 分钟）
    // 3. 手动调用 cancel()
    <-ctx.Done()
}
```

## 关联知识点

- **KP05-Agent运行时**：`Agent.Run()` 接收 `context.Context`，`SignalContext` 提供的 Context 可直接用于 Agent 运行时的优雅中止
- **KP11-状态管理**：状态机 `Machine.Run()` 和工作流 `Workflow.Run()` 均接收 `context.Context`，信号取消可触发执行流程的中断
- **KP09-日志系统**：信号触发优雅关闭时，可通过 `lg` 包记录 shutdown 事件和错误信息
- **KP08-命令行参数**：可通过命令行参数配置超时时间，结合 `SignalContext` 实现超时 + 信号的双重退出机制
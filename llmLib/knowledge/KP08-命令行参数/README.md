# KP08 — 命令行参数（command 包）

## 标题和概述

`command` 包是 llmLib 的 **可扩展命令行参数系统**，基于 Go 标准库 `flag` 包进行封装，提供了插件式的参数注册和解析能力。它负责：

- `CommandBuilder` 封装命令行参数的注册、解析和查询
- 支持参数的重复注册检测（同名参数返回错误）
- 提供 `MustRegister` 便捷方法用于初始化阶段快速挂载
- `LoadCommands` 预注册常用参数（`question` 提问内容）
- 返回未被消费的位置参数，支持子命令和多级命令行结构

## 核心概念

### 1. CommandBuilder 结构体

```go
type CommandBuilder struct {
    fs     *flag.FlagSet           // 标准库 FlagSet 实例
    values map[string]*string      // 参数名 → 解析后值的指针映射
}
```

`CommandBuilder` 是整个包的核心，封装了 `flag.FlagSet` 并增加了可扩展性。

### 2. NewCommandBuilder — 创建构建器

```go
func NewCommandBuilder() *CommandBuilder
```

创建一个空的命令行参数构建器：
- 初始化 `flag.FlagSet`（使用 `ContinueOnError` 模式，解析错误时不 panic）
- 初始化空的 `values` 映射
- FlagSet 名称为 `"llmlib"`

### 3. Register — 注册参数

```go
func (b *CommandBuilder) Register(name, usage, defaultValue string) error
```

注册一个 `string` 类型的命令行参数：
- `name`：参数名（对应命令行 `-name` 标志）
- `usage`：参数用法说明
- `defaultValue`：默认值
- 同名参数重复注册时返回错误 `"command flag already registered: {name}"`

### 4. Parse — 解析参数

```go
func (b *CommandBuilder) Parse(args []string) ([]string, error)
```

解析传入的参数列表（通常为 `os.Args[1:]`）：
- 返回未被 flag 消费的剩余位置参数（可用于子命令）
- 解析失败时返回错误

### 5. Get — 获取参数值

```go
func (b *CommandBuilder) Get(name string) string
```

获取指定参数解析后的值：
- 参数不存在时返回空字符串
- 参数存在但值为 nil 时返回空字符串

### 6. LoadCommands — 预注册常用命令

```go
func LoadCommands() *CommandBuilder
```

创建一个已预注册 `question` 参数的构建器：
- 参数名：`question`
- 用法：`提问内容`
- 默认值：空字符串

### 7. MustRegister — 强制注册

```go
func (b *CommandBuilder) MustRegister(name, usage, defaultValue string)
```

`Register` 的 panic 版本，注册失败时直接 panic：
- 适合初始化阶段快速挂载常用参数
- 避免错误处理代码的冗杂

## 类型/函数清单

### CommandBuilder 类型与方法
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `CommandBuilder` | `command/command.go` | 命令行参数构建器结构体 |
| `NewCommandBuilder()` | `command/command.go` | 创建空构建器 |
| `(*CommandBuilder).Register(name, usage, defaultValue)` | `command/command.go` | 注册 string 类型参数 |
| `(*CommandBuilder).Parse(args)` | `command/command.go` | 解析参数，返回剩余位置参数 |
| `(*CommandBuilder).Get(name)` | `command/command.go` | 获取参数值 |
| `(*CommandBuilder).MustRegister(name, usage, defaultValue)` | `command/command.go` | 强制注册（panic 版本） |

### 预配置
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `LoadCommands()` | `command/command.go` | 创建预注册 `question` 的构建器 |

### 底层依赖
| 类型 | 源文件 | 说明 |
|---|---|---|
| `flag.FlagSet` | Go 标准库 `flag` | 底层 Flag 解析引擎 |

## 使用示例

### 基础使用

```go
import (
    "fmt"
    "os"
    "github.com/Effortful-lion/agent-study/llmLib/command"
)

// 1. 创建构建器并注册参数
builder := command.NewCommandBuilder()
builder.Register("question", "提问内容", "")
builder.Register("model", "使用的模型", "doubao-pro-32k")
builder.Register("api-key", "API 密钥", "")

// 2. 解析命令行参数
args := os.Args[1:]
remaining, err := builder.Parse(args)
if err != nil {
    fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
    os.Exit(1)
}

// 3. 获取参数值
question := builder.Get("question")
model := builder.Get("model")
apiKey := builder.Get("api-key")

fmt.Printf("问题: %s\n", question)
fmt.Printf("模型: %s\n", model)
fmt.Printf("API Key: %s\n", apiKey)
fmt.Printf("剩余参数: %v\n", remaining)
```

### 命令行调用示例

```bash
# 基本用法
./app -question "你好" -model doubao-pro-32k

# 使用默认值
./app -question "计算 1+1"

# 带位置参数（未被消费的参数会返回）
./app -question "你好" subcommand arg1 arg2
# remaining = ["subcommand", "arg1", "arg2"]
```

### 使用 LoadCommands 快速启动

```go
// 使用预配置的构建器
builder := command.LoadCommands()

// 追加自定义参数
builder.MustRegister("model", "模型名称", "doubao-pro-32k")
builder.MustRegister("verbose", "详细输出", "false")
builder.MustRegister("session-id", "会话 ID", "")

builder.Parse(os.Args[1:])

question := builder.Get("question")
model := builder.Get("model")
```

### 子命令支持

```go
builder := command.NewCommandBuilder()
builder.Register("verbose", "详细模式", "false")

remaining, _ := builder.Parse(os.Args[1:])

// 解析子命令
if len(remaining) > 0 {
    subcommand := remaining[0]
    switch subcommand {
    case "chat":
        runChat(builder, remaining[1:])
    case "config":
        runConfig(builder, remaining[1:])
    default:
        fmt.Printf("未知子命令: %s\n", subcommand)
    }
}

func runChat(builder *command.CommandBuilder, args []string) {
    // 可为子命令注册额外参数
    builder.MustRegister("session", "会话 ID", "")
    builder.Parse(args)
}
```

### 重复注册检测

```go
builder := command.NewCommandBuilder()
err := builder.Register("model", "模型", "default")
// err == nil

err = builder.Register("model", "模型", "another")
// err != nil: "command flag already registered: model"
```

### 与 Agent 集成

```go
import (
    "context"
    "github.com/Effortful-lion/agent-study/llmLib/agent"
    "github.com/Effortful-lion/agent-study/llmLib/command"
    "github.com/Effortful-lion/agent-study/llmLib/provider"
    "github.com/Effortful-lion/agent-study/llmLib/tool"
)

func main() {
    // 1. 解析命令行
    builder := command.LoadCommands()
    builder.MustRegister("api-key", "API 密钥", "")
    builder.MustRegister("model", "模型", "doubao-pro-32k")
    builder.MustRegister("base-url", "API 基础 URL", "")
    builder.MustRegister("session-id", "会话 ID", "")
    builder.MustRegister("budget-steps", "最大步数", "10")

    builder.Parse(os.Args[1:])

    // 2. 获取参数
    question := builder.Get("question")
    apiKey := builder.Get("api-key")
    model := builder.Get("model")
    baseURL := builder.Get("base-url")
    sessionID := builder.Get("session-id")

    // 3. 初始化 Agent
    p, _ := provider.NewProvider("doubao")
    registry := tool.NewRegistryToolSet()
    registry.Register(&tool.CalculatorTool{})
    registry.Register(&tool.TimeTool{})

    opts := []agent.Option{
        agent.WithAgentAPIKey(apiKey),
    }
    if baseURL != "" {
        opts = append(opts, agent.WithAgentBaseURL(baseURL))
    }
    if sessionID != "" {
        store := agent.NewFileStore("./sessions")
        opts = append(opts, agent.WithStore(store, sessionID))
    }

    a := agent.New(p, model, registry, opts...)

    // 4. 运行 Agent
    ctx := context.Background()
    events, _ := a.Run(ctx, question)
    for event := range events {
        fmt.Println(event.Text)
    }
}
```

## 关联知识点

- **KP05-Agent运行时**：命令行参数常用于配置 `Agent` 的 `apiKey`、`baseURL`、`model`、`sessionID`、`budget` 等运行时选项
- **KP07-多Provider路由**：可通过 `command` 注册 `LLM_ROUTER_STRATEGY` 等参数，配合 `router.ReadStrategyFromEnv()` 或自定义逻辑实现命令行级别的路由策略选择
- **KP02-Provider体系**：命令行参数可指定 Provider 名称和模型，用于动态选择 `provider.NewProvider()` 的参数
- **Go 标准库 flag**：`CommandBuilder` 封装了 `flag.FlagSet`，增加了重复注册检测和链式构建能力
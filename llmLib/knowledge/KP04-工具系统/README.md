# KP04 — 工具系统（tool 包）

## 标题和概述

`tool` 包是 llmLib 的**Agent 工具体系**，为 Agent 提供感知（读取外部信息）和行动（执行操作）的能力。它实现了：

- `Tool` 接口定义，标准化所有工具的行为契约
- `Registry` 工具注册表，统一管理工具的注册、查找和调用
- 两种工具调用范式的自动检测与解析（Function Calling / ReAct）
- JSON Schema 的自动生成与转换，使工具参数能被 LLM 正确理解
- `JSONSchemaTool` 的标准实现，简化新工具的创建流程

## 核心概念

### 1. Tool 接口

```go
type Tool interface {
    Name() string                                               // 工具名称
    Description() string                                        // 工具描述
    Parameters() map[string]string                              // 参数描述（旧格式，向后兼容）
    Call(ctx context.Context, args map[string]any) (any, error) // 执行工具
}
```

这是所有工具的最小契约。模型通过 `Name` 识别工具、通过 `Description` 理解工具用途、通过 `Parameters` 了解参数结构。`Call` 接收已解析的 `map[string]any` 参数，返回任意结果。

### 2. SchemaTool 接口

```go
type SchemaTool interface {
    Tool
    ParametersSchema() json.RawMessage  // 返回 JSON Schema 格式的参数定义
}
```

扩展 `Tool` 接口，提供原生 JSON Schema 支持。`BuildToolDefs` 在构建工具定义时会自动检测此接口，优先使用 `ParametersSchema()`。

### 3. Registry — 工具注册表

`Registry` 是工具的容器，提供：
- `Register(t Tool)`：注册工具（同名覆盖）
- `Get(name)`：按名称查找工具
- `ToolDefs()`：构建 `[]core.ToolDef` 供模型调用
- `Call(ctx, name, args)`：按名称调用工具，不存在时返回 `ErrCategoryToolNotFound`

### 4. Tool Calling Paradigm — 工具调用范式

`ToolCallingParadigm` 接口定义两种解析策略：

**FunctionCallingParadigm**（结构化 JSON）
- 检测：响应包含 `"tool_calls"` 或 `"function"` 关键字
- 解析：从 JSON 中提取 `tool_calls[].name` 和 `tool_calls[].arguments`
- 适用：支持原生工具调用的模型（GPT-4、Claude 3.5+ 等）

**ReActParadigm**（文本格式）
- 检测：响应包含 `Action:`、`<function name=` 或 `[function`
- 解析：支持两种文本格式
  - `Action: tool_name\nAction Input: {"key":"value"}`
  - `<function name="tool_name">{"key":"value"}</function>`
- 适用：不支持原生工具调用的模型

`DetectParadigm(content)` 自动检测使用哪种范式，优先 Function Calling。

### 5. JSON Schema 生成

`Generate(v any)` 通过反射自动为任意类型生成 JSON Schema：
- 解引用指针
- 支持 string、bool、int*/uint*（integer）、float*（number）、struct（object）、slice/array（array）、map
- Struct 字段通过 `json` tag 控制名称和必填（`omitempty` → 非必填）
- 字段 `desc` tag 作为 JSON Schema 的 `description`
- 循环引用自动截断
- Struct 默认设置 `additionalProperties: false`（LLM 场景关键，防止模型产生未定义字段）

### 6. JSONSchemaTool — 标准实现

`JSONSchemaTool` 是推荐的工具实现方式：
- 构造时直接传入 JSON Schema 格式的 `parametersJSON`
- 实现了 `SchemaTool` 接口，原生支持 JSON Schema
- 通过 `NewJSONSchemaTool(name, desc, schema, callFn)` 一行创建

### 7. 参数转换

- `BuildArgs(argsJSON)`：将 JSON 字符串参数解析为 `map[string]any`，支持双重序列化场景
- `StructToMap(v)`：通过反射将结构体转为 `map[string]any`
- `mapToJSONSchema(params)`：将旧格式 `map[string]string` 参数自动转为 JSON Schema

## 类型/函数清单

### 接口
| 接口 | 源文件 | 说明 |
|---|---|---|
| `Tool` | `tool/tool.go` | 基础工具接口 |
| `SchemaTool` | `tool/tool_examples.go` | JSON Schema 扩展接口 |
| `ToolCallingParadigm` | `tool/tool_calling.go` | 工具调用范式接口 |

### 核心类型
| 类型 | 源文件 | 说明 |
|---|---|---|
| `Registry` | `tool/tool.go` | 工具注册表（`Tools map[string]Tool`） |
| `JSONSchemaTool` | `tool/tool_examples.go` | 标准 JSON Schema 工具实现 |
| `Schema` | `tool/json_schema.go` | JSON Schema 结构定义 |
| `FunctionCallingParadigm` | `tool/tool_calling.go` | Function Calling 范式解析器 |
| `ReActParadigm` | `tool/tool_calling.go` | ReAct 范式解析器 |

### 注册表方法
| 方法 | 源文件 | 说明 |
|---|---|---|
| `NewRegistryToolSet()` | `tool/tool.go` | 创建空注册表 |
| `Register(t)` | `tool/tool.go` | 注册工具 |
| `Get(name)` | `tool/tool.go` | 查找工具 |
| `ToolDefs()` | `tool/tool.go` | 构建 `[]core.ToolDef` |
| `Call(ctx, name, args)` | `tool/tool.go` | 调用工具 |

### 工具构建与转换
| 函数 | 源文件 | 说明 |
|---|---|---|
| `NewJSONSchemaTool(name, desc, schema, fn)` | `tool/tool_examples.go` | 创建 JSONSchemaTool |
| `BuildToolDefs(registry)` | `tool/tool_examples.go` | 从注册表构建 ToolDef 列表 |
| `mapToJSONSchema(params)` | `tool/tool_examples.go` | map 格式转 JSON Schema |
| `BuildArgs(argsJSON)` | `tool/tool.go` | JSON 字符串转 `map[string]any` |
| `StructToMap(v)` | `tool/tool.go` | 结构体转 `map[string]any` |

### JSON Schema 生成
| 函数 | 源文件 | 说明 |
|---|---|---|
| `Generate(v)` | `tool/json_schema.go` | 反射生成 JSON Schema |
| `generateType(typ, visited)` | `tool/json_schema.go` | 递归核心（未导出） |

### 范式检测与解析
| 函数 | 源文件 | 说明 |
|---|---|---|
| `DetectParadigm(content)` | `tool/tool_calling.go` | 自动检测工具调用范式 |
| `FunctionCallingParadigm.Detect(content)` | `tool/tool_calling.go` | 检测 Function Calling 格式 |
| `FunctionCallingParadigm.Parse(content)` | `tool/tool_calling.go` | 解析 Function Calling |
| `ReActParadigm.Detect(content)` | `tool/tool_calling.go` | 检测 ReAct 格式 |
| `ReActParadigm.Parse(content)` | `tool/tool_calling.go` | 解析 ReAct |

### 示例工具
| 类型 | 源文件 | 说明 |
|---|---|---|
| `CalculatorTool` | `tool/tool_examples.go` | 计算器工具（支持加减乘除和括号） |
| `TimeTool` | `tool/tool_examples.go` | 当前时间获取工具 |

## 使用示例

```go
import (
    "context"
    "encoding/json"
    "github.com/Effortful-lion/agent-study/llmLib/core"
    "github.com/Effortful-lion/agent-study/llmLib/tool"
)

// 1. 实现 Tool 接口（旧方式）
type WeatherTool struct{}

func (t *WeatherTool) Name() string        { return "get_weather" }
func (t *WeatherTool) Description() string { return "获取指定城市的天气" }
func (t *WeatherTool) Parameters() map[string]string {
    return map[string]string{
        "city": "string, 城市名称",
    }
}
func (t *WeatherTool) Call(ctx context.Context, args map[string]any) (any, error) {
    city, _ := args["city"].(string)
    return fmt.Sprintf("%s: 25°C, 晴", city), nil
}

// 2. 使用 JSONSchemaTool（推荐方式）
weatherTool := tool.NewJSONSchemaTool(
    "get_weather",
    "获取指定城市的天气",
    json.RawMessage(`{
        "type": "object",
        "properties": {
            "city": {"type": "string", "description": "城市名称"}
        },
        "required": ["city"]
    }`),
    func(ctx context.Context, args map[string]any) (any, error) {
        city, _ := args["city"].(string)
        return fmt.Sprintf("%s: 25°C, 晴", city), nil
    },
)

// 3. 使用 Generate 自动生成 JSON Schema
type SearchParams struct {
    Query string `json:"query" desc:"搜索关键词"`
    Limit int    `json:"limit" desc:"返回数量"`
}
schema := tool.Generate(SearchParams{})
schemaJSON, _ := json.Marshal(schema)

// 4. 注册和使用工具
registry := tool.NewRegistryToolSet()
registry.Register(weatherTool)
registry.Register(&WeatherTool{})
registry.Register(&tool.CalculatorTool{})
registry.Register(&tool.TimeTool{})

// 5. 构建 ToolDef 传给模型
toolDefs := registry.ToolDefs()
// toolDefs 可直接传入 core.ChatRequest.Tools

// 6. 调用工具
result, err := registry.Call(ctx, "get_weather", map[string]any{"city": "北京"})

// 7. 参数解析
args, err := tool.BuildArgs(`{"city": "北京"}`)

// 8. 范式检测（处理 ReAct 模型的文本输出）
modelOutput := `Action: get_weather
Action Input: {"city": "北京"}`
paradigm := tool.DetectParadigm(modelOutput)
if paradigm != nil {
    calls, err := paradigm.Parse(modelOutput)
    // calls[0].Name = "get_weather", calls[0].Args = {"city": "北京"}
}

// 9. StructToMap 结构体转参数
type CallArgs struct {
    City  string
    Unit  string
}
argsMap := tool.StructToMap(CallArgs{City: "上海", Unit: "metric"})
```

## 关联知识点

- **KP01-基础类型**：`BuildToolDefs` 产出 `[]core.ToolDef`，`ToolCall` 结构体在工具调用流程中被创建和传递
- **KP02-Provider体系**：`ToolCallProvider.ChatWithTools` 接收 `[]core.ToolDef`，模型返回的 `ToolCalls` 通过 `DetectParadigm` 解析
- **KP03-错误处理**：`Registry.Call` 内部使用 `errutil.NewAgentError` 创建工具相关错误（`ErrCategoryToolNotFound`、`ErrCategoryTool`）
- **Agent 模块**：Agent 主循环使用 `Registry` 管理工具，通过 `ToolCallingParadigm` 解析模型输出中的工具调用指令
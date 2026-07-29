## 二、边界映射实现总结

### 新增文件

| 文件 | 职责 |
|------|------|
| [provider/anthropic_adapter.go](file:///Users/lion/mycode/agent-study/llmLib/provider/anthropic_adapter.go) | Anthropic 原生格式的类型定义 + 双向映射函数 |
| [provider/openai_adapter.go](file:///Users/lion/mycode/agent-study/llmLib/provider/openai_adapter.go) | OpenAI 格式的类型定义 + 双向映射函数（对称设计） |

### 修改文件

| 文件 | 改动 |
|------|------|
| [provider/chat_claude.go](file:///Users/lion/mycode/agent-study/llmLib/provider/chat_claude.go) | 使用 `toAnthropicMessages()` + `toAnthropicTools()` + `parseAnthropicResponse()` 替代内联转换 |
| [provider/chat_openai.go](file:///Users/lion/mycode/agent-study/llmLib/provider/chat_openai.go) | 使用 `toOpenAIRequest()` + `parseOpenAIResponse()` 替代内联转换 |

### 核心映射规则

```
统一格式                          →  OpenAI 格式              →  Anthropic 格式
─────────────────────────────────────────────────────────────────────────────
System 消息                       →  system 消息（直通）       →  顶层 system 字段
User 消息                         →  user 消息（直通）         →  user 消息
Assistant（纯文本）               →  assistant 消息（直通）    →  assistant 消息
Assistant + ToolCalls             →  tool_calls 数组           →  tool_use 内容块
ToolRole（结果回填）              →  tool 消息 + tool_call_id  →  user 消息 + tool_result 内容块
ToolDef {type, function{...}}     →  {type, function{...}}     →  {name, description, input_schema}
```

### 数据流架构

```
Agent (统一格式)
  │
  ├─ stepThink(): callModel() → provider.ChatWithTools(messages, tools)
  │     │
  │     ├─ [OpenAI]  toOpenAIRequest() → HTTP → parseOpenAIResponse()
  │     └─ [Claude]  toAnthropicMessages() + toAnthropicTools() → HTTP → parseAnthropicResponse()
  │
  └─ stepAct(): 执行工具 → 结果回填到 state.Messages（统一格式）
        │
        └─ 下一轮 stepThink() 时，统一格式再由 provider 边界映射到各自协议
```
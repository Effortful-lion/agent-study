// 文件职责：
// - 定义 Agent 运行过程中向外发出的事件类型和事件结构。
// - 事件分层设计，让调用方能清楚看到调用层次：
//   Step 层：StepStart / StepEnd — 标记哪个阶段开始/结束
//   Model 层：ModelCall — 标记调用了哪个模型
//   Tool 层：ToolCall / ToolResult — 标记调用了哪个工具及结果
//   Result 层：Thought / Answer — 思考内容和最终答案
//   Control 层：Error / Done — 错误和完成信号

package llmlib

// EventType 表示 Agent 发出的事件类型。
type EventType string

const (
	// --- Step 层：阶段事件 ---
	EventStepStart EventType = "step_start" // 某个阶段开始（thinking / acting）
	EventStepEnd   EventType = "step_end"   // 某个阶段结束

	// --- Model 层：模型调用事件 ---
	EventModelCall     EventType = "model_call"     // 正在调用 ChatModel
	EventModelResponse EventType = "model_response" // 模型返回的原始内容（调试用）

	// --- Tool 层：工具调用事件 ---
	EventToolCall   EventType = "tool_call"   // 即将调用某工具
	EventToolResult EventType = "tool_result" // 工具返回了结果

	// --- Result 层：思考与答案 ---
	EventThought EventType = "thought" // 模型在调用工具前的推理过程
	EventAnswer  EventType = "answer"  // 模型给出的最终答案（完整内容）

	// --- Control 层：控制事件 ---
	EventError EventType = "error" // 执行过程中发生错误
	EventDone  EventType = "done"  // Agent 执行完成
)

// AgentEvent 是 Agent 运行过程中向外发出的一个事件。
//
// 事件流示例：
//
//	[step_start] thinking
//	[model_call] 调用模型 doubao-1.5-pro-256k
//	[thought] 我需要先计算 3+5*2...
//	[tool_call] calculator({"expression":"3+5*2"})
//	[tool_result] 计算结果: 13
//	[step_end] thinking → acting
//	[step_start] thinking
//	[model_call] 调用模型 doubao-1.5-pro-256k
//	[tool_call] get_current_time({})
//	[tool_result] 2026-07-25T10:30:00+08:00
//	[step_end] thinking → acting
//	[step_start] thinking
//	[model_call] 调用模型 doubao-1.5-pro-256k
//	[answer] 3+5*2=13，当前时间是 2026年7月25日 10:30
//	[step_end] thinking → done
//	[done]
type AgentEvent struct {
	Type EventType `json:"type"`
	Text string    `json:"text,omitempty"` // 思考内容 / 答案 / 错误信息 / 阶段描述
	Tool string    `json:"tool,omitempty"` // 涉及的工具名
	Args string    `json:"args,omitempty"` // 工具参数
	Step int       `json:"step,omitempty"` // 当前 Agent 步数
}

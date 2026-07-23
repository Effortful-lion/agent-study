// 文件职责：
// - 定义 Agent 运行过程中向外发出的事件类型和事件结构。
// - 通过 channel 把 Agent 内部过程抽象成 AgentEvent，实时推给终端、WebSocket 或 SSE。
// - 事件类型包括思考、工具调用、工具结果、答案增量、错误和完成。

package llmlib

// EventType 表示 Agent 发出的事件类型。
type EventType string

const (
	EventThought     EventType = "thought"      // 模型的一段思考，在调用工具前的推理过程
	EventToolCall    EventType = "tool_call"    // 即将调用某工具，包含工具名和参数
	EventToolResult  EventType = "tool_result"  // 工具返回了结果，包含工具名和返回值
	EventAnswerDelta EventType = "answer_delta" // 最终答案的一个增量（流式）
	EventError       EventType = "error"        // 执行过程中发生错误
	EventDone        EventType = "done"         // Agent 执行完成，正常结束或被停止条件终止
)

// AgentEvent 是 Agent 运行过程中向外发出的一个事件，用于展示执行过程和结果。
// 调用方可以通过监听事件流来实现实时状态展示、调试和日志记录。
type AgentEvent struct {
	Type EventType `json:"type"`
	Text string    `json:"text,omitempty"` // 思考内容 / 答案增量 / 错误信息
	Tool string    `json:"tool,omitempty"` // 涉及的工具名
	Args string    `json:"args,omitempty"` // 工具参数
	Step int       `json:"step,omitempty"` // 当前 Agent 步数
}
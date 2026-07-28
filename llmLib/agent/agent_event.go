// 文件职责：
// - 定义 Agent 运行过程中向外发出的事件类型和事件结构。

package agent

// EventType 表示 Agent 发出的事件类型。
type EventType string

const (
	EventStepStart EventType = "step_start"
	EventStepEnd   EventType = "step_end"
	EventModelCall     EventType = "model_call"
	EventModelResponse EventType = "model_response"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventThought EventType = "thought"
	EventAnswer  EventType = "answer"
	EventError EventType = "error"
	EventDone  EventType = "done"
)

// AgentEvent 是 Agent 运行过程中向外发出的一个事件。
type AgentEvent struct {
	Type EventType `json:"type"`
	Text string    `json:"text,omitempty"`
	Tool string    `json:"tool,omitempty"`
	Args string    `json:"args,omitempty"`
	Step int       `json:"step,omitempty"`
}

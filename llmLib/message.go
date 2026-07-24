package llmlib

import "encoding/json"

type ToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"arguments"`
	Result string          `json:"result,omitempty"`
}

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// openaiToolCall 用于 OpenAI 兼容的 tool_calls 序列化格式。
// 注意：function.arguments 必须是 JSON 字符串（而非 JSON 对象）。
type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// MarshalJSON 自定义序列化，处理不同角色的消息格式。
func (m Message) MarshalJSON() ([]byte, error) {
	switch m.Role {
	case Assistant:
		if len(m.ToolCalls) > 0 {
			openaiTCs := make([]openaiToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				openaiTCs[i] = openaiToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				openaiTCs[i].Function.Name = tc.Name
				// 将 json.RawMessage 转为字符串，满足 OpenAI 协议要求
				openaiTCs[i].Function.Arguments = string(tc.Args)
			}
			return json.Marshal(struct {
				Role      Role             `json:"role"`
				Content   string           `json:"content,omitempty"`
				ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
			}{
				Role:      m.Role,
				Content:   m.Content,
				ToolCalls: openaiTCs,
			})
		}
	case ToolRole:
		return json.Marshal(struct {
			Role       Role   `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id"`
		}{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		})
	}

	type alias Message
	return json.Marshal(alias(m))
}

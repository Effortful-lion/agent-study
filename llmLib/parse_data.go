// 文件职责：
// - 解析 OpenAI 兼容流协议中的单条 data 事件。
// - 供 SSE 消费逻辑提取增量文本和结束标记。

package llmlib

import (
	"encoding/json"
	"fmt"
)

// parseOpenAIDelta 从 OpenAI 风格的 SSE data 负载中提取文本增量和结束状态。
func parseOpenAIDelta(data []byte) (delta string, done bool, err error) {
	delta, done, _, err = parseOpenAIDeltaWithTools(data)
	return
}

// parseOpenAIDeltaWithTools 从 OpenAI 风格的 SSE data 负载中提取文本增量、结束状态和工具调用。
func parseOpenAIDeltaWithTools(data []byte) (delta string, done bool, toolCalls []ToolCall, err error) {
	if string(data) == "[DONE]" {
		return "", true, nil, nil
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return "", false, nil, fmt.Errorf("解析 OpenAI 流事件: %w", err)
	}
	if len(chunk.Choices) == 0 {
		return "", false, nil, nil
	}
	for _, tc := range chunk.Choices[0].Delta.ToolCalls {
		args := normalizeArgs(tc.Function.Arguments)
		toolCalls = append(toolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}
	return chunk.Choices[0].Delta.Content, false, toolCalls, nil
}

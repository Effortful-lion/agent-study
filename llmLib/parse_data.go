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
	if string(data) == "[DONE]" {
		return "", true, nil
	}
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return "", false, fmt.Errorf("解析 OpenAI 流事件: %w", err)
	}
	// 无 choice 时视为可忽略空事件，继续等待后续文本片段。
	if len(chunk.Choices) == 0 {
		return "", false, nil
	}
	return chunk.Choices[0].Delta.Content, false, nil
}

package llmlib

import (
	"encoding/json"
	"fmt"
)

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
	if len(chunk.Choices) == 0 {
		return "", false, nil
	}
	return chunk.Choices[0].Delta.Content, false, nil
}

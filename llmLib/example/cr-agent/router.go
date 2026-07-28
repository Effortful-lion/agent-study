package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

type intentResult struct {
	IsCode bool   `json:"is_code"`
	Reason string `json:"reason"`
}

func detectIntent(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, input string) (intentResult, error) {
	if looksLikeCode(input) {
		return intentResult{IsCode: true, Reason: "contains Go code patterns"}, nil
	}

	system := `判断以下输入是否为 Go 代码片段。
输出格式：只输出 JSON {"is_code":true/false,"reason":"说明"}
- is_code: true 表示是 Go 代码，false 表示不是
- reason: 简要说明判断依据`

	messages := []llmlib.Message{
		llmlib.NewSystemMessage(system),
		llmlib.NewUserMessage(input),
	}

	out, err := p.Chat(ctx, cfg, messages)
	if err != nil {
		return intentResult{}, err
	}

	return parseIntentResult(out.Content)
}

func looksLikeCode(input string) bool {
	patterns := []string{
		"func ",
		"package ",
		"import ",
		"type ",
		"var ",
		"if ",
		"for ",
		"go func",
		"channel",
		"goroutine",
		"defer ",
		"panic(",
		"recover(",
		"context.Context",
		"error",
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range patterns {
		if strings.Contains(inputLower, pattern) {
			return true
		}
	}
	return false
}

func parseIntentResult(content string) (intentResult, error) {
	var result intentResult
	if err := json.Unmarshal([]byte(extractJSON(content)), &result); err != nil {
		return intentResult{}, fmt.Errorf("parse intent: %w\nraw: %s", err, content)
	}
	return result, nil
}
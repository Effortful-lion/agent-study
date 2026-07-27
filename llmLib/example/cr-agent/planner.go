package main

import (
	"context"
	"encoding/json"
	"fmt"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

type reviewPlan struct {
	Dimensions []string `json:"dimensions"`
}

func planReview(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, code string) (reviewPlan, error) {
	system := `你是资深 Go 代码审查专家。分析以下 Go 代码，列出最值得审查的 3-5 个维度。
只输出 JSON: {"dimensions":["...","..."]}

可选维度包括：正确性、并发安全、错误处理、性能、可读性、安全性、可测试性、架构设计、API 设计、代码风格`

	messages := []llmlib.Message{
		llmlib.NewSystemMessage(system),
		llmlib.NewUserMessage(code),
	}

	out, err := p.Chat(ctx, cfg, messages)
	if err != nil {
		return reviewPlan{}, err
	}

	return parseReviewPlan(out.Content)
}

func parseReviewPlan(content string) (reviewPlan, error) {
	var plan reviewPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return reviewPlan{}, fmt.Errorf("parse review plan: %w", err)
	}
	return plan, nil
}
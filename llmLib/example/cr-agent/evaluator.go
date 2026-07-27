package main

import (
	"context"
	"encoding/json"
	"fmt"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

type evaluationResult struct {
	Score    float64 `json:"score"`
	Feedback string  `json:"feedback"`
}

func evaluateReport(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, code string, results reviewResults) (evaluationResult, error) {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return evaluationResult{}, err
	}

	system := `你是代码审查报告评估专家。请评估以下审查报告是否充分。

评估标准：
1. 是否遗漏了明显问题
2. 发现的问题是否具体可操作
3. 修复建议是否合理

输出格式：只输出 JSON {"score":0.0-1.0,"feedback":"改进建议"}
- score >= 0.8 表示报告充分，可以结束
- score < 0.8 表示需要补充审查

feedback 字段说明需要补充哪些维度的审查或哪些方面需要改进。`

	messages := []llmlib.Message{
		llmlib.NewSystemMessage(system),
		llmlib.NewUserMessage(fmt.Sprintf("代码:\n%s\n\n审查报告:\n%s", code, string(resultsJSON))),
	}

	out, err := p.Chat(ctx, cfg, messages)
	if err != nil {
		return evaluationResult{}, err
	}

	return parseEvaluationResult(out.Content)
}

func parseEvaluationResult(content string) (evaluationResult, error) {
	var result evaluationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return evaluationResult{}, fmt.Errorf("parse evaluation: %w", err)
	}
	return result, nil
}
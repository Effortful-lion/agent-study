package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```") {
		if idx := strings.Index(content, "\n"); idx != -1 {
			content = content[idx+1:]
		}
		if end := strings.LastIndex(content, "```"); end != -1 {
			content = content[:end]
		}
		content = strings.TrimSpace(content)
	}

	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		content = content[firstBrace : lastBrace+1]
	}

	return content
}

type reviewFinding struct {
	Dimension string `json:"dimension"`
	Issues    []struct {
		Severity    string `json:"severity"`
		Location    string `json:"location"`
		Description string `json:"description"`
		Suggestion  string `json:"suggestion"`
	} `json:"issues"`
}

type reviewResults struct {
	Findings []reviewFinding `json:"findings"`
}

func generateReviews(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, code string, dimensions []string) (reviewResults, error) {
	results := make([]reviewFinding, 0, len(dimensions))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(dimensions))

	for _, dim := range dimensions {
		wg.Add(1)
		go func(dimension string) {
			defer wg.Done()

			finding, err := reviewDimension(ctx, p, cfg, code, dimension)
			if err != nil {
				errCh <- fmt.Errorf("dimension %q: %w", dimension, err)
				return
			}

			mu.Lock()
			results = append(results, finding)
			mu.Unlock()
		}(dim)
	}

	wg.Wait()
	close(errCh)

	var firstErr error
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return reviewResults{}, firstErr
	}

	return reviewResults{Findings: results}, nil
}

func reviewDimension(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, code, dimension string) (reviewFinding, error) {
	system := fmt.Sprintf(`你是专注于"%s"维度的 Go 代码审查专家。
请针对以下代码进行深度审查，输出结构化的审查发现。

审查要求：
1. 找出该维度下的所有问题
2. 每个问题包含：严重程度(high/medium/low)、位置(行号/函数名)、描述、修复建议
3. 只输出 JSON: {"dimension":"%s","issues":[{"severity":"...","location":"...","description":"...","suggestion":"..."}]}
4. 即使没有问题也输出，issues 为空数组`, dimension, dimension)

	messages := []llmlib.Message{
		llmlib.NewSystemMessage(system),
		llmlib.NewUserMessage(code),
	}

	out, err := p.Chat(ctx, cfg, messages)
	if err != nil {
		return reviewFinding{}, err
	}

	var finding reviewFinding
	if err := json.Unmarshal([]byte(extractJSON(out.Content)), &finding); err != nil {
		return reviewFinding{}, fmt.Errorf("parse review: %w\nraw: %s", err, out.Content)
	}

	finding.Dimension = dimension
	return finding, nil
}

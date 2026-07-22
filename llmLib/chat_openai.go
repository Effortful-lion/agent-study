// 文件职责：
// - 实现 OpenAI 兼容协议的同步和流式聊天调用。
// - 供 OpenAI 及兼容该协议的多家服务商共享复用。

package llmlib

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// OpenAIChat 使用 OpenAI 兼容聊天接口发起同步请求，并把响应解析为统一结构。
func OpenAIChat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	chatReq := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 将统一请求结构写入 OpenAI 兼容的 /chat/completions 接口。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	// 只提取首个 choice 文本和 usage 信息，其余字段保持由上游自行扩展。
	var raw struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, errors.New("parse response: choices is empty")
	}

	return &ChatResponse{
		Content:      raw.Choices[0].Message.Content,
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}, nil
}

// OpenAIChatStream 使用 OpenAI 兼容流接口发起请求，并把 SSE 事件转换为统一流式片段。
func OpenAIChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	stream := make(chan StreamChunk)

	url := cfg.BaseURL + "/chat/completions"
	chatReq := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	go func() {
		defer close(stream)

		// 在独立 goroutine 中持续读取上游流，避免阻塞调用方。
		client := NewClient()
		resp, err := client.Do(req)
		if err != nil {
			stream <- StreamChunk{Err: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			stream <- StreamChunk{
				Err: fmt.Errorf("chat stream failed: status=%d body=%s", resp.StatusCode, string(b)),
			}
			return
		}

		if err := ParseSSE(resp.Body, func(data []byte) error {
			// 把每个 data 事件提取为文本增量，遇到完成标记时主动结束读取。
			delta, done, err := parseOpenAIDelta(data)
			if err != nil {
				return err
			}
			if done {
				return io.EOF
			}
			if delta != "" {
				select {
				case stream <- StreamChunk{Content: delta}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}); err != nil && err != io.EOF {
			stream <- StreamChunk{Err: err}
		}
	}()

	return stream, nil
}

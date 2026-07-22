// 文件职责：
// - 实现 Anthropic Claude 原生消息协议的同步和流式调用。
// - 供 Claude provider 在统一接口下接入非 OpenAI 兼容的上游服务。

package llmlib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ClaudeChat 使用 Claude 消息接口发起同步请求，并转换为统一响应结构。
func ClaudeChat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	chatReq := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   false,
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Claude 使用 /v1/messages 端点和 x-api-key 头部完成鉴权。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)

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

	var raw struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Claude 会把内容拆成多个 block，这里按顺序拼接为最终文本。
	var content string
	for _, c := range raw.Content {
		content += c.Text
	}
	return &ChatResponse{
		Content:      content,
		InputTokens:  raw.Usage.InputTokens,
		OutputTokens: raw.Usage.OutputTokens,
	}, nil
}

// ClaudeChatStream 使用 Claude 的 SSE 流接口持续输出文本增量。
func ClaudeChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	stream := make(chan StreamChunk)

	url := cfg.BaseURL + "/v1/messages"
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
	req.Header.Set("x-api-key", cfg.APIKey)

	go func() {
		defer close(stream)

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
			if string(data) == "[DONE]" {
				return io.EOF
			}
			// 仅转发 Claude 的 content_block_delta 事件，其余事件类型直接忽略。
			var raw struct {
				Type  string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return fmt.Errorf("decode stream chunk: %w", err)
			}
			if raw.Type == "content_block_delta" {
				select {
				case stream <- StreamChunk{Content: raw.Delta.Text}:
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

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

func normalizeArgs(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return args
	}
	var argsStr string
	if err := json.Unmarshal(args, &argsStr); err == nil {
		var obj map[string]any
		if err := json.Unmarshal([]byte(argsStr), &obj); err == nil {
			normalized, _ := json.Marshal(obj)
			return normalized
		}
	}
	return args
}

// OpenAIChat 使用 OpenAI 兼容聊天接口发起同步请求，并把响应解析为统一结构。
func OpenAIChat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, nil)
}

// OpenAIChatWithTools 使用 OpenAI 兼容聊天接口发起带工具调用的同步请求。
func OpenAIChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	chatReq := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w\nURL: %s\nModel: %s", err, url, cfg.Model)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
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

	var toolCalls []ToolCall
	for _, tc := range raw.Choices[0].Message.ToolCalls {
		args := normalizeArgs(tc.Function.Arguments)
		toolCalls = append(toolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return &ChatResponse{
		Content:      raw.Choices[0].Message.Content,
		ToolCalls:    toolCalls,
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}, nil
}

// OpenAIChatStream 使用 OpenAI 兼容流接口发起请求，并把 SSE 事件转换为统一流式片段。
func OpenAIChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, nil)
}

// OpenAIChatStreamWithTools 使用 OpenAI 兼容流接口发起带工具调用的请求。
func OpenAIChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	stream := make(chan StreamChunk)

	url := cfg.BaseURL + "/chat/completions"
	chatReq := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
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
			delta, done, toolCalls, err := parseOpenAIDeltaWithTools(data)
			if err != nil {
				return err
			}
			if done {
				if len(toolCalls) > 0 {
					select {
					case stream <- StreamChunk{ToolCalls: toolCalls}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
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

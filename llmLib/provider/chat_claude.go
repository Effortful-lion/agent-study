// 文件职责：
// - 实现 Anthropic Claude 原生消息协议的同步和流式调用。
// - 供 Claude provider 在统一接口下接入非 OpenAI 兼容的上游服务。

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/lg"
	"github.com/Effortful-lion/agent-study/llmLib/transport"
)

// ClaudeChat 使用 Claude 消息接口发起同步请求，并转换为统一响应结构。
func ClaudeChat(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (*core.ChatResponse, error) {
	return ClaudeChatWithTools(ctx, cfg, messages, nil)
}

// ClaudeChatWithTools 使用 Claude 消息接口发起带工具调用的同步请求。
func ClaudeChatWithTools(ctx context.Context, cfg core.LLMConfig, messages []core.Message, tools []core.ToolDef) (*core.ChatResponse, error) {
	type claudeTool struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	var claudeTools []claudeTool
	for _, t := range tools {
		claudeTools = append(claudeTools, claudeTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	reqBody := struct {
		Model    string        `json:"model"`
		Messages []core.Message `json:"messages"`
		Tools    []claudeTool  `json:"tools,omitempty"`
	}{
		Model:    cfg.Model,
		Messages: messages,
	}
	if len(claudeTools) > 0 {
		reqBody.Tools = claudeTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		lg.Frame.Error("claude: 序列化请求失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		lg.Frame.Error("claude: 创建 HTTP 请求失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)

	client := transport.NewClient()
	resp, err := client.Do(req)
	if err != nil {
		lg.Frame.Error("claude: HTTP 请求失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		lg.Frame.Error("claude: 非 2xx 响应", lg.Fields{"status": resp.StatusCode})
		return nil, fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var raw struct {
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			ToolUse *struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"tool_use"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		lg.Frame.Error("claude: 解析响应失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var content string
	var toolCalls []core.ToolCall
	for _, c := range raw.Content {
		if c.Type == "text" {
			content += c.Text
		} else if c.Type == "tool_use" && c.ToolUse != nil {
			toolCalls = append(toolCalls, core.ToolCall{
				ID:   c.ToolUse.ID,
				Name: c.ToolUse.Name,
				Args: c.ToolUse.Input,
			})
		}
	}

	return &core.ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		InputTokens:  raw.Usage.InputTokens,
		OutputTokens: raw.Usage.OutputTokens,
	}, nil
}

// ClaudeChatStream 使用 Claude 的 SSE 流接口持续输出文本增量。
func ClaudeChatStream(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (<-chan core.StreamChunk, error) {
	return ClaudeChatStreamWithTools(ctx, cfg, messages, nil)
}

// ClaudeChatStreamWithTools 使用 Claude 的 SSE 流接口发起带工具调用的请求。
func ClaudeChatStreamWithTools(ctx context.Context, cfg core.LLMConfig, messages []core.Message, tools []core.ToolDef) (<-chan core.StreamChunk, error) {
	stream := make(chan core.StreamChunk)

	type claudeTool struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	var claudeTools []claudeTool
	for _, t := range tools {
		claudeTools = append(claudeTools, claudeTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}

	reqBody := struct {
		Model    string        `json:"model"`
		Messages []core.Message `json:"messages"`
		Tools    []claudeTool  `json:"tools,omitempty"`
		Stream   bool          `json:"stream"`
	}{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   true,
	}
	if len(claudeTools) > 0 {
		reqBody.Tools = claudeTools
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		lg.Frame.Error("claude: 流式请求序列化失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := cfg.BaseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		lg.Frame.Error("claude: 流式创建 HTTP 请求失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", cfg.APIKey)

	go func() {
		defer close(stream)

		client := transport.NewClient()
		resp, err := client.Do(req)
		if err != nil {
			stream <- core.StreamChunk{Err: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			stream <- core.StreamChunk{
				Err: fmt.Errorf("chat stream failed: status=%d body=%s", resp.StatusCode, string(b)),
			}
			return
		}

		if err := ParseSSE(resp.Body, func(data []byte) error {
			if string(data) == "[DONE]" {
				return io.EOF
			}
			var raw struct {
				Type  string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
				ContentBlock struct {
					Type    string `json:"type"`
					ToolUse *struct {
						ID    string          `json:"id"`
						Name  string          `json:"name"`
						Input json.RawMessage `json:"input"`
					} `json:"tool_use"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return fmt.Errorf("decode stream chunk: %w", err)
			}
			if raw.Type == "content_block_delta" && raw.Delta.Text != "" {
				select {
				case stream <- core.StreamChunk{Content: raw.Delta.Text}:
				case <-ctx.Done():
					return ctx.Err()
				}
			} else if raw.Type == "content_block" && raw.ContentBlock.Type == "tool_use" && raw.ContentBlock.ToolUse != nil {
				select {
				case stream <- core.StreamChunk{ToolCalls: []core.ToolCall{{
					ID:   raw.ContentBlock.ToolUse.ID,
					Name: raw.ContentBlock.ToolUse.Name,
					Args: raw.ContentBlock.ToolUse.Input,
				}}}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}); err != nil && err != io.EOF {
			stream <- core.StreamChunk{Err: err}
		}
	}()

	return stream, nil
}

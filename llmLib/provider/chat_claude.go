// 文件职责：
// - 实现 Anthropic Claude 原生消息协议的同步和流式调用。
// - 通过 anthropic_adapter.go 中的边界映射层，将统一消息格式转换为 Anthropic 原生格式。
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
//
// 边界映射流程：
//  1. 统一消息 → Anthropic 原生格式（system→顶层字段、ToolRole→tool_result、ToolCalls→tool_use）
//  2. 统一 ToolDef → Anthropic tools 格式（{type,function} → {name,description,input_schema}）
//  3. 发送 HTTP 请求到 Anthropic Messages API
//  4. Anthropic 原生响应 → 统一 ChatResponse（tool_use→ToolCall）
func ClaudeChatWithTools(ctx context.Context, cfg core.LLMConfig, messages []core.Message, tools []core.ToolDef) (*core.ChatResponse, error) {
	// 步骤 1-2：边界映射 - 统一格式 → Anthropic 原生格式
	systemPrompt, anthMessages := toAnthropicMessages(messages)
	anthTools := toAnthropicTools(tools)

	// 构建 Anthropic 请求体
	reqBody := struct {
		Model     string             `json:"model"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		Tools     []anthropicTool    `json:"tools,omitempty"`
		MaxTokens int                `json:"max_tokens"`
	}{
		Model:     cfg.Model,
		System:    systemPrompt,
		Messages:  anthMessages,
		Tools:     anthTools,
		MaxTokens: 4096,
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
	req.Header.Set("anthropic-version", "2023-06-01")

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

	// 步骤 4：边界映射 - Anthropic 原生响应 → 统一格式
	var raw anthropicRawResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		lg.Frame.Error("claude: 解析响应失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return parseAnthropicResponse(raw), nil
}

// ClaudeChatStream 使用 Claude 的 SSE 流接口持续输出文本增量。
func ClaudeChatStream(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (<-chan core.StreamChunk, error) {
	return ClaudeChatStreamWithTools(ctx, cfg, messages, nil)
}

// ClaudeChatStreamWithTools 使用 Claude 的 SSE 流接口发起带工具调用的请求。
//
// 边界映射流程与 ClaudeChatWithTools 相同，但使用流式传输。
// 流式事件中 tool_use 通过 content_block_start 事件传递，文本增量通过 content_block_delta 事件传递。
func ClaudeChatStreamWithTools(ctx context.Context, cfg core.LLMConfig, messages []core.Message, tools []core.ToolDef) (<-chan core.StreamChunk, error) {
	stream := make(chan core.StreamChunk)

	// 边界映射 - 统一格式 → Anthropic 原生格式
	systemPrompt, anthMessages := toAnthropicMessages(messages)
	anthTools := toAnthropicTools(tools)

	reqBody := struct {
		Model     string             `json:"model"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		Tools     []anthropicTool    `json:"tools,omitempty"`
		MaxTokens int                `json:"max_tokens"`
		Stream    bool               `json:"stream"`
	}{
		Model:     cfg.Model,
		System:    systemPrompt,
		Messages:  anthMessages,
		Tools:     anthTools,
		MaxTokens: 4096,
		Stream:    true,
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
	req.Header.Set("anthropic-version", "2023-06-01")

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
					Type    string            `json:"type"`
					ToolUse *anthropicToolUse `json:"tool_use"`
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
			} else if raw.Type == "content_block_start" && raw.ContentBlock.Type == "tool_use" && raw.ContentBlock.ToolUse != nil {
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

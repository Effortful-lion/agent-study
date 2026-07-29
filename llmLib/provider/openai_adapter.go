// 文件职责：
// - 实现统一消息格式与 OpenAI 兼容协议之间的边界映射。
// - 统一格式本身就是 OpenAI 兼容的，此文件提供显式的转换函数以保持与 Anthropic 适配器的对称性。
// - 供 OpenAICompatibleChat / OpenAICompatibleChatStream 在发送请求前调用。

package provider

import (
	"encoding/json"

	"github.com/Effortful-lion/agent-study/llmLib/core"
)

// ============================================================================
// 边界映射：统一格式 → OpenAI 兼容格式（直通，无需转换）
// ============================================================================

// toOpenAIRequest 将统一格式构建为 OpenAI 兼容的 ChatRequest。
// 由于统一格式本身就是 OpenAI 兼容的，此函数直接透传，无需转换。
// 保留此函数以保持与 Anthropic 适配器的对称性，便于未来扩展。
func toOpenAIRequest(cfg core.LLMConfig, messages []core.Message, tools []core.ToolDef, stream bool) core.ChatRequest {
	return core.ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   stream,
		Tools:    tools,
	}
}

// ============================================================================
// 边界映射：OpenAI 兼容格式 → 统一格式
// ============================================================================

// parseOpenAIResponse 将 OpenAI 兼容的原始响应解析为统一的 core.ChatResponse。
// 注意：tool_calls 中的 function.arguments 可能是 JSON 字符串或 JSON 对象，需要 normalizeArgs 处理。
func parseOpenAIResponse(raw openaiRawResponse) *core.ChatResponse {
	if len(raw.Choices) == 0 {
		return &core.ChatResponse{}
	}

	var toolCalls []core.ToolCall
	for _, tc := range raw.Choices[0].Message.ToolCalls {
		args := normalizeArgs(tc.Function.Arguments)
		toolCalls = append(toolCalls, core.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return &core.ChatResponse{
		Content:      raw.Choices[0].Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: raw.Choices[0].FinishReason,
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}
}

// openaiRawResponse 是 OpenAI Chat Completions API 的原始响应结构。
type openaiRawResponse struct {
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
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

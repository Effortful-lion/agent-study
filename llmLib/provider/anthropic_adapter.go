// 文件职责：
// - 实现统一消息格式与 Anthropic Claude 原生消息协议之间的双向边界映射。
// - 处理三大差异：system 消息→顶层字段、ToolRole→tool_result 用户消息、ToolCalls→tool_use 内容块。
// - 供 ClaudeChatWithTools / ClaudeChatStreamWithTools 在发送请求前调用。

package provider

import (
	"encoding/json"

	"github.com/Effortful-lion/agent-study/llmLib/core"
)

// ============================================================================
// Anthropic 原生消息类型
// ============================================================================

// anthropicMessage 是 Anthropic Messages API 的原生消息格式。
// 与统一的 core.Message 不同，Anthropic 的消息结构更灵活，支持 content 数组。
type anthropicMessage struct {
	Role    string               `json:"role"`
	Content []anthropicContent   `json:"content"`
}

// anthropicContent 是 Anthropic 消息中的内容块，可以是文本、工具调用或工具结果。
type anthropicContent struct {
	Type       string              `json:"type"`
	Text       string              `json:"text,omitempty"`
	ToolUse    *anthropicToolUse   `json:"tool_use,omitempty"`
	ToolResult *anthropicToolResult `json:"tool_result,omitempty"`
}

// anthropicToolUse 表示 Anthropic 的 tool_use 内容块（助手发起工具调用）。
type anthropicToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// anthropicToolResult 表示 Anthropic 的 tool_result 内容块（用户回填工具结果）。
type anthropicToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

// anthropicTool 是 Anthropic 的 tools 数组元素格式。
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ============================================================================
// 边界映射：统一格式 → Anthropic 原生格式
// ============================================================================

// toAnthropicMessages 将统一的 core.Message 列表转换为 Anthropic 原生消息格式。
//
// 映射规则：
//   - System 消息 → 提取到顶层 system 字段（不进入 messages 数组）
//   - User 消息 → {role: "user", content: [{type: "text", text: "..."}]}
//   - Assistant 消息（无 tool_calls）→ {role: "assistant", content: [{type: "text", text: "..."}]}
//   - Assistant 消息（有 tool_calls）→ {role: "assistant", content: [{type: "tool_use", ...}]}
//   - ToolRole 消息 → {role: "user", content: [{type: "tool_result", tool_use_id: "...", content: "..."}]}
//
// 返回值：systemPrompt（顶层系统提示）、messages（Anthropic 格式的消息列表）。
func toAnthropicMessages(messages []core.Message) (systemPrompt string, anthMessages []anthropicMessage) {
	anthMessages = make([]anthropicMessage, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case core.System:
			// Anthropic 不支持 system 角色作为消息，提取到顶层 system 字段
			if systemPrompt == "" {
				systemPrompt = msg.Content
			} else {
				systemPrompt += "\n\n" + msg.Content
			}

		case core.User:
			anthMessages = append(anthMessages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{
					{Type: "text", Text: msg.Content},
				},
			})

		case core.Assistant:
			if len(msg.ToolCalls) > 0 {
				// 有工具调用：转换为 tool_use 内容块
				contents := make([]anthropicContent, 0, len(msg.ToolCalls)+1)
				if msg.Content != "" {
					contents = append(contents, anthropicContent{Type: "text", Text: msg.Content})
				}
				for _, tc := range msg.ToolCalls {
					contents = append(contents, anthropicContent{
						Type: "tool_use",
						ToolUse: &anthropicToolUse{
							ID:    tc.ID,
							Name:  tc.Name,
							Input: tc.Args,
						},
					})
				}
				anthMessages = append(anthMessages, anthropicMessage{
					Role:    "assistant",
					Content: contents,
				})
			} else {
				// 纯文本回复
				anthMessages = append(anthMessages, anthropicMessage{
					Role: "assistant",
					Content: []anthropicContent{
						{Type: "text", Text: msg.Content},
					},
				})
			}

		case core.ToolRole:
			// Anthropic 不支持 tool 角色，工具结果必须作为 user 消息的 tool_result 内容块
			anthMessages = append(anthMessages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{
					{
						Type: "tool_result",
						ToolResult: &anthropicToolResult{
							ToolUseID: msg.ToolCallID,
							Content:   msg.Content,
						},
					},
				},
			})
		}
	}

	return systemPrompt, anthMessages
}

// toAnthropicTools 将统一的 core.ToolDef 列表转换为 Anthropic 的 tools 格式。
// 核心差异：Anthropic 使用 {name, description, input_schema} 而非 {type, function:{name, description, parameters}}。
func toAnthropicTools(tools []core.ToolDef) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	anthTools := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		anthTools = append(anthTools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return anthTools
}

// ============================================================================
// 边界映射：Anthropic 原生格式 → 统一格式
// ============================================================================

// parseAnthropicResponse 将 Anthropic 原生响应解析为统一的 core.ChatResponse。
// 处理 content 数组中的 text 和 tool_use 内容块。
func parseAnthropicResponse(raw anthropicRawResponse) *core.ChatResponse {
	var content string
	var toolCalls []core.ToolCall

	for _, c := range raw.Content {
		switch c.Type {
		case "text":
			content += c.Text
		case "tool_use":
			if c.ToolUse != nil {
				toolCalls = append(toolCalls, core.ToolCall{
					ID:   c.ToolUse.ID,
					Name: c.ToolUse.Name,
					Args: c.ToolUse.Input,
				})
			}
		}
	}

	return &core.ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		InputTokens:  raw.Usage.InputTokens,
		OutputTokens: raw.Usage.OutputTokens,
	}
}

// anthropicRawResponse 是 Anthropic Messages API 的原始响应结构。
type anthropicRawResponse struct {
	Content []struct {
		Type    string             `json:"type"`
		Text    string             `json:"text"`
		ToolUse *anthropicToolUse  `json:"tool_use"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
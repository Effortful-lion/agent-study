// 文件职责：
// - 暴露 llmlib 的主调用入口，负责按服务商名称组装配置并分发请求。
// - 主要包含同步调用、流式调用、默认地址选择和常用消息构造器。
// - 供业务侧以统一接口访问不同模型服务商。

package llmlib

import (
	"context"
	"fmt"
)

// Chat 按服务商名称发起一次同步聊天调用，并返回统一响应结构。
func Chat(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	// 先解析服务商实现，避免配置装配完成后才发现 provider 不存在。
	p, err := NewProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// 调用方未显式指定地址时，回退到服务商的默认入口。
	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}

	return p.Chat(ctx, cfg, messages)
}

// ChatStream 按服务商名称发起流式聊天调用，并返回统一的流式输出通道。
func ChatStream(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (<-chan StreamChunk, error) {
	p, err := NewProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// 流式调用和同步调用共用同一套默认地址装配逻辑。
	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}

	return p.ChatStream(ctx, cfg, messages)
}

// getDefaultBaseURL 根据服务商名称返回内置默认接口地址。
func getDefaultBaseURL(providerName string) string {
	switch providerName {
	case "openai":
		return OPENAI_BASEURL
	case "doubao":
		return DOUBAO_BASEURL
	case "deepseek":
		return DEEPSEEK_BASEURL
	case "claude":
		return CLAUDE_BASEURL
	case "zhipu":
		return ZHIPU_BASEURL
	case "tongyi":
		return TONGYI_BASEURL
	case "kimi":
		return KIMI_BASEURL
	default:
		return ""
	}
}

// NewMessage 构造一条带角色的消息，供调用方快速组装上下文。
func NewMessage(role Role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// NewUserMessage 构造用户输入消息。
func NewUserMessage(content string) Message {
	return NewMessage(User, content)
}

// NewSystemMessage 构造系统提示消息。
func NewSystemMessage(content string) Message {
	return NewMessage(System, content)
}

// NewAssistantMessage 构造助手历史回复消息。
func NewAssistantMessage(content string) Message {
	return NewMessage(Assistant, content)
}

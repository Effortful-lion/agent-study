package llmlib

import (
	"context"
	"fmt"
)

// Provider 定义了 LLM 服务商的接口
type Provider interface {
	Name() string
	Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error)
	ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error)
}

// DeepSeekProvider DeepSeek 服务商实现，使用 OpenAI 兼容协议
type DeepSeekProvider struct{}

func NewDeepSeekProvider() *DeepSeekProvider {
	return &DeepSeekProvider{}
}

func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

func (p *DeepSeekProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *DeepSeekProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// DoubaoProvider 豆包服务商实现，使用 OpenAI 兼容协议
type DoubaoProvider struct{}

func NewDoubaoProvider() *DoubaoProvider {
	return &DoubaoProvider{}
}

func (p *DoubaoProvider) Name() string {
	return "doubao"
}

func (p *DoubaoProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *DoubaoProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// ClaudeProvider Claude 服务商实现，使用 Claude 协议
type ClaudeProvider struct{}

func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return ClaudeChat(ctx, cfg, messages)
}

func (p *ClaudeProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return ClaudeChatStream(ctx, cfg, messages)
}

// OpenAIProvider OpenAI 服务商实现，使用 OpenAI 兼容协议
type OpenAIProvider struct{}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// ZhipuProvider 智谱 AI 服务商实现，使用 OpenAI 兼容协议
type ZhipuProvider struct{}

func NewZhipuProvider() *ZhipuProvider {
	return &ZhipuProvider{}
}

func (p *ZhipuProvider) Name() string {
	return "zhipu"
}

func (p *ZhipuProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *ZhipuProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// TongyiProvider 阿里云通义服务商实现，使用 OpenAI 兼容协议
type TongyiProvider struct{}

func NewTongyiProvider() *TongyiProvider {
	return &TongyiProvider{}
}

func (p *TongyiProvider) Name() string {
	return "tongyi"
}

func (p *TongyiProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *TongyiProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// KimiProvider 月之暗面 Kimi 服务商实现，使用 OpenAI 兼容协议
type KimiProvider struct{}

func NewKimiProvider() *KimiProvider {
	return &KimiProvider{}
}

func (p *KimiProvider) Name() string {
	return "kimi"
}

func (p *KimiProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *KimiProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// NewProvider 根据名称创建对应的服务商实例
// 支持的名称: "deepseek", "doubao", "claude", "openai", "zhipu", "tongyi", "kimi"
func NewProvider(name string) (Provider, error) {
	switch name {
	case "deepseek":
		return NewDeepSeekProvider(), nil
	case "doubao":
		return NewDoubaoProvider(), nil
	case "claude":
		return NewClaudeProvider(), nil
	case "openai":
		return NewOpenAIProvider(), nil
	case "zhipu":
		return NewZhipuProvider(), nil
	case "tongyi":
		return NewTongyiProvider(), nil
	case "kimi":
		return NewKimiProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

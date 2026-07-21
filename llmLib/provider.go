package llmlib

import (
	"context"
	"fmt"
)

type Provider interface {
	Name() string
	Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error)
	ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error)
}

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

func NewProvider(name string) (Provider, error) {
	switch name {
	case "deepseek":
		return NewDeepSeekProvider(), nil
	case "doubao":
		return NewDoubaoProvider(), nil
	case "claude":
		return NewClaudeProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

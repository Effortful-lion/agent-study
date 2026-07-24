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

type ToolCallProvider interface {
	Provider
	ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error)
	ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error)
}

func NewProvider(name string) (Provider, error) {
	switch name {
	case ProviderDeepSeek:
		return NewDeepSeekProvider(), nil
	case ProviderDoubao:
		return NewDoubaoProvider(), nil
	case ProviderClaude:
		return NewClaudeProvider(), nil
	case ProviderOpenAI:
		return NewOpenAIProvider(), nil
	case ProviderZhipu:
		return NewZhipuProvider(), nil
	case ProviderTongyi:
		return NewTongyiProvider(), nil
	case ProviderKimi:
		return NewKimiProvider(), nil
	case ProviderQwen:
		return NewQwenProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
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

func (p *DeepSeekProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *DeepSeekProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
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

func (p *DoubaoProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *DoubaoProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
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

func (p *ClaudeProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return ClaudeChatWithTools(ctx, cfg, messages, tools)
}

func (p *ClaudeProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return ClaudeChatStreamWithTools(ctx, cfg, messages, tools)
}

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

func (p *OpenAIProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *OpenAIProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
}

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

func (p *ZhipuProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *ZhipuProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
}

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

func (p *TongyiProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *TongyiProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
}

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

func (p *KimiProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *KimiProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
}

// QwenProvider 支持本地或远程部署的 Qwen 系列模型（OpenAI 兼容协议）。
type QwenProvider struct{}

func NewQwenProvider() *QwenProvider {
	return &QwenProvider{}
}

func (p *QwenProvider) Name() string {
	return "qwen"
}

func (p *QwenProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

func (p *QwenProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

func (p *QwenProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return OpenAIChatWithTools(ctx, cfg, messages, tools)
}

func (p *QwenProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return OpenAIChatStreamWithTools(ctx, cfg, messages, tools)
}

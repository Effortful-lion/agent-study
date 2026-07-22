// 文件职责：
// - 定义服务商抽象接口以及各家模型服务的适配实现。
// - 负责把 provider 名称映射到具体协议处理函数，供主入口和路由层调用。

package llmlib

import (
	"context"
	"fmt"
)

// Provider 抽象单个模型服务商应具备的同步和流式调用能力。
type Provider interface {
	// Name 返回服务商标识，供日志、路由结果和错误信息展示。
	Name() string
	// Chat 发起一次同步请求，并把上游结果转换为统一响应结构。
	Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error)
	// ChatStream 发起一次流式请求，并把上游事件转换为统一数据块。
	ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error)
}

// DeepSeekProvider 适配 DeepSeek 服务，内部复用 OpenAI 兼容协议实现。
type DeepSeekProvider struct{}

// NewDeepSeekProvider 创建 DeepSeek 服务商实例。
func NewDeepSeekProvider() *DeepSeekProvider {
	return &DeepSeekProvider{}
}

// Name 返回 DeepSeek 的 provider 标识。
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// Chat 使用 OpenAI 兼容协议向 DeepSeek 发起同步请求。
func (p *DeepSeekProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 兼容协议向 DeepSeek 发起流式请求。
func (p *DeepSeekProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// DoubaoProvider 适配豆包服务，内部复用 OpenAI 兼容协议实现。
type DoubaoProvider struct{}

// NewDoubaoProvider 创建豆包服务商实例。
func NewDoubaoProvider() *DoubaoProvider {
	return &DoubaoProvider{}
}

// Name 返回豆包的 provider 标识。
func (p *DoubaoProvider) Name() string {
	return "doubao"
}

// Chat 使用 OpenAI 兼容协议向豆包发起同步请求。
func (p *DoubaoProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 兼容协议向豆包发起流式请求。
func (p *DoubaoProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// ClaudeProvider 适配 Anthropic Claude 服务，使用其原生消息协议。
type ClaudeProvider struct{}

// NewClaudeProvider 创建 Claude 服务商实例。
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

// Name 返回 Claude 的 provider 标识。
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Chat 使用 Claude 原生协议发起同步请求。
func (p *ClaudeProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return ClaudeChat(ctx, cfg, messages)
}

// ChatStream 使用 Claude 原生协议发起流式请求。
func (p *ClaudeProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return ClaudeChatStream(ctx, cfg, messages)
}

// OpenAIProvider 适配 OpenAI 服务，直接使用 OpenAI 协议实现。
type OpenAIProvider struct{}

// NewOpenAIProvider 创建 OpenAI 服务商实例。
func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{}
}

// Name 返回 OpenAI 的 provider 标识。
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Chat 使用 OpenAI 协议发起同步请求。
func (p *OpenAIProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 协议发起流式请求。
func (p *OpenAIProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// ZhipuProvider 适配智谱服务，内部复用 OpenAI 兼容协议实现。
type ZhipuProvider struct{}

// NewZhipuProvider 创建智谱服务商实例。
func NewZhipuProvider() *ZhipuProvider {
	return &ZhipuProvider{}
}

// Name 返回智谱的 provider 标识。
func (p *ZhipuProvider) Name() string {
	return "zhipu"
}

// Chat 使用 OpenAI 兼容协议向智谱发起同步请求。
func (p *ZhipuProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 兼容协议向智谱发起流式请求。
func (p *ZhipuProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// TongyiProvider 适配阿里云通义服务，内部复用 OpenAI 兼容协议实现。
type TongyiProvider struct{}

// NewTongyiProvider 创建通义服务商实例。
func NewTongyiProvider() *TongyiProvider {
	return &TongyiProvider{}
}

// Name 返回通义的 provider 标识。
func (p *TongyiProvider) Name() string {
	return "tongyi"
}

// Chat 使用 OpenAI 兼容协议向通义发起同步请求。
func (p *TongyiProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 兼容协议向通义发起流式请求。
func (p *TongyiProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// KimiProvider 适配月之暗面 Kimi 服务，内部复用 OpenAI 兼容协议实现。
type KimiProvider struct{}

// NewKimiProvider 创建 Kimi 服务商实例。
func NewKimiProvider() *KimiProvider {
	return &KimiProvider{}
}

// Name 返回 Kimi 的 provider 标识。
func (p *KimiProvider) Name() string {
	return "kimi"
}

// Chat 使用 OpenAI 兼容协议向 Kimi 发起同步请求。
func (p *KimiProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	return OpenAIChat(ctx, cfg, messages)
}

// ChatStream 使用 OpenAI 兼容协议向 Kimi 发起流式请求。
func (p *KimiProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return OpenAIChatStream(ctx, cfg, messages)
}

// NewProvider 按名称创建对应的服务商实例，供主入口和环境装配流程复用。
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

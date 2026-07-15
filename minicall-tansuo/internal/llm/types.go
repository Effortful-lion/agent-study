package llm

import "context"

// Role chat role
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Provider model provider
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// Message chat Prompt
type Message struct {
	Role    Role           `json:"role"`
	Content MessageContent `json:"content"`
}

// TextContent 快速构造纯文本 content，同时保留 MessageContent 的多模态扩展能力。
func TextContent(text string) MessageContent {
	return MessageContent{Text: text}
}

// ChatRequest req
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse resp
type ChatResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// StreamChunk 流式数据块
type StreamChunk struct {
	Content string
	Err     error
}

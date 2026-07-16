package main

type Role string

const (
	User     Role = "user"
	System   Role = "system"
	Assident Role = "assident"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"` // 承载多轮上下文
	Stream   bool      `json:"stream"`
}

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

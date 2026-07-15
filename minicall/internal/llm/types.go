package llm

// 用 llm.ChatRequest / ChatResponse / Message

type Role string

const (
	RoleUser Role = "user"
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

package llmlib

// ChatRequest 表示聊天请求的结构
type ChatRequest struct {
	Model    string    `json:"model"`    // 使用的模型名称
	Messages []Message `json:"messages"` // 消息列表
	Stream   bool      `json:"stream"`   // 是否启用流式响应
}

// ChatResponse 表示聊天响应的结构
type ChatResponse struct {
	Content      string // 回复内容
	InputTokens  int    // 输入 token 数量
	OutputTokens int    // 输出 token 数量
}

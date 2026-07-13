package internal

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ChatResponse 大模型对话完整非流式返回响应体
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"` // 当前轮次模型生成的回复消息
	} `json:"choices"` // 模型生成结果数组，通常仅返回1条结果

	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`     // 输入提示词消耗token数量
		CompletionTokens int `json:"completion_tokens"` // 模型输出回复消耗token数量
		TotalTokens      int `json:"total_tokens"`      // 本轮对话总消耗token（输入+输出）
	} `json:"usage"` // Token用量统计，用于计费、限流统计
}

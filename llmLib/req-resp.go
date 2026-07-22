// 文件职责：
// - 定义与上游模型接口交互时复用的通用请求和响应结构。
// - 主要供 OpenAI 兼容协议和 Claude 协议适配层组装数据时使用。

package llmlib

// ChatRequest 表示发送给模型服务商的标准聊天请求。
type ChatRequest struct {
	Model    string    `json:"model"`    // 目标模型名称，来自配置并写入请求体。
	Messages []Message `json:"messages"` // 对话消息列表，来自上层调用方。
	Stream   bool      `json:"stream"`   // 是否要求流式返回，决定上游响应模式。
}

// ChatResponse 表示从上游响应中提取出的统一结果。
type ChatResponse struct {
	Content      string // 模型最终回复内容，返回给调用方继续消费。
	InputTokens  int    // 输入 token 数量，来自上游 usage 信息或本地估算。
	OutputTokens int    // 输出 token 数量，来自上游 usage 信息或本地估算。
}

// 文件职责：
// - 定义聊天上下文中的基础消息结构。
// - 供请求组装、协议转换和响应解析流程统一复用。

package llmlib

// Message 表示一次聊天上下文中的单条消息，会直接序列化给上游模型接口。
type Message struct {
	Role    Role   `json:"role"`    // 消息角色，来自调用方并决定该消息在上下文中的身份。
	Content string `json:"content"` // 消息正文，来自提示词或历史对话内容。
}

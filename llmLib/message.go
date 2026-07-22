package llmlib

// Message 表示一条聊天消息
type Message struct {
	Role    Role   `json:"role"`    // 消息角色: User、System、Assistant
	Content string `json:"content"` // 消息内容
}

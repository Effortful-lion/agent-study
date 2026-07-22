package llmlib

// Role 表示消息的角色类型
type Role string

const (
	User      Role = "user"      // 用户角色，代表用户输入
	System    Role = "system"    // 系统角色，用于设置助手的行为和背景
	Assistant Role = "assistant" // 助手角色，代表模型的回复
)

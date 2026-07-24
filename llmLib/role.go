// 文件职责：
// - 定义聊天消息的角色枚举。
// - 供消息构造器、请求序列化和上游协议转换时统一约束角色取值。

package llmlib

// Role 表示消息在对话上下文中的身份类型。
type Role string

const (
	User      Role = "user"      // 用户角色，表示来自最终调用方的输入内容。
	System    Role = "system"    // 系统角色，用于注入行为约束、背景或全局提示。
	Assistant Role = "assistant" // 助手角色，表示历史回复或模型生成内容。
	ToolRole  Role = "tool"      // 工具角色，表示工具调用执行结果。
)

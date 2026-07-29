package core

func NewMessage(role Role, content string) Message {
	return Message{Role: role, Content: content}
}

func NewUserMessage(content string) Message {
	return Message{Role: User, Content: content}
}

func NewSystemMessage(content string) Message {
	return Message{Role: System, Content: content}
}

func NewAssistantMessage(content string) Message {
	return Message{Role: Assistant, Content: content}
}
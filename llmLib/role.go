package llmlib

type Role string

const (
	User      Role = "user"
	System    Role = "system"
	Assistant Role = "assistant"
)

package main

import "context"

type Claude struct{}

func NewClaude() *Claude {
	return &Claude{}
}

func (c *Claude) Name() string {
	return "claude"
}

func (c *Claude) Chat(ctx context.Context, cfg LLMConfig, question string) (*ChatResponse, error) {
	return ClaudeChat(ctx, cfg, question)
}

func (c *Claude) ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	return ClaudeChatStream(ctx, cfg, question)
}

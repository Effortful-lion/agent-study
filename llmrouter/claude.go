package main

import "context"

type Claude struct {
	baseProvider
}

func NewClaude() *Claude {
	c := &Claude{}
	c.baseProvider.Provider = c
	return c
}

func (c *Claude) Name() string {
	return "claude"
}

func (c *Claude) Chat(ctx context.Context, cfg LLMConfig, question string) error {
	return ClaudeChat(ctx, cfg, question)
}

func (c *Claude) ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	return ClaudeChatStream(ctx, cfg, question)
}

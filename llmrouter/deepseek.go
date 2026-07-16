package main

import "context"

type DeepSeek struct {
	baseProvider
}

func NewDeepSeek() *DeepSeek {
	d := &DeepSeek{}
	d.baseProvider.Provider = d
	return d
}

func (d *DeepSeek) Name() string {
	return "deepseek"
}

func (d *DeepSeek) Chat(ctx context.Context, cfg LLMConfig, question string) error {
	return GPTChat(ctx, cfg, question)
}

func (d *DeepSeek) ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	return GPTChatStream(ctx, cfg, question)
}

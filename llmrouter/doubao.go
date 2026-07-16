package main

import "context"

type Doubao struct{}

func NewDoubao() *Doubao {
	return &Doubao{}
}

func (d *Doubao) Name() string {
	return "doubao"
}

func (d *Doubao) Chat(ctx context.Context, cfg LLMConfig, question string) (*ChatResponse, error) {
	return GPTChat(ctx, cfg, question)
}

func (d *Doubao) ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	return GPTChatStream(ctx, cfg, question)
}

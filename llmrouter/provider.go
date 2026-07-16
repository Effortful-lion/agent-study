package main

// 提供 Provider 的抽象，适配多家 api 厂商

import (
	"context"
	"errors"
)

type Provider interface {
	Name() string
	Chat(ctx context.Context, cfg LLMConfig, question string) (*ChatResponse, error)
	ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error)
}

func NewProvider(name string) (Provider, error) {
	switch name {
	case "doubao":
		return NewDoubao(), nil
	case "deepseek":
		return NewDeepSeek(), nil
	case "claude":
		return NewClaude(), nil
	default:
		return nil, errors.New("unknown provider: " + name)
	}
}

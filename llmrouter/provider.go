package main

// 提供 Provider 的抽象，适配多家 api 厂商

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type Provider interface {
	Name() string
	Chat(ctx context.Context, cfg LLMConfig, question string) error
	ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error)
	// 下面两个可选，主要是为了 stdout 输出调用
	PrintChat(ctx context.Context, cfg LLMConfig, question string)
	PrintChatStream(ctx context.Context, cfg LLMConfig, question string)
}

type baseProvider struct {
	Provider
}

func (b *baseProvider) PrintChat(ctx context.Context, cfg LLMConfig, question string) {
	err := b.Provider.Chat(ctx, cfg, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (b *baseProvider) PrintChatStream(ctx context.Context, cfg LLMConfig, question string) {
	stream, err := b.Provider.ChatStream(ctx, cfg, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Fprintln(os.Stderr, chunk.Err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, chunk.Content)
		os.Stdout.Sync()
	}
}

func NewProvider(name string) (Provider, error) {
	switch name {
	case "deepseek":
		return NewDeepSeek(), nil
	case "claude":
		return NewClaude(), nil
	default:
		return nil, errors.New("unknown provider: " + name)
	}
}

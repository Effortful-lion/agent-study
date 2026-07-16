package main

import (
	"context"
	"fmt"
	"os"
)

func PrintChat(ctx context.Context, cfg LLMConfig, question string) {
	err := GPTChat(ctx, cfg, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func PrintChatStream(ctx context.Context, cfg LLMConfig, question string) {
	stream, err := GPTChatStream(ctx, cfg, question)
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

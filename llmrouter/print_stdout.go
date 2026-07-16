package main

import (
	"context"
	"fmt"
	"os"
)

func PrintRouterChat(ctx context.Context, router *Router, question string) {
	result, err := router.Chat(ctx, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, result.Response.Content)
	printRouteSummary(result.Provider, result.Model, result.Response, result.Cost)
}

func PrintRouterChatStream(ctx context.Context, router *Router, question string) {
	stream, err := router.ChatStream(ctx, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var result RouteStreamChunk
	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Fprintln(os.Stderr, chunk.Err)
			os.Exit(1)
		}
		if chunk.Content != "" {
			fmt.Fprint(os.Stdout, chunk.Content)
			os.Stdout.Sync()
		}
		if chunk.Done {
			result = chunk
		}
	}

	printRouteSummary(result.Provider, result.Model, result.Response, result.Cost)
}

func printRouteSummary(provider string, model string, resp *ChatResponse, cost float64) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "provider=%s\n", provider)
	fmt.Fprintf(os.Stdout, "model=%s\n", model)
	fmt.Fprintf(os.Stdout, "input_tokens=%d\n", resp.InputTokens)
	fmt.Fprintf(os.Stdout, "output_tokens=%d\n", resp.OutputTokens)
	fmt.Fprintf(os.Stdout, "estimated_cost=$%.6f\n", cost)
}

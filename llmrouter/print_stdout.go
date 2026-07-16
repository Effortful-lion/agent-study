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
	printRouteSummary(result.Provider, result.Model, result.Response, result.Cost, result.Latency)
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

	printRouteSummary(result.Provider, result.Model, result.Response, result.Cost, result.Latency)
}

func printRouteSummary(provider string, model string, resp *ChatResponse, cost float64, latency LatencySnapshot) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "provider=%s\n", provider)
	fmt.Fprintf(os.Stdout, "model=%s\n", model)
	fmt.Fprintf(os.Stdout, "input_tokens=%d\n", resp.InputTokens)
	fmt.Fprintf(os.Stdout, "output_tokens=%d\n", resp.OutputTokens)
	fmt.Fprintf(os.Stdout, "estimated_cost=$%.6f\n", cost)
	fmt.Fprintf(os.Stdout, "latency_samples=%d\n", latency.Samples)
	fmt.Fprintf(os.Stdout, "latency_p50_ms=%.2f\n", float64(latency.P50.Microseconds())/1000)
	fmt.Fprintf(os.Stdout, "latency_p95_ms=%.2f\n", float64(latency.P95.Microseconds())/1000)
}

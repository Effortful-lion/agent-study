package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lion/llmlib"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	question := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, "用法: llmrouter <你的问题>")
		os.Exit(1)
	}

	services, err := llmlib.LoadAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	strategy := llmlib.ReadStrategyFromEnv()
	router := llmlib.NewRouter(services, strategy)
	PrintRouterChatStream(ctx, router, question)
}

func PrintRouterChatStream(ctx context.Context, router *llmlib.Router, question string) {
	messages := []llmlib.Message{llmlib.NewUserMessage(question)}
	stream, err := router.ChatStream(ctx, messages)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Fprintln(os.Stderr, chunk.Err)
			os.Exit(1)
		}

		if chunk.Content != "" {
			fmt.Print(chunk.Content)
			os.Stdout.Sync()
		}

		if chunk.Done {
			fmt.Println()
			printRouteResult(chunk.Provider, chunk.Model, chunk.Cost, chunk.Latency, chunk.LastErrors)
			return
		}
	}
}

func printRouteResult(provider, model string, cost float64, latency llmlib.LatencySnapshot, lastErrors []error) {
	fmt.Printf("\n\n--- 路由结果 ---\n")
	fmt.Printf("Provider: %s\n", provider)
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("Cost: %.6f 元\n", cost)
	fmt.Printf("Latency - Samples: %d, P50: %v, P95: %v\n", latency.Samples, latency.P50, latency.P95)
	if len(lastErrors) > 0 {
		fmt.Printf("Failed Providers: %d\n", len(lastErrors))
	}
}

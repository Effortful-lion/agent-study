package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Router struct {
	services []LLMService
	strategy Strategy
	metrics  *LatencyMetrics
}

type RouteResult struct {
	Provider   string
	Model      string
	Response   *ChatResponse
	Cost       float64
	LastErrors []error
	Latency    LatencySnapshot
}

type RouteStreamChunk struct {
	Provider   string
	Model      string
	Content    string
	Err        error
	Done       bool
	Response   *ChatResponse
	Cost       float64
	LastErrors []error
	Latency    LatencySnapshot
}

func NewRouter(services []LLMService, strategy Strategy) *Router {
	return &Router{
		services: services,
		strategy: strategy,
		metrics:  NewLatencyMetrics(),
	}
}

func (r *Router) Chat(ctx context.Context, question string) (*RouteResult, error) {
	start := time.Now()
	// 定义时间记录器，需要退出前调用
	recordSnapshot := func() LatencySnapshot {
		r.metrics.Record(time.Since(start))
		return r.metrics.Snapshot()
	}

	if len(r.services) == 0 {
		recordSnapshot()
		return nil, errors.New("router has no services")
	}

	var errs []error
	for _, service := range SelectStrategy(r.strategy, r.services) {
		resp, err := service.Provider.Chat(ctx, service.Config, question)
		if err == nil {
			return &RouteResult{
				Provider:   service.Provider.Name(),
				Model:      service.Config.Model,
				Response:   resp,
				Cost:       estimateCost(service.Config, resp),
				LastErrors: errs,
				Latency:    recordSnapshot(),
			}, nil
		}

		errs = append(errs, fmt.Errorf("%s: %w", service.Provider.Name(), err))
		if ctx.Err() != nil {
			recordSnapshot()
			return nil, ctx.Err()
		}
	}

	recordSnapshot()
	return nil, fmt.Errorf("all providers failed: %s", joinErrors(errs))
}

func (r *Router) ChatStream(ctx context.Context, question string) (<-chan RouteStreamChunk, error) {
	if len(r.services) == 0 {
		return nil, errors.New("router has no services")
	}

	out := make(chan RouteStreamChunk)
	go func() {
		start := time.Now()
		recorded := false
		recordSnapshot := func() LatencySnapshot {
			if !recorded {
				r.metrics.Record(time.Since(start))
				recorded = true
			}
			return r.metrics.Snapshot()
		}
		defer func() {
			if !recorded {
				r.metrics.Record(time.Since(start))
			}
		}()
		defer close(out)

		var errs []error
		for _, service := range SelectStrategy(r.strategy, r.services) {
			stream, err := service.Provider.ChatStream(ctx, service.Config, question)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", service.Provider.Name(), err))
				if ctx.Err() != nil {
					sendRouteStreamErr(ctx, out, ctx.Err())
					return
				}
				continue
			}

			var content strings.Builder
			hasContent := false
			for chunk := range stream {
				if chunk.Err != nil {
					wrapped := fmt.Errorf("%s: %w", service.Provider.Name(), chunk.Err)
					if !hasContent {
						errs = append(errs, wrapped)
						break
					}
					sendRouteStreamErr(ctx, out, wrapped)
					return
				}
				if chunk.Content == "" {
					continue
				}

				hasContent = true
				content.WriteString(chunk.Content)
				if !sendRouteStreamChunk(ctx, out, RouteStreamChunk{
					Provider: service.Provider.Name(),
					Model:    service.Config.Model,
					Content:  chunk.Content,
				}) {
					return
				}
			}

			if hasContent {
				resp := buildStreamResponse(question, content.String())
				if !sendRouteStreamChunk(ctx, out, RouteStreamChunk{
					Provider:   service.Provider.Name(),
					Model:      service.Config.Model,
					Done:       true,
					Response:   resp,
					Cost:       estimateCost(service.Config, resp),
					LastErrors: errs,
					Latency:    recordSnapshot(),
				}) {
					return
				}
				return
			}
		}

		sendRouteStreamErr(ctx, out, fmt.Errorf("all providers failed: %s", joinErrors(errs)))
	}()

	return out, nil
}

// 预估成本
func estimateCost(cfg LLMConfig, resp *ChatResponse) float64 {
	if resp == nil {
		return 0
	}

	inputCost := float64(resp.InputTokens) / 1_000_000 * cfg.InputPricePerMillion
	outputCost := float64(resp.OutputTokens) / 1_000_000 * cfg.OutputPricePerMillion
	return inputCost + outputCost
}

func buildStreamResponse(question string, answer string) *ChatResponse {
	return &ChatResponse{
		Content:      answer,
		InputTokens:  estimateTokenCount(question),
		OutputTokens: estimateTokenCount(answer),
	}
}

func estimateTokenCount(text string) int {
	runes := len([]rune(strings.TrimSpace(text)))
	if runes == 0 {
		return 0
	}

	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens == 0 {
		return 1
	}
	return tokens
}

func sendRouteStreamChunk(ctx context.Context, out chan<- RouteStreamChunk, chunk RouteStreamChunk) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendRouteStreamErr(ctx context.Context, out chan<- RouteStreamChunk, err error) {
	sendRouteStreamChunk(ctx, out, RouteStreamChunk{Err: err})
}

// 合并 error
func joinErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
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

	var errs []routeError
	for _, service := range SelectStrategy(r.strategy, r.services) {
		resp, err := service.Provider.Chat(ctx, service.Config, question)
		if err == nil {
			return &RouteResult{
				Provider:   service.Provider.Name(),
				Model:      service.Config.Model,
				Response:   resp,
				Cost:       estimateCost(service.Config, resp),
				LastErrors: routeErrorsAsErrors(errs),
				Latency:    recordSnapshot(),
			}, nil
		}

		errs = append(errs, routeError{Service: service, Err: err})
		if ctx.Err() != nil {
			recordSnapshot()
			return nil, ctx.Err()
		}
	}

	recordSnapshot()
	return nil, fmt.Errorf("all providers failed:\n%s", formatRouteErrors(errs))
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

		var errs []routeError
		for _, service := range SelectStrategy(r.strategy, r.services) {
			stream, err := service.Provider.ChatStream(ctx, service.Config, question)
			if err != nil {
				errs = append(errs, routeError{Service: service, Err: err})
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
					if !hasContent {
						errs = append(errs, routeError{Service: service, Err: chunk.Err})
						break
					}
					sendRouteStreamErr(ctx, out, fmt.Errorf("%s: %w", service.Provider.Name(), chunk.Err))
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
					LastErrors: routeErrorsAsErrors(errs),
					Latency:    recordSnapshot(),
				}) {
					return
				}
				return
			}
		}

		sendRouteStreamErr(ctx, out, fmt.Errorf("all providers failed:\n%s", formatRouteErrors(errs)))
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

type routeError struct {
	Service LLMService
	Err     error
}

// 合并 error
func formatRouteErrors(errs []routeError) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		category, suggestion := diagnoseError(err.Err)
		parts = append(parts, fmt.Sprintf(
			"- provider=%s model=%s base_url=%s category=%s error=%q 建议: %s",
			err.Service.Provider.Name(),
			err.Service.Config.Model,
			err.Service.Config.BaseURL,
			category,
			err.Err.Error(),
			suggestion,
		))
	}
	return strings.Join(parts, "\n")
}

func routeErrorsAsErrors(errs []routeError) []error {
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		out = append(out, fmt.Errorf("%s: %w", err.Service.Provider.Name(), err.Err))
	}
	return out
}

func diagnoseError(err error) (string, string) {
	if err == nil {
		return "unknown", "查看 provider 配置和上游服务日志"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", "检查请求超时、网络连通性、代理配置和服务商状态"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled", "请求已被取消，检查是否按下 Ctrl+C 或上游 context 被取消"
	}
	if errors.Is(err, io.EOF) || strings.Contains(strings.ToLower(err.Error()), "eof") {
		return "connection_closed", "检查网络、代理、网关地址；远端可能在返回 HTTP 响应前关闭了连接"
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var netErr net.Error
		if errors.As(urlErr, &netErr) && netErr.Timeout() {
			return "timeout", "检查请求超时、网络连通性、代理配置和服务商状态"
		}
		return "network", "检查 base_url 是否正确、DNS/代理是否可用、目标服务是否可访问"
	}

	if strings.Contains(err.Error(), "status=401") || strings.Contains(err.Error(), "status=403") {
		return "auth", "检查 API Key 是否正确、是否有模型权限"
	}
	if strings.Contains(err.Error(), "status=404") {
		return "not_found", "检查 base_url、接口路径和模型名称是否正确"
	}
	if strings.Contains(err.Error(), "status=429") {
		return "rate_limited", "检查限流、额度和并发请求数量"
	}
	if strings.Contains(err.Error(), "status=5") {
		return "provider_5xx", "服务商返回 5xx，稍后重试或切换 provider"
	}

	return "unknown", "查看原始错误、provider 配置和上游服务状态"
}

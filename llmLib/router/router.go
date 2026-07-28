package router

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/provider"
)

type Strategy int

const (
	StrategyDefault Strategy = iota
	StrategyCheapestFirst
	StrategyLowestLatency
)

func (s Strategy) String() string {
	switch s {
	case StrategyCheapestFirst:
		return "cheapest"
	case StrategyLowestLatency:
		return "latency"
	default:
		return "default"
	}
}

type LLMService struct {
	Provider provider.Provider
	Config   core.LLMConfig
}

type RouteResult struct {
	Provider   string
	Model      string
	Response   *core.ChatResponse
	Cost       float64
	Latency    LatencySnapshot
	LastErrors []error
}

type RouteStreamChunk struct {
	Provider string
	Model    string
	Chunk    *core.StreamChunk
	Err      error
}

type LatencySnapshot struct {
	Samples int
	P50     time.Duration
	P95     time.Duration
}

type LatencyMetrics struct {
	records []time.Duration
}

func NewLatencyMetrics() *LatencyMetrics {
	return &LatencyMetrics{}
}

func (m *LatencyMetrics) Record(d time.Duration) {
	m.records = append(m.records, d)
}

func (m *LatencyMetrics) Snapshot() LatencySnapshot {
	if len(m.records) == 0 {
		return LatencySnapshot{}
	}
	sorted := make([]time.Duration, len(m.records))
	copy(sorted, m.records)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	p50Idx := len(sorted) * 50 / 100
	p95Idx := len(sorted) * 95 / 100
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}

	return LatencySnapshot{
		Samples: len(sorted),
		P50:     sorted[p50Idx],
		P95:     sorted[p95Idx],
	}
}

type Router struct {
	services []LLMService
	strategy Strategy
	metrics  *LatencyMetrics
}

func NewRouter(services []LLMService, strategy Strategy) *Router {
	return &Router{
		services: services,
		strategy: strategy,
		metrics:  NewLatencyMetrics(),
	}
}

func (r *Router) Chat(ctx context.Context, messages []core.Message) (*RouteResult, error) {
	if len(r.services) == 0 {
		return nil, fmt.Errorf("router: no services available")
	}

	ordered := r.sortServices()
	var lastErr error

	for _, svc := range ordered {
		start := time.Now()
		resp, err := svc.Provider.Chat(ctx, svc.Config, messages)
		elapsed := time.Since(start)
		r.metrics.Record(elapsed)

		if err != nil {
			lastErr = err
			continue
		}

		cost := estimateCost(resp, svc.Config)

		return &RouteResult{
			Provider: svc.Provider.Name(),
			Model:    svc.Config.Model,
			Response: resp,
			Cost:     cost,
			Latency:  r.metrics.Snapshot(),
		}, nil
	}

	return &RouteResult{
		LastErrors: []error{lastErr},
	}, lastErr
}

func (r *Router) sortServices() []LLMService {
	ordered := make([]LLMService, len(r.services))
	copy(ordered, r.services)

	switch r.strategy {
	case StrategyCheapestFirst:
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].Config.InputPricePerMillion+ordered[i].Config.OutputPricePerMillion <
				ordered[j].Config.InputPricePerMillion+ordered[j].Config.OutputPricePerMillion
		})
	case StrategyLowestLatency:
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].Config.LatencyMS < ordered[j].Config.LatencyMS
		})
	}
	return ordered
}

func estimateCost(resp *core.ChatResponse, cfg core.LLMConfig) float64 {
	inputCost := float64(resp.InputTokens) / 1_000_000 * cfg.InputPricePerMillion
	outputCost := float64(resp.OutputTokens) / 1_000_000 * cfg.OutputPricePerMillion
	return inputCost + outputCost
}

type RouterAdapter struct {
	router *Router
}

func NewRouterAdapter(r *Router) *RouterAdapter {
	return &RouterAdapter{router: r}
}

func (a *RouterAdapter) Name() string { return "router-adapter" }

func (a *RouterAdapter) Chat(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (*core.ChatResponse, error) {
	result, err := a.router.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (a *RouterAdapter) ChatStream(ctx context.Context, cfg core.LLMConfig, messages []core.Message) (<-chan core.StreamChunk, error) {
	return nil, fmt.Errorf("router adapter: stream not supported")
}

// 文件职责：
// - 提供多服务商路由、失败切换、策略排序和延迟统计能力。
// - 负责在多个已配置 provider 之间选择调用顺序，并汇总最终结果与诊断信息。

package llmlib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Strategy 表示 Router 选择服务商尝试顺序时使用的调度策略。
type Strategy string

const (
	StrategyDefault       Strategy = "default"        // 默认策略，按服务列表原始顺序依次尝试。
	StrategyCheapestFirst Strategy = "cheapest_first" // 成本优先策略，按静态价格从低到高排序。
	StrategyLowestLatency Strategy = "lowest_latency" // 延迟优先策略，按静态延迟从低到高排序。
)

// LLMService 绑定一个 provider 实现及其对应配置，作为 Router 的候选节点。
type LLMService struct {
	Provider Provider  // 服务商实现，负责真正发起请求。
	Config   LLMConfig // 当前服务商的模型、价格和连接配置。
}

// RouteResult 表示同步路由调用的最终结果，以及前序失败节点的信息。
type RouteResult struct {
	Provider   string          // 实际成功返回结果的服务商名称。
	Model      string          // 实际使用的模型名称，来自命中的服务配置。
	Response   *ChatResponse   // 模型返回的统一响应数据。
	Cost       float64         // 按静态单价和 token 用量估算的调用成本。
	LastErrors []error         // 在命中成功前失败过的服务错误列表。
	Latency    LatencySnapshot // 当前 Router 累积延迟样本快照。
}

// RouteStreamChunk 表示流式路由中的单次输出事件，兼容中间文本和最终收尾信息。
type RouteStreamChunk struct {
	Provider   string          // 当前输出所属的服务商名称。
	Model      string          // 当前输出所属的模型名称。
	Content    string          // 增量文本内容，流式中间事件时非空。
	Err        error           // 流式过程中的错误事件。
	Done       bool            // 是否为流式成功结束后的收尾事件。
	Response   *ChatResponse   // Done 事件携带的完整响应汇总。
	Cost       float64         // Done 事件对应的估算成本。
	LastErrors []error         // 成功前失败过的服务错误列表。
	Latency    LatencySnapshot // Done 事件对应的延迟统计快照。
}

// Router 维护候选服务列表、调度策略和运行期延迟统计。
type Router struct {
	services []LLMService
	strategy Strategy
	metrics  *LatencyMetrics
}

// NewRouter 创建一个可在多个服务商之间切换的路由器实例。
func NewRouter(services []LLMService, strategy Strategy) *Router {
	return &Router{
		services: services,
		strategy: strategy,
		metrics:  NewLatencyMetrics(),
	}
}

// Chat 按当前策略依次尝试各服务商，直到某个节点成功返回或全部失败。
func (r *Router) Chat(ctx context.Context, messages []Message) (*RouteResult, error) {
	start := time.Now()
	recordSnapshot := func() LatencySnapshot {
		r.metrics.Record(time.Since(start))
		return r.metrics.Snapshot()
	}

	if len(r.services) == 0 {
		recordSnapshot()
		return nil, errors.New("router has no services")
	}

	var errs []routeError
	for _, service := range selectStrategy(r.strategy, r.services) {
		// 按排序后的顺序逐个尝试 provider，命中成功后立即返回。
		resp, err := service.Provider.Chat(ctx, service.Config, messages)
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
			// 上游 context 已取消时优先返回取消错误，不再继续尝试其他 provider。
			recordSnapshot()
			return nil, ctx.Err()
		}
	}

	recordSnapshot()
	return nil, fmt.Errorf("all providers failed:\n%s", formatRouteErrors(errs))
}

// ChatStream 按当前策略依次尝试流式服务，命中首个有效输出后继续消费至结束。
func (r *Router) ChatStream(ctx context.Context, messages []Message) (<-chan RouteStreamChunk, error) {
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
		for _, service := range selectStrategy(r.strategy, r.services) {
			// 先尝试建立当前 provider 的流式连接，连接失败则切换到下一个节点。
			stream, err := service.Provider.ChatStream(ctx, service.Config, messages)
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
					// 尚未产出任何文本时，将其视为当前 provider 建流失败并继续切换。
					if !hasContent {
						errs = append(errs, routeError{Service: service, Err: chunk.Err})
						break
					}
					// 已开始向调用方输出内容后不再切换 provider，直接把错误透出。
					sendRouteStreamErr(ctx, out, fmt.Errorf("%s: %w", service.Provider.Name(), chunk.Err))
					return
				}
				if chunk.Content == "" {
					continue
				}

				hasContent = true
				content.WriteString(chunk.Content)
				// 将上游文本增量原样转发给调用方消费。
				if !sendRouteStreamChunk(ctx, out, RouteStreamChunk{
					Provider: service.Provider.Name(),
					Model:    service.Config.Model,
					Content:  chunk.Content,
				}) {
					return
				}
			}

			if hasContent {
				var inputTokens, outputTokens int
				for _, msg := range messages {
					inputTokens += estimateTokens(msg.Content)
				}
				outputTokens = estimateTokens(content.String())
				resp := &ChatResponse{
					Content:      content.String(),
					InputTokens:  inputTokens,
					OutputTokens: outputTokens,
				}
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

// estimateCost 根据配置的静态单价和 token 数估算一次调用成本。
func estimateCost(cfg LLMConfig, resp *ChatResponse) float64 {
	if resp == nil {
		return 0
	}
	inputCost := float64(resp.InputTokens) / 1_000_000 * cfg.InputPricePerMillion
	outputCost := float64(resp.OutputTokens) / 1_000_000 * cfg.OutputPricePerMillion
	return inputCost + outputCost
}

// selectStrategy 根据策略返回新的服务顺序，避免修改原始切片。
func selectStrategy(strategy Strategy, services []LLMService) []LLMService {
	switch strategy {
	case StrategyCheapestFirst:
		return strategyCheapestFirst(services)
	case StrategyLowestLatency:
		return strategyLowestLatency(services)
	default:
		return strategyDefault(services)
	}
}

// strategyDefault 复制原始顺序，作为默认尝试列表。
func strategyDefault(services []LLMService) []LLMService {
	return append([]LLMService(nil), services...)
}

// strategyCheapestFirst 按输入输出单价总和从低到高排列服务。
func strategyCheapestFirst(services []LLMService) []LLMService {
	ordered := strategyDefault(services)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].Config.InputPricePerMillion + ordered[i].Config.OutputPricePerMillion
		right := ordered[j].Config.InputPricePerMillion + ordered[j].Config.OutputPricePerMillion
		return left < right
	})
	return ordered
}

// strategyLowestLatency 按静态延迟指标从低到高排列服务。
func strategyLowestLatency(services []LLMService) []LLMService {
	ordered := strategyDefault(services)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Config.LatencyMS < ordered[j].Config.LatencyMS
	})
	return ordered
}

// sendRouteStreamChunk 向下游发送流式事件，并在上下文取消时终止发送。
func sendRouteStreamChunk(ctx context.Context, out chan<- RouteStreamChunk, chunk RouteStreamChunk) bool {
	select {
	case out <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}

// sendRouteStreamErr 把错误包装为流式事件发送给调用方。
func sendRouteStreamErr(ctx context.Context, out chan<- RouteStreamChunk, err error) {
	sendRouteStreamChunk(ctx, out, RouteStreamChunk{Err: err})
}

// routeError 记录单个服务节点失败时的配置和原始错误。
type routeError struct {
	Service LLMService // 失败的服务节点。
	Err     error      // 该节点返回的原始错误。
}

// formatRouteErrors 把各 provider 的失败信息和诊断建议拼成多行文本。
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

// routeErrorsAsErrors 将内部错误结构转换为对外暴露的 error 列表。
func routeErrorsAsErrors(errs []routeError) []error {
	out := make([]error, 0, len(errs))
	for _, err := range errs {
		out = append(out, fmt.Errorf("%s: %w", err.Service.Provider.Name(), err.Err))
	}
	return out
}

// diagnoseError 按常见网络和鉴权场景归类错误，并返回排查建议。
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

// LatencySnapshot 表示 Router 当前累计请求延迟的统计快照。
type LatencySnapshot struct {
	Samples int           // 总采样次数，用于表示统计样本规模。
	P50     time.Duration // 50 分位延迟，即中位数。
	P95     time.Duration // 95 分位延迟，用于观察尾部慢请求。
}

// LatencyMetrics 负责线程安全地累积延迟样本并生成分位统计。
type LatencyMetrics struct {
	mu      sync.Mutex
	samples []time.Duration
}

// NewLatencyMetrics 创建空的延迟统计容器。
func NewLatencyMetrics() *LatencyMetrics {
	return &LatencyMetrics{}
}

// Record 记录一次延迟样本，负值会被直接忽略。
func (m *LatencyMetrics) Record(latency time.Duration) {
	if latency < 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, latency)
}

// Snapshot 复制当前样本并计算常用分位值，供路由结果对外展示。
func (m *LatencyMetrics) Snapshot() LatencySnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.samples) == 0 {
		return LatencySnapshot{}
	}

	samples := append([]time.Duration(nil), m.samples...)
	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	return LatencySnapshot{
		Samples: len(samples),
		P50:     percentile(samples, 0.50),
		P95:     percentile(samples, 0.95),
	}
}

// percentile 在已排序样本中按给定分位点取值。
func percentile(samples []time.Duration, p float64) time.Duration {
	index := int(math.Ceil(float64(len(samples))*p)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(samples) {
		index = len(samples) - 1
	}
	return samples[index]
}

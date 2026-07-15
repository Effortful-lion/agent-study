// 文件职责：
// - 封装对大模型 HTTP API 的请求发送能力，向上层 ai.Provider 隐藏 HTTP 细节。
// - 这里集中管理连接池、单次请求生命周期、并发限流、非流式请求重试和 SSE 流式读取。
// - 调用方必须通过 context 控制每次请求的超时或取消，http.Client 本身不设置全局 Timeout。

package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultDialTimeout 控制 TCP 建连阶段的最长等待时间，避免目标地址不可达时长期阻塞。
	defaultDialTimeout = 10 * time.Second

	// defaultKeepAlive 控制 TCP keep-alive 探测间隔，帮助复用长连接并及时发现失效连接。
	defaultKeepAlive = 30 * time.Second

	// defaultMaxIdleConns 是全局空闲连接池上限，支撑多请求场景下复用连接。
	defaultMaxIdleConns = 100

	// defaultMaxIdleConnsPerHost 是单个上游 host 的空闲连接池上限，避免高并发时频繁 TCP/TLS 握手。
	defaultMaxIdleConnsPerHost = 20

	// defaultIdleConnTimeout 控制空闲连接保留时间，超时后由 Transport 回收。
	defaultIdleConnTimeout = 90 * time.Second

	// defaultTLSHandshakeTimeout 控制 TLS 握手阶段超时，避免 HTTPS 握手异常时卡住请求。
	defaultTLSHandshakeTimeout = 10 * time.Second

	// defaultMaxConcurrentRequests 是客户端侧并发闸门，防止瞬时请求量压垮本进程或上游服务。
	defaultMaxConcurrentRequests = 64

	// defaultMaxRetries 是额外重试次数；总请求次数为 1 + defaultMaxRetries。
	defaultMaxRetries = 2

	// defaultRetryBaseDelay 是指数退避的初始等待时间。
	defaultRetryBaseDelay = 50 * time.Millisecond

	// defaultRetryMaxDelay 是单次退避等待的上限，避免连续失败时等待时间无限增长。
	defaultRetryMaxDelay = 2 * time.Second
)

// Client 是传输层客户端，负责把 JSON/SSE 调用统一封装为可复用的 HTTP 能力。
type Client struct {
	httpClient *http.Client // 底层 HTTP 执行器，只配置 Transport，不设置全局 Timeout。
	baseURL    string       // 上游 API 根地址，创建时去掉末尾斜杠，调用时拼接 path。
}

// NewClient 创建生产级 HTTP 客户端，调用方继续通过 context 管理每次请求的生命周期。
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: NewHTTPClient(),
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// NewHTTPClient 返回一个普通 *http.Client，但它的 Transport 已经内置限流和重试。
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &retryTransport{
			base:    newTransport(),
			retry:   defaultRetryConfig(),
			limiter: newRequestLimiter(defaultMaxConcurrentRequests),
		},
	}
}

// newTransport 创建可复用连接池的 Transport，集中配置 TCP/TLS 建连和空闲连接回收策略。
func newTransport() *http.Transport {
	// Dialer 只负责 TCP 层建连和 keep-alive；请求整体超时由外层 context 控制。
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: defaultKeepAlive,
	}

	// http.Transport 内置连接池；只要复用同一个实例，就能复用 TCP/TLS 连接。
	return &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// retryConfig 描述重试策略，字段保持简单，避免调用方理解复杂概念。
type retryConfig struct {
	maxRetries int           // 额外重试次数；总请求次数为 1 + maxRetries。
	baseDelay  time.Duration // 指数退避的初始等待时间。
	maxDelay   time.Duration // 单次退避等待的上限。
}

// defaultRetryConfig 返回当前客户端的默认重试策略。
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxRetries: defaultMaxRetries,
		baseDelay:  defaultRetryBaseDelay,
		maxDelay:   defaultRetryMaxDelay,
	}
}

// requestLimiter 是很薄的并发限流器，用 channel 控制同时进入底层 Transport 的请求数。
type requestLimiter struct {
	tokens chan struct{} // 每个 token 代表一个可执行请求的并发名额。
}

// newRequestLimiter 创建固定并发上限的请求限流器。
func newRequestLimiter(maxConcurrent int) *requestLimiter {
	return &requestLimiter{tokens: make(chan struct{}, maxConcurrent)}
}

// wait 获取一个并发名额；如果请求 context 已取消，则立即返回 context 错误。
func (l *requestLimiter) wait(ctx context.Context) error {
	select {
	case l.tokens <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// done 归还并发名额，必须和 wait 成对出现。
func (l *requestLimiter) done() {
	<-l.tokens
}

// retryTransport 是一层 http.RoundTripper 中间件，把限流和重试变成 HTTP 执行链路的一部分。
type retryTransport struct {
	base    http.RoundTripper // 真正负责发请求的底层 Transport，通常是 *http.Transport。
	retry   retryConfig       // 非流式和流式请求在拿到最终响应前共享的重试策略。
	limiter *requestLimiter   // 可为空；为空时只重试不限流。
}

// RoundTrip 先做 context 可取消的限流，再把请求交给底层 Transport，并对临时失败做有限重试。
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if t.limiter != nil {
		if err := t.limiter.wait(ctx); err != nil {
			return nil, err
		}
		defer t.limiter.done()
	}

	if req.Body != nil && req.GetBody != nil {
		// RoundTripper 需要负责关闭传入的 Body；重试时每轮使用 GetBody 生成独立副本。
		defer req.Body.Close()
	}

	var lastErr error
	for attempt := 0; attempt <= t.retry.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		nextReq, err := cloneRequestForAttempt(req, attempt)
		if err != nil {
			return nil, err
		}

		resp, err := t.base.RoundTrip(nextReq)
		if err == nil && !shouldRetry(resp.StatusCode) {
			return resp, nil
		}

		if err != nil {
			if !canRetryRequestBody(req) {
				return nil, err
			}
			lastErr = err
		} else {
			if !canRetryRequestBody(req) {
				return resp, nil
			}
			lastErr = fmt.Errorf("服务端返回 %d", resp.StatusCode)
			closeResponseForRetry(resp)
		}

		if attempt == t.retry.maxRetries {
			break
		}
		if err := sleepBackoff(ctx, attempt, t.retry); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", t.retry.maxRetries, lastErr)
}

// StreamJSON 发起 SSE 流式 JSON 请求，并把每个 data 事件交给 onData 逐条处理。
func (c *Client) StreamJSON(ctx context.Context, path string, headers map[string]string, payload any, onData func(string) error) error {
	// 请求体只序列化一次；流式请求不做重试，避免上游已输出部分内容后重复消费。
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// 每次请求都绑定调用方传入的 context，用它承接超时、取消和链路退出。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 限流和重试已经隐藏在 http.Client.Transport 中，这里按普通 HTTP 请求使用即可。
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	// 从网络 IO 按行扫描 SSE 数据，scanner 会随着响应流持续阻塞读取。
	scanner := bufio.NewScanner(resp.Body)
	/*
	  SSE 一个事件可以有多行 data:：
	  data: line1
	  data: line2

	  协议上这应该合并成：
	  line1
	  line2

	  所以这里用了：
	  data := strings.Join(lines, "\n")

	  虽然大模型 API 一般一条事件只有一行 data:，但这样写更符合 SSE 协议。
	*/
	var eventData []string
	for scanner.Scan() {
		// scanner.Scan 正常读到一行后再检查 context，确保调用方取消时尽快退出消费循环。
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		// 空行表示一个 SSE event 结束，此时把之前收集到的 data 行合并后交给回调。
		if line == "" {
			if len(eventData) == 0 {
				continue
			}
			data := strings.Join(eventData, "\n")
			err := onData(data)
			if err != nil {
				return err
			}
			eventData = nil
			continue
		}
		if strings.HasPrefix(line, "data:") {
			// 只提取 data: 行，忽略 event/id/retry 等本客户端暂不关心的 SSE 字段。
			eventData = append(eventData, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		// 读取响应体失败时优先返回 context 错误，让调用方能区分主动取消和网络异常。
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	if len(eventData) == 0 {
		return nil
	}
	// 响应流最后没有空行时，仍把残留 data 作为最后一个事件交给调用方。
	data := strings.Join(eventData, "\n")
	return onData(data)
}

// PostJSON 发起普通 JSON 请求，并把成功响应解析到 out；重试和限流由 http.Client.Transport 统一处理。
func (c *Client) PostJSON(ctx context.Context, path string, headers map[string]string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 调用方只看到一次普通 Do；失败重试、退避等待和并发限流都由 retryTransport 完成。
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

// cloneRequestForAttempt 为每次请求尝试准备独立 Request，避免 RoundTripper 修改原始 req。
func cloneRequestForAttempt(req *http.Request, attempt int) (*http.Request, error) {
	if req.Body == nil {
		return req.Clone(req.Context()), nil
	}

	if req.GetBody == nil {
		if attempt == 0 {
			return req, nil
		}
		return nil, fmt.Errorf("请求体不支持重试")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	cloned.Body = body
	return cloned, nil
}

// canRetryRequestBody 判断请求体是否能为下一次重试重新打开。
func canRetryRequestBody(req *http.Request) bool {
	return req.Body == nil || req.GetBody != nil
}

// closeResponseForRetry 关闭可重试响应，并尽量读完 Body 以便底层连接回到连接池。
func closeResponseForRetry(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// shouldRetry 判断响应状态是否属于可重试的临时性失败。
func shouldRetry(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

// sleepBackoff 按指数退避加随机抖动等待，并在等待期间响应 context 取消。
func sleepBackoff(ctx context.Context, attempt int, retry retryConfig) error {
	// 退避时间随 attempt 翻倍，减少高并发失败时对上游的同步重压。
	delay := retry.baseDelay << attempt
	if delay > retry.maxDelay {
		delay = retry.maxDelay
	}
	// 抖动让不同请求的重试时间错开，降低重试风暴概率。
	delay += time.Duration(rand.Int63n(int64(delay / 2)))

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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
	httpClient *http.Client  // 底层 HTTP 执行器，只配置 Transport，不设置全局 Timeout。
	baseURL    string        // 上游 API 根地址，创建时去掉末尾斜杠，调用时拼接 path。
	limiter    chan struct{} // 并发令牌桶，每次请求发送前占用一个令牌，结束后归还。
}

// NewClient 创建生产级 HTTP 客户端，调用方继续通过 context 管理每次请求的生命周期。
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Transport: newTransport()},
		baseURL:    strings.TrimRight(baseURL, "/"),
		limiter:    make(chan struct{}, defaultMaxConcurrentRequests),
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

	// c.do 统一经过并发限流；底层 Transport 负责复用连接池中的空闲连接。
	resp, err := c.do(req)
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

// PostJSON 发起普通 JSON 请求，针对临时性失败做有限重试并把成功响应解析到 out。
func (c *Client) PostJSON(ctx context.Context, path string, headers map[string]string, payload any, out any) error {
	// body 必须提前序列化成 []byte，保证每次重试都能重新创建可读取的请求体。
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt <= defaultMaxRetries; attempt++ {
		// 每轮开始前先检查 context，避免已取消时继续排队、建连或等待退避。
		if err := ctx.Err(); err != nil {
			return err
		}

		// 每次重试都新建 Request，避免复用已被 http.Client 消费过的 Body。
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.do(req)
		if err != nil {
			// 如果错误来自调用方取消或超时，直接返回 context 错误，不再重试。
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// 其他网络错误按临时失败处理，等待指数退避后继续尝试。
			lastErr = err
			if attempt == defaultMaxRetries {
				return lastErr
			}
			if err := sleepBackoff(ctx, attempt); err != nil {
				return err
			}
			continue
		}

		if shouldRetry(resp.StatusCode) {
			// 429/5xx 通常代表限流或服务端临时异常，适合做有限重试。
			lastErr = fmt.Errorf("unexpected status code: %s", resp.Status)
			// 重试前必须读完并关闭响应体，Transport 才有机会复用底层连接。
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if attempt == defaultMaxRetries {
				return lastErr
			}
			if err := sleepBackoff(ctx, attempt); err != nil {
				return err
			}
			continue
		}
		defer resp.Body.Close()

		// 非重试状态直接返回给调用方，避免把 4xx 这类业务/参数错误伪装成临时失败。
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %s", resp.Status)
		}

		// 成功响应由调用方提供 out 承接，transport 层不关心具体业务结构。
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
		return nil
	}

	return lastErr
}

// do 统一执行 HTTP 请求，并在进入 http.Client.Do 前做 context 可取消的并发限流。
func (c *Client) do(req *http.Request) (*http.Response, error) {
	select {
	case c.limiter <- struct{}{}:
		// 请求结束后归还令牌，保证后续请求可以继续进入连接池执行。
		defer func() { <-c.limiter }()
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	// 真正的 HTTP 执行交给标准库；连接复用、建连超时和 TLS 超时由 Transport 处理。
	return c.httpClient.Do(req)
}

// shouldRetry 判断响应状态是否属于可重试的临时性失败。
func shouldRetry(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

// sleepBackoff 按指数退避加随机抖动等待，并在等待期间响应 context 取消。
func sleepBackoff(ctx context.Context, attempt int) error {
	// 退避时间随 attempt 翻倍，减少高并发失败时对上游的同步重压。
	delay := defaultRetryBaseDelay << attempt
	if delay > defaultRetryMaxDelay {
		delay = defaultRetryMaxDelay
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

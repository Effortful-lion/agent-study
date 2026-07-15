package transport

import (
	"fmt"
	"io"
	"net/http"
)

const maxRetries = 2

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			// 内置 retry 2次：用 transport.NewClient().Do(req) 发送请求，并只对 429/5xx 重试
			Transport: retryTransport{base: http.DefaultTransport},
		},
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

type retryTransport struct {
	base http.RoundTripper
}

// RoundTrip 实现 transport 接口：RoundTripper
func (t retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// for 循环前 提前判断 ctx 是否取消
		if err := req.Context().Err(); err != nil {
			return nil, err
		}

		nextReq, err := requestForAttempt(req, attempt)
		if err != nil {
			return nil, err
		}

		resp, err := base.RoundTrip(nextReq)
		if err != nil {
			return nil, err
		}
		if !shouldRetry(resp.StatusCode) || attempt == maxRetries {
			return resp, nil
		}
		closeBody(resp)
	}

	return nil, fmt.Errorf("unreachable retry state")
}

func requestForAttempt(req *http.Request, attempt int) (*http.Request, error) {
	if req.Body == nil {
		return req.Clone(req.Context()), nil
	}
	if attempt == 0 {
		return req, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("request body cannot be retried")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	cloned.Body = body
	return cloned, nil
}

func shouldRetry(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

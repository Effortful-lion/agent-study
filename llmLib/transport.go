package llmlib

import (
	"crypto/tls"
	"net/http"
	"time"
)

// ClientOption 客户端选项，用于配置 HTTP 客户端
type ClientOption func(*http.Client)

// NewClient 创建一个新的 HTTP 客户端，默认超时时间为 30 秒
func NewClient(opts ...ClientOption) *http.Client {
	c := &http.Client{
		Timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithTimeout 设置 HTTP 客户端的超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *http.Client) {
		c.Timeout = timeout
	}
}

// WithTLSConfig 设置 HTTP 客户端的 TLS 配置
func WithTLSConfig(cfg *tls.Config) ClientOption {
	return func(c *http.Client) {
		if c.Transport == nil {
			c.Transport = &http.Transport{}
		}
		if t, ok := c.Transport.(*http.Transport); ok {
			t.TLSClientConfig = cfg
		}
	}
}

// WithTransport 设置 HTTP 客户端的 Transport
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *http.Client) {
		c.Transport = transport
	}
}

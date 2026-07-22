// 文件职责：
// - 封装可复用的 HTTP 客户端创建和传输层配置选项。
// - 供各协议适配器在发起请求时统一使用默认超时与自定义 transport。

package llmlib

import (
	"crypto/tls"
	"net/http"
	"time"
)

// ClientOption 表示对 HTTP 客户端的单项配置函数。
type ClientOption func(*http.Client)

// NewClient 创建一个带默认超时的 HTTP 客户端，并按顺序应用额外选项。
func NewClient(opts ...ClientOption) *http.Client {
	c := &http.Client{
		Timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithTimeout 覆盖客户端整体请求超时时间。
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *http.Client) {
		c.Timeout = timeout
	}
}

// WithTLSConfig 为客户端的 Transport 注入 TLS 配置。
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

// WithTransport 直接替换客户端使用的 RoundTripper。
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *http.Client) {
		c.Transport = transport
	}
}

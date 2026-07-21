package llmlib

import (
	"crypto/tls"
	"net/http"
	"time"
)

type ClientOption func(*http.Client)

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *http.Client) {
		c.Timeout = timeout
	}
}

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

func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *http.Client) {
		c.Transport = transport
	}
}

func NewClient(opts ...ClientOption) *http.Client {
	c := &http.Client{
		Timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

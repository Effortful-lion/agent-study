package main

import "net/http"

// 无重试 Client
type Client struct {
	httpClient *http.Client
}

// 无重试 Client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

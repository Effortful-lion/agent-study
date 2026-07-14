package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// 传输层客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 2 * time.Minute},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (c *Client) StreamJSON(ctx context.Context, path string, headers map[string]string, payload any, onData func(string) error) error {
	// body
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// req
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// resp
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	// 从网络io scan数据
	scanner := bufio.NewScanner(resp.Body)
	// 一个 sse 事件可以有多行 data：
	/*
	  因为 SSE 一个事件可以有多行 data:：
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
		// TODO 这里思考需要加 c.Done() 监听吗？
		// 我这里进行context的直接检查
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		// 注意每个事件之间有一个空行。
		// 所以空行表示一个SSE event结束，表示可以发送 data: 行了
		if line == "" {
			if len(eventData) == 0 {
				return nil
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
			eventData = append(eventData, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(eventData) == 0 {
		return nil
	}
	data := strings.Join(eventData, "\n")
	return onData(data)
}

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

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

/*
[]基础实现：开发openai.Provider，封装普通对话Chat、流式对话ChatStream两个核心方法；
[]异构适配：开发claude.Provider，内部完成请求 / 响应协议转换，对外对齐通用Provider接口；
[]兼容厂商快速接入：对接豆包 / DeepSeek，仅修改基础地址、密钥、模型名称，复用 OpenAI 实现；
[]配置与工厂封装：实现BuildAll工厂函数，通过环境变量批量加载、初始化所有模型服务商；
[]路由故障转移：开发Router调度器，支持模型调用失败自动切换备用服务商，最终输出：服务商标识、token 消耗、预估计费成本。
[]调度策略：实现「最便宜优先 CheapestFirst」「最低延迟 LowestLatency」两种路由算法；
[]性能观测：统计全链路 P50、P95 分位延迟指标；
*/

func main() {
	// 用 signal.NotifyContext 贯穿取消
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 从命令行参数拼接用户问题
	question := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, "用法: minicall <你的问题>")
		os.Exit(1)
	}

	cfg, err := loadConfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	//err = Chat(ctx, cfg, question)
	//if err != nil {
	//	fmt.Fprintln(os.Stderr, err)
	//	os.Exit(1)
	//}
	stream, err := ChatStream(ctx, cfg, question)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Fprintln(os.Stderr, chunk.Err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, chunk.Content)
		// 强制刷新输出缓冲区，确保内容立即显示到终端（否则可能因为缓冲导致延迟）
		os.Stdout.Sync()
	}
}

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// 读取 LLM_BASE_URL、LLM_API_KEY、LLM_MODEL
func loadConfigFromEnv() (LLMConfig, error) {
	cfg := LLMConfig{
		BaseURL: strings.TrimRight(os.Getenv("LLM_BASE_URL"), "/"),
		APIKey:  os.Getenv("LLM_API_KEY"),
		Model:   os.Getenv("LLM_MODEL"),
	}
	if cfg.BaseURL == "" {
		return cfg, errors.New("请设置 LLM_BASE_URL，例如: export LLM_BASE_URL=https://api.deepseek.com")
	}
	if cfg.APIKey == "" {
		return cfg, errors.New("请设置 LLM_API_KEY，例如: export LLM_API_KEY=sk-xxx")
	}
	if cfg.Model == "" {
		return cfg, errors.New("请设置 LLM_MODEL，例如: export LLM_MODEL=deepseek-chat")
	}
	return cfg, nil
}

type Role string

const (
	User     Role = "user"
	System   Role = "system"
	Assident Role = "assident"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"` // 承载多轮上下文
	Stream   bool      `json:"stream"`
}

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

type ChatResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// 非流式调用
func Chat(ctx context.Context, cfg LLMConfig, question string) error {
	// body
	chatReq := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: User, Content: question},
		},
		Stream: false,
	}
	body, err := json.Marshal(chatReq)

	// req
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// resp
	client := NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 解析 resp
	chatResp, err := parseResp(resp)
	if err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// 输出 resp
	fmt.Println(chatResp.Content)
	return nil
}

func parseResp(resp *http.Response) (*ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, errors.New("parse response: choices is empty")
	}

	return &ChatResponse{
		Content:      raw.Choices[0].Message.Content,
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}, nil
}

// StreamChunk 流式数据块
type StreamChunk struct {
	Content string
	Err     error
}

// 流式调用
func ChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	stream := make(chan StreamChunk)

	url := cfg.BaseURL + "/chat/completions"
	chatReq := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: User, Content: question},
		},
		Stream: true,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	go func() {
		defer close(stream)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			stream <- StreamChunk{Err: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			stream <- StreamChunk{
				Err: fmt.Errorf("chat stream failed: status=%d body=%s", resp.StatusCode, string(b)),
			}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var raw struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &raw); err != nil {
				stream <- StreamChunk{Err: fmt.Errorf("decode stream chunk: %w", err)}
				return
			}

			var content string
			for _, choice := range raw.Choices {
				content += choice.Delta.Content
			}

			select {
			case stream <- StreamChunk{Content: content}:
			case <-ctx.Done():
				stream <- StreamChunk{Err: ctx.Err()}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			stream <- StreamChunk{Err: err}
		}
	}()

	return stream, nil
}

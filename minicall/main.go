package main

import (
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

	"github.com/Effortful-lion/agent-study/minicall/internal/llm"
	"github.com/Effortful-lion/agent-study/minicall/internal/transport"
)

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

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

	// 用 signal.NotifyContext 贯穿取消
	if err := runOnce(ctx, cfg, question); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "\n已中断")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "出错:", err)
		os.Exit(1)
	}
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

func runOnce(ctx context.Context, cfg LLMConfig, question string) error {
	chatReq := llm.ChatRequest{
		Model: cfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: question},
		},
		Stream: false,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// 用 http.NewRequestWithContext 构造请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// 用 transport.NewClient().Do(req) 发送请求，并只对 429/5xx 重试
	resp, err := transport.NewClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	// 解析 choices[0].message.content 和 usage
	chatResp, err := parseChatResponse(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(chatResp.Content)
	fmt.Printf("tokens: input=%d output=%d\n", chatResp.InputTokens, chatResp.OutputTokens)
	return nil
}

// 解析 choices[0].message.content 和 usage
func parseChatResponse(r io.Reader) (*llm.ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message llm.Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, errors.New("parse response: choices is empty")
	}

	return &llm.ChatResponse{
		Content:      raw.Choices[0].Message.Content,
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}, nil
}

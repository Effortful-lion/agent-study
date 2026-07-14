package ai

import (
	"context"
	"encoding/json"
	"io"

	"github.com/Effortful-lion/agent-study/minicall/internal/llm"
	"github.com/Effortful-lion/agent-study/minicall/internal/transport"
)

type Config struct {
	Model   string
	BaseURL string
	APIKey  string
}

type ChatModel struct {
	cfg       Config
	transport *transport.Client
}

func NewChatModel(cfg Config) *ChatModel {
	return &ChatModel{
		cfg:       cfg,
		transport: transport.NewClient(cfg.BaseURL),
	}
}

// 流式一次性调用
func (c *ChatModel) StreamInvokeChat(ctx context.Context, question string, out io.Writer) error {
	req := llm.ChatRequest{
		Model: c.cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: question},
		},
		Stream: true,
	}

	// 解析 resp 的 data
	// 这里应该知道大模型返回格式（所以是特殊部分，即每个大模型可能不同）
	onData := func(data string) error {
		if data == "[DONE]" {
			return nil
		}
		var resp llm.ChatStreamResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return err
		}
		for _, choice := range resp.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if _, err := io.WriteString(out, choice.Delta.Content); err != nil {
				return err
			}
		}
		return nil
	}

	return c.transport.StreamJSON(ctx, "/chat/completions", map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
	}, req, onData)
}

// 非流式一次性调用
func (c *ChatModel) InvokeChat(ctx context.Context, question string) (*llm.ChatResponse, error) {
	req := llm.ChatRequest{
		Model: c.cfg.Model,
		Messages: []llm.Message{
			{Role: "user", Content: question},
		},
		Stream: false,
	}

	var resp llm.ChatResponse
	if err := c.transport.PostJSON(ctx, "/chat/completions", map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
	}, req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

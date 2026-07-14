package ai

import (
	"context"

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

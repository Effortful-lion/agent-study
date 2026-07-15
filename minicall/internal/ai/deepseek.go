package ai

import (
	"context"

	"github.com/Effortful-lion/agent-study/minicall/internal/llm"
	"github.com/Effortful-lion/agent-study/minicall/internal/transport"
)

type Config struct {
	Name    string
	Model   string
	BaseURL string
	APIKey  string
}

type DeepSeekModel struct {
	cfg       Config
	transport *transport.Client
}

func NewDeepSeekModel(cfg Config) *DeepSeekModel {
	if cfg.Name == "" {
		cfg.Name = "deepseek"
	}
	return &DeepSeekModel{
		cfg:       cfg,
		transport: transport.NewClient(cfg.BaseURL),
	}
}

func (c *DeepSeekModel) Name() string {
	return c.cfg.Name
}

func (c *DeepSeekModel) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	req.Model = c.model(req.Model)
	req.Stream = false

	var raw openAIChatResponse
	if err := c.transport.PostJSON(ctx, "/chat/completions", c.authHeaders(), req, &raw); err != nil {
		return nil, err
	}

	resp := &llm.ChatResponse{
		InputTokens:  raw.Usage.PromptTokens,
		OutputTokens: raw.Usage.CompletionTokens,
	}
	if len(raw.Choices) > 0 {
		resp.Content = raw.Choices[0].Message.Content.Text
	}
	return resp, nil
}

func (c *DeepSeekModel) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	req.Model = c.model(req.Model)
	req.Stream = true

	ch := make(chan llm.StreamChunk)
	go func() {
		defer close(ch)

		err := c.transport.StreamJSON(ctx, "/chat/completions", c.authHeaders(), req, func(data string) error {
			if data == "[DONE]" {
				return nil
			}

			raw, err := llm.ParseInto[openAIChatStreamResponse](data)
			if err != nil {
				return err
			}
			for _, choice := range raw.Choices {
				if choice.Delta.Content.Text == "" {
					continue
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case ch <- llm.StreamChunk{Content: choice.Delta.Content.Text}:
				}
			}
			return nil
		})
		if err != nil {
			select {
			case <-ctx.Done():
			case ch <- llm.StreamChunk{Err: err}:
			}
		}
	}()

	return ch, nil
}

func (c *DeepSeekModel) model(reqModel string) string {
	if reqModel != "" {
		return reqModel
	}
	return c.cfg.Model
}

func (c *DeepSeekModel) authHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
	}
}

type openAIChatResponse struct {
	Choices []struct {
		Message llm.Message `json:"message"`
	} `json:"choices"`

	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIChatStreamResponse struct {
	Choices []struct {
		Delta llm.Message `json:"delta"`
	} `json:"choices"`
}

var _ llm.Provider = (*DeepSeekModel)(nil)

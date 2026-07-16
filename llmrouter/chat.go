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
	"strings"
)

//=================================适配 OpenAI 风格=================================

// 非流式调用
func GPTChat(ctx context.Context, cfg LLMConfig, question string) (*ChatResponse, error) {
	// body
	chatReq := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: User, Content: question},
		},
		Stream: false,
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// req
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	// resp
	client := NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	// 解析 resp
	chatResp, err := parseResp(resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return chatResp, nil
}

// 流式调用
func GPTChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
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

//=================================适配 claude 风格=================================

func ClaudeChat(ctx context.Context, cfg LLMConfig, question string) (*ChatResponse, error) {
	chatReq := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: User, Content: question},
		},
		Stream: false,
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)

	client := NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat failed: status=%d body=%s", resp.StatusCode, string(b))
	}

	var raw struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var content string
	for _, c := range raw.Content {
		content += c.Text
	}
	return &ChatResponse{
		Content:      content,
		InputTokens:  raw.Usage.InputTokens,
		OutputTokens: raw.Usage.OutputTokens,
	}, nil
}

func ClaudeChatStream(ctx context.Context, cfg LLMConfig, question string) (<-chan StreamChunk, error) {
	stream := make(chan StreamChunk)

	url := cfg.BaseURL + "/v1/messages"
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
	req.Header.Set("x-api-key", cfg.APIKey)

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
				Type  string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &raw); err != nil {
				stream <- StreamChunk{Err: fmt.Errorf("decode stream chunk: %w", err)}
				return
			}

			if raw.Type == "content_block_delta" {
				select {
				case stream <- StreamChunk{Content: raw.Delta.Text}:
				case <-ctx.Done():
					stream <- StreamChunk{Err: ctx.Err()}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			stream <- StreamChunk{Err: err}
		}
	}()

	return stream, nil
}

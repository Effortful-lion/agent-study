// Package llmlib 是一个 LLM 开发的标准库，提供统一的接口来调用各种 AI 服务商的 API。
//
// 主要特性:
//   - 支持多种 AI 服务商: OpenAI、DeepSeek、Doubao、Claude、Zhipu、Tongyi、Kimi
//   - 提供同步和流式两种调用方式
//   - 统一的消息格式和响应结构
//   - 支持自定义 HTTP 客户端配置
//   - 内置 token 估算功能
//
// 快速开始:
//
//	resp, err := llmlib.Chat(ctx, "deepseek", apiKey, []llmlib.Message{
//	    llmlib.NewUserMessage("你好"),
//	}, llmlib.WithModel("deepseek-chat"))
package llmlib

import (
	"context"
	"fmt"
)

// Chat 发送同步聊天请求到指定的 AI 服务商
// providerName: 服务商名称，支持 "openai"、"deepseek"、"doubao"、"claude"、"zhipu"、"tongyi"、"kimi"
// apiKey: API 密钥
// messages: 消息列表
// opts: 可选配置项，如 WithModel、WithBaseURL
//
// 示例:
//
//	resp, err := llmlib.Chat(ctx, "deepseek", apiKey, []llmlib.Message{
//	    llmlib.NewUserMessage("你好"),
//	}, llmlib.WithModel("deepseek-chat"))
func Chat(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	p, err := NewProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}

	return p.Chat(ctx, cfg, messages)
}

// ChatStream 发送流式聊天请求到指定的 AI 服务商
// 返回一个 channel，用于接收流式响应数据
//
// 示例:
//
//	stream, err := llmlib.ChatStream(ctx, "deepseek", apiKey, []llmlib.Message{
//	    llmlib.NewUserMessage("你好"),
//	}, llmlib.WithModel("deepseek-chat"))
//	for chunk := range stream {
//	    if chunk.Err != nil {
//	        return err
//	    }
//	    fmt.Print(chunk.Content)
//	}
func ChatStream(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (<-chan StreamChunk, error) {
	p, err := NewProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}

	return p.ChatStream(ctx, cfg, messages)
}

// getDefaultBaseURL 根据服务商名称获取默认 BaseURL
func getDefaultBaseURL(providerName string) string {
	switch providerName {
	case "openai":
		return OPENAI_BASEURL
	case "doubao":
		return DOUBAO_BASEURL
	case "deepseek":
		return DEEPSEEK_BASEURL
	case "claude":
		return CLAUDE_BASEURL
	case "zhipu":
		return ZHIPU_BASEURL
	case "tongyi":
		return TONGYI_BASEURL
	case "kimi":
		return KIMI_BASEURL
	default:
		return ""
	}
}

// NewMessage 创建一条消息
func NewMessage(role Role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// NewUserMessage 创建一条用户消息
func NewUserMessage(content string) Message {
	return NewMessage(User, content)
}

// NewSystemMessage 创建一条系统消息
func NewSystemMessage(content string) Message {
	return NewMessage(System, content)
}

// NewAssistantMessage 创建一条助手消息
func NewAssistantMessage(content string) Message {
	return NewMessage(Assistant, content)
}

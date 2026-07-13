package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURL = "https://api.deepseek.com" // 模型厂商的 base URL
	model   = "deepseek-v4-pro"          // 使用的模型名称
)

// Message 对话单条消息结构体，对齐OpenAI标准消息格式
type Message struct {
	Role    string `json:"role"`    // 消息角色：system系统提示/user用户输入/assistant模型回复
	Content string `json:"content"` // 消息文本内容
}

// ChatRequest 大模型对话请求体，兼容OpenAI接口入参规范
type ChatRequest struct {
	Model    string    `json:"model"`    // 指定调用的模型名称，如gpt-4o、deepseek-v3等
	Messages []Message `json:"messages"` // 历史对话消息数组，按对话顺序排列
	Stream   bool      `json:"stream"`   // 是否开启流式返回：true流式分片输出，false一次性完整返回
}

// ChatResponse 大模型对话完整非流式返回响应体
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"` // 当前轮次模型生成的回复消息
	} `json:"choices"` // 模型生成结果数组，通常仅返回1条结果

	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`     // 输入提示词消耗token数量
		CompletionTokens int `json:"completion_tokens"` // 模型输出回复消耗token数量
		TotalTokens      int `json:"total_tokens"`      // 本轮对话总消耗token（输入+输出）
	} `json:"usage"` // Token用量统计，用于计费、限流统计
}

func main() {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		log.Fatal("请先设置 LLM_API_KEY")
	}

	question := "用一句话解释什么是 AI Agent"
	if len(os.Args) > 1 {
		question = strings.Join(os.Args[1:], " ")
	}

	payload := ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: question},
		},
		Stream: false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		log.Fatalf("模型接口返回 %s: %s", resp.Status, raw)
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatal(err)
	}
	if len(result.Choices) == 0 {
		log.Fatal("模型响应中没有 choices")
	}

	fmt.Println(result.Choices[0].Message.Content)
	fmt.Printf(
		"token: input=%d output=%d total=%d\n",
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
}

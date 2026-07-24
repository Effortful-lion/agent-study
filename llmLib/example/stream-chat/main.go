package main

import (
	"context"
	"fmt"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

//func main() {
//	apiKey := os.Getenv(llmlib.DOUBAO_API_KEY)
//	if apiKey == "" {
//		fmt.Println("请设置 DOUBAO_API_KEY 环境变量")
//		return
//	}
//
//	messages := []llmlib.Message{
//		llmlib.NewUserMessage("用一句话描述什么是人工智能"),
//	}
//
//	stream, err := llmlib.ChatStream(context.Background(), llmlib.ProviderDoubao, apiKey, messages)
//	if err != nil {
//		fmt.Printf("流式聊天失败: %v\n", err)
//		return
//	}
//
//	fmt.Print("响应内容: ")
//	for chunk := range stream {
//		if chunk.Err != nil {
//			fmt.Printf("\n流式错误: %v\n", chunk.Err)
//			return
//		}
//		fmt.Print(chunk.Content)
//	}
//	fmt.Println()
//}

func main() {
	// 本地不需要 apikey
	messages := []llmlib.Message{
		llmlib.NewUserMessage("用go语言写一个快速排序，只输出go代码。"),
	}

	var options []llmlib.ChatOption
	options = append(options,
		llmlib.WithModel("Qwen2.5-Coder-3B-Instruct-4bit"),
		llmlib.WithBaseURL("http://localhost:8095/v1"))

	stream, err := llmlib.ChatStream(context.Background(), llmlib.ProviderQwen, "", messages, options...)
	if err != nil {
		fmt.Printf("流式聊天失败: %v\n", err)
		return
	}

	fmt.Print("响应内容: ")
	for chunk := range stream {
		if chunk.Err != nil {
			fmt.Printf("\n流式错误: %v\n", chunk.Err)
			return
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}

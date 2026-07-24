package main

import (
	"context"
	"fmt"
	"os"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

func main() {
	// ==============================================
	// 方式一：使用本地部署的 Qwen 模型（推荐）
	// 本地模型默认地址 http://localhost:8095/v1，无需 API Key
	// ==============================================
	//{
	//	providerName := llmlib.ProviderQwen
	//
	//	// 本地模型通常不需要 API Key，但本地服务可能仍然需要校验
	//	// 如果本地服务不需要鉴权，传空字符串或任意占位字符串即可
	//	apiKey := os.Getenv(llmlib.QWEN_API_KEY)
	//	if apiKey == "" {
	//		apiKey = os.Getenv(llmlib.API_KEY)
	//	}
	//	if apiKey == "" {
	//		// 本地部署通常不需要鉴权，使用占位 key
	//		apiKey = "not-needed"
	//	}
	//
	//	userInput := "写一个快速排序"
	//	if len(os.Args) > 1 {
	//		userInput = os.Args[1]
	//	}
	//
	//	messages := []llmlib.Message{
	//		llmlib.NewSystemMessage("你是一个友好的助手"),
	//		llmlib.NewUserMessage(userInput),
	//	}
	//
	//	// WithModel 和 WithBaseURL 都可以按需覆盖
	//	var options []llmlib.ChatOption
	//	options = append(options,
	//		llmlib.WithModel("Qwen2.5-Coder-3B-Instruct-4bit"),
	//		llmlib.WithBaseURL("http://localhost:8095/v1"))
	//
	//	resp, err := llmlib.Chat(
	//		context.Background(),
	//		providerName,
	//		apiKey,
	//		messages,
	//		options...,
	//	// 如果本地模型的模型名不同，可以通过 WithModel 覆盖
	//	// llmlib.WithModel("qwen3"),
	//	// 如果本地服务地址不同，可以通过 WithBaseURL 覆盖
	//	// llmlib.WithBaseURL("http://localhost:8095/v1"),
	//	)
	//	if err != nil {
	//		fmt.Printf("本地 Qwen 模型聊天失败: %v\n", err)
	//		return
	//	}
	//
	//	fmt.Printf("[本地 Qwen] 响应内容: %s\n", resp.Content)
	//	fmt.Printf("[本地 Qwen] 输入 token: %d, 输出 token: %d\n", resp.InputTokens, resp.OutputTokens)
	//}

	// ==============================================
	// 方式二（原代码，已注释）：使用 DeepSeek 远程模型
	// ==============================================
	{
		providerName := llmlib.ProviderDoubao
		apiKey := os.Getenv(llmlib.API_KEY)
		if apiKey == "" {
			apiKey = os.Getenv(llmlib.DOUBAO_API_KEY)
		}
		if apiKey == "" {
			fmt.Println("请设置 API_KEY 或 DOUBAO_API_KEY 环境变量")
			return
		}

		userInput := "你好，介绍一下你自己"
		if len(os.Args) > 1 {
			userInput = os.Args[1]
		}

		messages := []llmlib.Message{
			llmlib.NewSystemMessage("你是一个友好的助手"),
			llmlib.NewUserMessage(userInput),
		}

		resp, err := llmlib.Chat(context.Background(), providerName, apiKey, messages, llmlib.WithModel("doubao-seed-2-0-code-preview-260215"))
		if err != nil {
			fmt.Printf("聊天失败: %v\n", err)
			return
		}

		fmt.Printf("响应内容: %s\n", resp.Content)
		fmt.Printf("输入 token: %d, 输出 token: %d\n", resp.InputTokens, resp.OutputTokens)
	}
}

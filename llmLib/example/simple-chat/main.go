package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Effortful-lion/agent-study/llmLib"
)

func main() {
	providerName := llmlib.ProviderDeepSeek
	apiKey := os.Getenv(llmlib.API_KEY)
	if apiKey == "" {
		apiKey = os.Getenv(llmlib.DEEPSEEK_API_KEY)
	}
	if apiKey == "" {
		fmt.Println("请设置 API_KEY 或 DEEPSEEK_API_KEY 环境变量")
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

	resp, err := llmlib.Chat(context.Background(), providerName, apiKey, messages, llmlib.WithModel("deepseek-v4-flash"))
	if err != nil {
		fmt.Printf("聊天失败: %v\n", err)
		return
	}

	fmt.Printf("响应内容: %s\n", resp.Content)
	fmt.Printf("输入 token: %d, 输出 token: %d\n", resp.InputTokens, resp.OutputTokens)
}

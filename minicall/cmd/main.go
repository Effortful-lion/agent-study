package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Effortful-lion/agent-study/minicall/internal/ai"
)

func main() {
	// 1. read the env from local environment
	apikey := os.Getenv("LLM_API_KEY")
	if apikey == "" {
		fmt.Println("Please set LLM_API_KEY environment variable, eg: export LLM_API_KEY=sk-xxx")
		os.Exit(1)
	}
	baseurl := os.Getenv("LLM_BASE_URL")
	if baseurl == "" {
		fmt.Println("Please set LLM_BASE_URL environment variable, eg: export LLM_BASE_URL=https://api.deepseek.com")
		os.Exit(1)
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		fmt.Println("Please set model environment variable, eg: export LLM_MODEL=deepseek-v4-pro")
		os.Exit(1)
	}

	// 2. new a question
	// 注册长参数 --question
	var question string
	flag.StringVar(&question, "question", "你是什么模型？", "输入提问内容")
	// 绑定短参数 -q，复用同一个变量
	flag.StringVar(&question, "q", "你是什么模型？", "输入提问内容简写")
	flag.Parse()

	// 3. ask to ai 问题：我们这里需要调用ai能力，但是ai能力封装在 internal 中
	client := ai.NewChatModel(ai.Config{
		Model:   model,
		BaseURL: baseurl,
		APIKey:  apikey,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.StreamInvokeChat(ctx, question, os.Stdout); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println()
}

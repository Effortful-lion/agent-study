package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Effortful-lion/agent-study/minicall-tansuo/internal/ai"
	"github.com/Effortful-lion/agent-study/minicall-tansuo/internal/llm"
)

var (
	apikey   string
	model    string
	baseurl  string
	question string
)

func CLICmd() {
	// 2. new a question
	// 注册长参数 --question
	flag.StringVar(&question, "question", "你是什么模型？", "输入提问内容")
	// 绑定短参数 -q，复用同一个变量
	flag.StringVar(&question, "q", "你是什么模型？", "输入提问内容简写")
	flag.Parse()
}

func Getenv() {
	// 1. read the env from local environment
	apikey = os.Getenv("LLM_API_KEY")
	if apikey == "" {
		fmt.Println("Please set LLM_API_KEY environment variable, eg: export LLM_API_KEY=sk-xxx")
		os.Exit(1)
	}
	baseurl = os.Getenv("LLM_BASE_URL")
	if baseurl == "" {
		fmt.Println("Please set LLM_BASE_URL environment variable, eg: export LLM_BASE_URL=https://api.deepseek.com")
		os.Exit(1)
	}
	model = os.Getenv("LLM_MODEL")
	if model == "" {
		fmt.Println("Please set model environment variable, eg: export LLM_MODEL=deepseek-v4-pro")
		os.Exit(1)
	}
}

func main() {
	// 配置环境变量
	Getenv()

	// cli cmd
	CLICmd()

	// 3. ask to ai 问题：我们这里需要调用ai能力，但是ai能力封装在 internal 中
	client := ai.NewDeepSeekModel(ai.Config{
		Name:    "deepseek",
		Model:   model,
		BaseURL: baseurl,
		APIKey:  apikey,
	})
	var provider llm.Provider = client

	// Ctrl+C 会直接取消 ctx，并贯穿到 http.NewRequestWithContext 绑定的请求。
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 全局链路超时控制。
	ctx, cancel := context.WithTimeout(signalCtx, 5*time.Minute)
	defer cancel()

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: llm.TextContent(question)},
		},
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\n用户取消")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("\n请求超时")
			return
		}

		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(resp.Content)
	fmt.Printf("token: input=%d output=%d\n", resp.InputTokens, resp.OutputTokens)
}

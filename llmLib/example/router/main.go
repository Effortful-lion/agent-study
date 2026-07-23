package main

import (
	"context"
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib"
)

func main() {
	fmt.Println("=== 多厂商路由示例 ===")
	fmt.Println("使用方式：")
	fmt.Println("  1. 设置多个 provider 的 API_KEY 环境变量")
	fmt.Println("     export DOUBAO_API_KEY=xxx")
	fmt.Println("     export DEEPSEEK_API_KEY=xxx")
	fmt.Println("     export ZHIPU_API_KEY=xxx")
	fmt.Println("  2. 系统会自动检测并启用已配置的 provider")
	fmt.Println("  3. Router 会根据策略选择最优 provider")
	fmt.Println()

	services, err := llmlib.LoadAll()
	if err != nil {
		fmt.Printf("加载服务失败: %v\n", err)
		return
	}

	if len(services) == 0 {
		fmt.Println("没有可用的服务，请设置至少一个 provider 的 API_KEY")
		return
	}

	fmt.Printf("已加载 %d 个服务商:\n", len(services))
	for _, s := range services {
		fmt.Printf("  - %s (模型: %s, 输入单价: %.4f, 输出单价: %.4f)\n",
			s.Name, s.Config.Model, s.Config.InputPricePerMillion, s.Config.OutputPricePerMillion)
	}
	fmt.Println()

	strategy := llmlib.ReadStrategyFromEnv()
	fmt.Printf("使用策略: %s\n", strategy)
	fmt.Println()

	router := llmlib.NewRouter(services, strategy)

	messages := []llmlib.Message{
		llmlib.NewUserMessage("用中文写一段关于编程的名言"),
	}

	fmt.Println("--- 同步路由调用 ---")
	result, err := router.Chat(context.Background(), messages)
	if err != nil {
		fmt.Printf("路由调用失败: %v\n", err)
		return
	}

	fmt.Printf("成功服务商: %s\n", result.Provider)
	fmt.Printf("使用模型: %s\n", result.Model)
	fmt.Printf("响应内容: %s\n", result.Response.Content)
	fmt.Printf("估算成本: %.6f\n", result.Cost)
	fmt.Printf("延迟统计: %d 样本, P50=%v, P95=%v\n",
		result.Latency.Samples, result.Latency.P50, result.Latency.P95)

	if len(result.LastErrors) > 0 {
		fmt.Printf("失败记录: %d 个\n", len(result.LastErrors))
	}
}
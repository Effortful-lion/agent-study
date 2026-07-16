package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

/*
[]基础实现：开发openai.Provider，封装普通对话Chat、流式对话ChatStream两个核心方法；
[]异构适配：开发claude.Provider，内部完成请求 / 响应协议转换，对外对齐通用Provider接口；
[]兼容厂商快速接入：对接豆包 / DeepSeek，仅修改基础地址、密钥、模型名称，复用 OpenAI 实现；
[]配置与工厂封装：实现BuildAll工厂函数，通过环境变量批量加载、初始化所有模型服务商；
[]路由故障转移：开发Router调度器，支持模型调用失败自动切换备用服务商，最终输出：服务商标识、token 消耗、预估计费成本。
[]调度策略：实现「最便宜优先 CheapestFirst」「最低延迟 LowestLatency」两种路由算法；
[]性能观测：统计全链路 P50、P95 分位延迟指标；
*/

func main() {
	// 用 signal.NotifyContext 贯穿取消
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 从命令行参数拼接用户问题
	question := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, "用法: minicall <你的问题>")
		os.Exit(1)
	}

	if err := LoadDotEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	services, err := BuildAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	router := NewRouter(services, ReadStrategyFromEnv())
	PrintRouterChatStream(ctx, router, question)
}

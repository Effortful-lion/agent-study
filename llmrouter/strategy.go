package main

import (
	"os"
	"sort"
	"strings"
)

// 各种生产策略

//===========================模型调度策略===========================

// 调度策略接口
type Strategy string

const (
	StrategyDefault       Strategy = "default"
	StrategyCheapestFirst Strategy = "cheapest_first"
	StrategyLowestLatency Strategy = "lowest_latency"
)

// ReadStrategyFromEnv 从环境变量读取路由策略，未配置或无法识别时使用默认策略。
func ReadStrategyFromEnv() Strategy {
	_ = LoadDotEnv()

	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_ROUTING_STRATEGY")))
	raw = strings.ReplaceAll(raw, "_", "")
	raw = strings.ReplaceAll(raw, "-", "")

	switch raw {
	case "cheapestfirst":
		return StrategyCheapestFirst
	case "lowestlatency":
		return StrategyLowestLatency
	default:
		return StrategyDefault
	}
}

// SelectStrategy 在每次 chat 调用前根据策略选择实际尝试顺序。
func SelectStrategy(strategy Strategy, services []LLMService) []LLMService {
	switch strategy {
	case StrategyCheapestFirst:
		return strategyCheapestFirst(services)
	case StrategyLowestLatency:
		return strategyLowestLatency(services)
	default:
		return strategyDefault(services)
	}
}

// 默认调度
func strategyDefault(services []LLMService) []LLMService {
	return append([]LLMService(nil), services...)
}

// 最低花费调度
func strategyCheapestFirst(services []LLMService) []LLMService {
	ordered := strategyDefault(services)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].Config.InputPricePerMillion + ordered[i].Config.OutputPricePerMillion
		right := ordered[j].Config.InputPricePerMillion + ordered[j].Config.OutputPricePerMillion
		return left < right
	})
	return ordered
}

// 最快响应调度
func strategyLowestLatency(services []LLMService) []LLMService {
	ordered := strategyDefault(services)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Config.LatencyMS < ordered[j].Config.LatencyMS
	})
	return ordered
}

//===========================模型调度策略===========================

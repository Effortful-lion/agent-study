// 文件职责：
// - 定义 Agent 的停止条件和 Token 预算，避免 Agent 死循环、超时或成本失控。
// - AgentBudget 包含步数、token、时长限制和重试策略。
// - ShouldStop 判断是否应停止执行，ShouldRetry 判断是否应重试某动作。

package llmlib

import "time"

// AgentBudgetConfig 定义 Agent 的停止条件和预算约束，所有字段为 0 表示不限制。
// 预算是 Agent 运行时的重要护栏，确保执行过程可控、可感知。
type AgentBudgetConfig struct {
	MaxSteps         int           // 最大执行步数，0 表示不限制
	MaxTotalTokens   int           // 累计最大 token 数，0 表示不限制
	MaxDuration      time.Duration // 最大运行时长，0 表示不限制
	MaxRetries       int           // 单次工具调用最大重试次数
	MaxActionRetries int           // 同一动作最大重复次数，防止死循环
}

// DefaultAgentBudgetConfig 返回一个安全的默认预算配置，适用于大多数场景。
// 默认配置：10 步、100000 token、5 分钟、3 次重试、3 次动作重复。
func DefaultAgentBudgetConfig() AgentBudgetConfig {
	return AgentBudgetConfig{
		MaxSteps:         10,
		MaxTotalTokens:   100000,
		MaxDuration:      5 * time.Minute,
		MaxRetries:       3,
		MaxActionRetries: 3,
	}
}

// ShouldStop 根据当前状态判断是否应停止执行。
// 满足以下任一条件即停止：步数超限、token 超限、时长超限。
func (b AgentBudgetConfig) ShouldStop(state *State) bool {
	if b.MaxSteps > 0 && state.Step >= b.MaxSteps {
		return true
	}
	if b.MaxTotalTokens > 0 && state.Usage.InputTokens+state.Usage.OutputTokens >= b.MaxTotalTokens {
		return true
	}
	if b.MaxDuration > 0 && time.Since(state.StartedAt) >= b.MaxDuration {
		return true
	}
	return false
}

// ShouldRetry 判断是否应重试某动作，防止同一动作无限循环。
// 当 MaxActionRetries <= 0 时允许无限重试。
func (b AgentBudgetConfig) ShouldRetry(actionKey string, actionCounts map[string]int) bool {
	if b.MaxActionRetries <= 0 {
		return true
	}
	if count := actionCounts[actionKey]; count >= b.MaxActionRetries {
		return false
	}
	return true
}

// 文件职责：
// - 定义 Agent 的停止条件和 Token 预算，避免 Agent 死循环、超时或成本失控。

package agent

import "time"

// AgentBudgetConfig 定义 Agent 的停止条件和预算约束，所有字段为 0 表示不限制。
type AgentBudgetConfig struct {
	MaxSteps         int
	MaxTotalTokens   int
	MaxDuration      time.Duration
	MaxRetries       int
	MaxActionRetries int
}

// DefaultAgentBudgetConfig 返回一个安全的默认预算配置。
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
func (b AgentBudgetConfig) ShouldRetry(actionKey string, actionCounts map[string]int) bool {
	if b.MaxActionRetries <= 0 {
		return true
	}
	if count := actionCounts[actionKey]; count >= b.MaxActionRetries {
		return false
	}
	return true
}

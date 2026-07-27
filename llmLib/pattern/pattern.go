// Package pattern 提供 AI Agent 的 5 大工作模式（Agentic Patterns）。
//
// 5 大范式参考 Anthropic "Building Effective Agents"：
//
//  1. Chain（链式）          — 最简单的顺序执行：A → B → C → D
//  2. Router（路由）          — 根据输入选择不同分支
//  3. Parallel（并行）        — 多个步骤同时执行，结果聚合
//  4. Orchestrator（编排者-工作者）— 动态规划 + 子任务分发 + 合成
//  5. Evaluator-Optimizer（评估器-优化器）— 生成→评估→反馈→改进的循环
//
// 每个模式都是一个 Pattern 接口的实现，内部使用 state.Workflow 作为执行引擎。
//
// 与 Agent 的关系：
//   - Agent 的 Think-Act-Observe 循环本身就是一种 Graph 模式（有环图）
//   - Chain 可用于在 Agent 前后添加预处理/后处理步骤
//   - Router 可用于根据用户意图选择不同的 Agent 配置
//   - Parallel 可用于 Agent 内的并行工具调用
package pattern

import (
	"context"
)

// Pattern 是所有 Agent 工作模式的统一接口。
type Pattern interface {
	// Name 返回模式名称。
	Name() string

	// Description 返回模式描述。
	Description() string

	// Execute 执行该模式。
	// initialState: 初始状态，包含用户输入等上下文
	// 返回: 最终状态和执行过程中的事件流
	Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error)
}

// Event 是模式执行过程中产生的事件。
// 事件类型与 Agent 的事件系统对齐：
//
//	step_start / step_end  — 步骤开始/结束
//	tool_call / tool_result — 工具调用/结果（通过 _events 传递）
//	answer                  — 最终答案
//	error / done            — 错误/完成
type Event struct {
	Type    string // 事件类型：step_start, step_end, tool_call, tool_result, answer, error, done
	Step    int    // 当前步骤编号
	Name    string // 步骤名称
	Content string // 步骤内容 / 答案
	Tool    string // 工具名（tool_call/tool_result 事件）
	Args    string // 工具参数（tool_call 事件）
	Error   error  // 错误信息（error 事件）
}

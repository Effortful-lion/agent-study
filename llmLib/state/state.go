// Package state 提供通用的状态机和工作流执行引擎。
//
// 核心抽象：
//   - Machine: 状态机，管理状态转换和回调执行
//   - Step: 单个步骤，是状态机的基本执行单元
//   - Workflow: 链式工作流，提供 Do(step1, step2, ...) 风格的 API
//   - Middleware: 中间件，支持重试、超时、日志等横切关注点
//
// 与 llmLib 的关系：
//   - state 是纯粹的通用执行引擎，不依赖任何 LLM 相关类型
//   - llmLib 的 Agent 用 state.Workflow 来组织 Think-Act-Observe 循环
//   - llmLib/pattern 包用 state 来实现 5 大 Agent 范式
package state

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ============================================================================
// 基础类型
// ============================================================================

// Phase 是状态机的状态标识，使用 string 类型方便序列化和调试。
type Phase string

// StepFunc 是单个步骤的执行函数。
// 接收当前状态，返回下一步状态和可能的错误。
type StepFunc func(ctx context.Context, state map[string]any) (map[string]any, error)

// ConditionFunc 是条件判断函数，用于条件转换。
// 返回 true 时走 then 分支，false 时走 else 分支。
type ConditionFunc func(state map[string]any) bool

// Middleware 是中间件函数，包装 StepFunc 实现横切关注点。
type Middleware func(next StepFunc) StepFunc

// ============================================================================
// 转换定义
// ============================================================================

// Transition 定义一次状态转换：从 From 状态到 To 状态，执行 Action。
// 支持通过 Condition 实现条件分支。
type Transition struct {
	From      Phase         // 源状态
	To        Phase         // 目标状态
	Action    StepFunc      // 转换时执行的动作，可为 nil（纯状态跳转）
	Condition ConditionFunc // 条件判断，nil 表示无条件转换
}

// TransitionBuilder 提供流畅的转换定义 API。
type TransitionBuilder struct {
	transitions []Transition
}

// T 创建一个转换构建器。
func T(from Phase) *TransitionBuilder {
	return &TransitionBuilder{}
}

// To 指定目标状态。
func (tb *TransitionBuilder) To(to Phase) *TransitionBuilder {
	tb.transitions = append(tb.transitions, Transition{To: to})
	return tb
}

// Do 指定转换时执行的动作。
func (tb *TransitionBuilder) Do(action StepFunc) *TransitionBuilder {
	last := len(tb.transitions) - 1
	if last >= 0 {
		tb.transitions[last].Action = action
	}
	return tb
}

// When 指定转换的条件。
func (tb *TransitionBuilder) When(cond ConditionFunc) *TransitionBuilder {
	last := len(tb.transitions) - 1
	if last >= 0 {
		tb.transitions[last].Condition = cond
	}
	return tb
}

// Build 返回构建好的转换列表。
func (tb *TransitionBuilder) Build() []Transition {
	return tb.transitions
}

// ============================================================================
// 状态机定义
// ============================================================================

// MachineDef 定义状态机的完整配置。
type MachineDef struct {
	Initial     Phase        // 初始状态
	Transitions []Transition // 所有状态转换
	OnError     StepFunc     // 错误处理回调，可为 nil
	Middlewares []Middleware // 全局中间件，作用于所有转换
}

// Validate 校验定义的有效性。
func (def *MachineDef) Validate() error {
	if def.Initial == "" {
		return errors.New("state: 初始状态不能为空")
	}
	if len(def.Transitions) == 0 {
		return errors.New("state: 至少需要一个转换")
	}
	// 检查是否有从 Initial 出发的转换
	hasEntry := false
	for _, t := range def.Transitions {
		if t.From == def.Initial {
			hasEntry = true
			break
		}
	}
	if !hasEntry {
		return fmt.Errorf("state: 没有从初始状态 %q 出发的转换", def.Initial)
	}
	return nil
}

// ============================================================================
// 状态机运行时
// ============================================================================

// Machine 是状态机的运行时实例。
type Machine struct {
	def     *MachineDef
	state   map[string]any
	phase   Phase
	mws     []Middleware
	onError StepFunc
	history []Transition // 执行历史
}

// NewMachine 创建一个新的状态机实例。
func NewMachine(def *MachineDef, initialState map[string]any) (*Machine, error) {
	if err := def.Validate(); err != nil {
		return nil, err
	}
	if initialState == nil {
		initialState = make(map[string]any)
	}
	return &Machine{
		def:   def,
		state: initialState,
		phase: def.Initial,
		mws:   def.Middlewares,
	}, nil
}

// Phase 返回当前状态。
func (m *Machine) Phase() Phase {
	return m.phase
}

// State 返回当前状态数据。
func (m *Machine) State() map[string]any {
	return m.state
}

// History 返回执行历史。
func (m *Machine) History() []Transition {
	return m.history
}

// Run 启动状态机，执行到终态或出错。
// 终态的定义：当前状态下没有可匹配的转换。
func (m *Machine) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		transition, ok := m.matchTransition()
		if !ok {
			// 没有可匹配的转换，到达终态
			return nil
		}

		if err := m.executeTransition(ctx, transition); err != nil {
			return err
		}

		m.phase = transition.To
		m.history = append(m.history, transition)
	}
}

// RunUntil 执行到指定状态为止。
func (m *Machine) RunUntil(ctx context.Context, target Phase) error {
	for m.phase != target {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		transition, ok := m.matchTransition()
		if !ok {
			return fmt.Errorf("state: 无法从 %q 到达 %q：没有可匹配的转换", m.phase, target)
		}

		if err := m.executeTransition(ctx, transition); err != nil {
			return err
		}

		m.phase = transition.To
		m.history = append(m.history, transition)
	}
	return nil
}

// matchTransition 在当前状态中查找第一个满足条件的转换。
func (m *Machine) matchTransition() (Transition, bool) {
	for _, t := range m.def.Transitions {
		if t.From != m.phase {
			continue
		}
		if t.Condition != nil && !t.Condition(m.state) {
			continue
		}
		return t, true
	}
	return Transition{}, false
}

// executeTransition 执行一次转换，包括动作和中间件。
func (m *Machine) executeTransition(ctx context.Context, t Transition) error {
	if t.Action == nil {
		return nil
	}

	// 包装中间件
	action := t.Action
	for i := len(m.mws) - 1; i >= 0; i-- {
		action = m.mws[i](action)
	}

	newState, err := action(ctx, m.state)
	if err != nil {
		if m.def.OnError != nil {
			// 错误处理回调可以修改状态
			errState, errErr := m.def.OnError(ctx, m.state)
			if errErr == nil {
				m.state = errState
			}
		}
		return err
	}

	if newState != nil {
		m.state = newState
	}
	return nil
}

// ============================================================================
// 内置中间件
// ============================================================================

// RetryMiddleware 创建重试中间件。
// maxRetries: 最大重试次数
// baseDelay: 基础延迟
// maxDelay: 最大延迟
// isRetryable: 判断错误是否可重试，nil 表示所有错误都可重试
func RetryMiddleware(maxRetries int, baseDelay, maxDelay time.Duration, isRetryable func(error) bool) Middleware {
	return func(next StepFunc) StepFunc {
		return func(ctx context.Context, state map[string]any) (map[string]any, error) {
			var lastErr error
			delay := baseDelay
			for i := 0; i <= maxRetries; i++ {
				newState, err := next(ctx, state)
				if err == nil {
					return newState, nil
				}
				lastErr = err
				if isRetryable != nil && !isRetryable(err) {
					return newState, err
				}
				if i < maxRetries {
					select {
					case <-ctx.Done():
						return state, ctx.Err()
					case <-time.After(delay):
					}
					delay *= 2
					if delay > maxDelay {
						delay = maxDelay
					}
				}
			}
			return state, fmt.Errorf("state: 重试 %d 次后仍然失败: %w", maxRetries, lastErr)
		}
	}
}

// TimeoutMiddleware 创建超时中间件。
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next StepFunc) StepFunc {
		return func(ctx context.Context, state map[string]any) (map[string]any, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return next(ctx, state)
		}
	}
}

// RecoveryMiddleware 创建 panic 恢复中间件。
func RecoveryMiddleware() Middleware {
	return func(next StepFunc) StepFunc {
		return func(ctx context.Context, state map[string]any) (map[string]any, error) {
			var result map[string]any
			var execErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						execErr = fmt.Errorf("state: panic recovered: %v", r)
						state["_error"] = execErr.Error()
						result = state
					}
				}()
				result, execErr = next(ctx, state)
			}()
			return result, execErr
		}
	}
}

// LogMiddleware 创建日志中间件（通过回调函数输出）。
func LogMiddleware(logFn func(phase Phase, msg string)) Middleware {
	return func(next StepFunc) StepFunc {
		return func(ctx context.Context, state map[string]any) (map[string]any, error) {
			newState, err := next(ctx, state)
			if err != nil {
				if logFn != nil {
					logFn(Phase(state["_phase"].(string)), fmt.Sprintf("执行失败: %v", err))
				}
			}
			return newState, err
		}
	}
}

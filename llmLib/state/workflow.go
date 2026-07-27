package state

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Step 接口 — 可组合的执行单元
// ============================================================================

// Step 定义工作流中的一个可执行步骤。
// 每个 Step 有名称、执行逻辑和可选的配置（重试、超时等）。
type Step struct {
	Name        string
	Fn          StepFunc
	MaxRetries  int
	RetryDelay  time.Duration
	MaxDelay    time.Duration
	Timeout     time.Duration
	IsRetryable func(error) bool
}

// StepOption 是 Step 的配置选项。
type StepOption func(*Step)

// WithRetry 配置步骤的重试策略。
func WithRetry(maxRetries int, delay time.Duration) StepOption {
	return func(s *Step) {
		s.MaxRetries = maxRetries
		s.RetryDelay = delay
	}
}

// WithRetryPolicy 配置完整的重试策略。
func WithRetryPolicy(maxRetries int, baseDelay, maxDelay time.Duration, isRetryable func(error) bool) StepOption {
	return func(s *Step) {
		s.MaxRetries = maxRetries
		s.RetryDelay = baseDelay
		s.MaxDelay = maxDelay
		s.IsRetryable = isRetryable
	}
}

// WithTimeout 配置步骤的超时时间。
func WithTimeout(timeout time.Duration) StepOption {
	return func(s *Step) {
		s.Timeout = timeout
	}
}

// NewStep 创建一个新的步骤。
func NewStep(name string, fn StepFunc, opts ...StepOption) *Step {
	s := &Step{
		Name:       name,
		Fn:         fn,
		MaxRetries: 0,
		RetryDelay: time.Second,
		MaxDelay:   time.Minute,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ============================================================================
// Workflow — 链式工作流
// ============================================================================

// Workflow 是一组按顺序执行的步骤，提供 Do(step1, step2, ...) 风格 API。
// 支持全局中间件、状态传递和错误处理。
type Workflow struct {
	steps       []*Step
	middlewares []Middleware
	onError     StepFunc
}

// Do 创建一个工作流并添加步骤。
// 用法: workflow := Do(step1, step2, step3)
func Do(steps ...*Step) *Workflow {
	return &Workflow{steps: steps}
}

// Then 追加步骤到工作流末尾，返回自身以支持链式调用。
func (w *Workflow) Then(steps ...*Step) *Workflow {
	w.steps = append(w.steps, steps...)
	return w
}

// Use 添加全局中间件。
func (w *Workflow) Use(mws ...Middleware) *Workflow {
	w.middlewares = append(w.middlewares, mws...)
	return w
}

// OnError 设置错误处理回调。
func (w *Workflow) OnError(fn StepFunc) *Workflow {
	w.onError = fn
	return w
}

// Run 执行工作流中的所有步骤。
// 步骤按顺序执行，上一步的输出状态作为下一步的输入。
func (w *Workflow) Run(ctx context.Context, initialState map[string]any) (map[string]any, error) {
	if initialState == nil {
		initialState = make(map[string]any)
	}
	state := initialState
	state["_step"] = 0

	for i, step := range w.steps {
		state["_step"] = i
		state["_step_name"] = step.Name

		// 构建当前步骤的执行函数（包装中间件）
		fn := step.Fn
		if step.MaxRetries > 0 {
			retryMW := RetryMiddleware(step.MaxRetries, step.RetryDelay, step.MaxDelay, step.IsRetryable)
			fn = retryMW(fn)
		}
		if step.Timeout > 0 {
			timeoutMW := TimeoutMiddleware(step.Timeout)
			fn = timeoutMW(fn)
		}
		// 应用全局中间件（内层先执行）
		for j := len(w.middlewares) - 1; j >= 0; j-- {
			fn = w.middlewares[j](fn)
		}

		newState, err := fn(ctx, state)
		if err != nil {
			if w.onError != nil {
				errState, errErr := w.onError(ctx, state)
				if errErr == nil {
					state = errState
				}
			}
			return state, fmt.Errorf("workflow step %q failed: %w", step.Name, err)
		}
		if newState != nil {
			state = newState
		}
	}

	state["_step"] = len(w.steps)
	state["_step_name"] = "done"
	return state, nil
}

// RunWithCallback 执行工作流，每步完成后回调。
// 适用于需要实时反馈的场景（如 SSE 推送）。
func (w *Workflow) RunWithCallback(ctx context.Context, initialState map[string]any, callback func(stepName string, state map[string]any, err error) error) (map[string]any, error) {
	if initialState == nil {
		initialState = make(map[string]any)
	}
	state := initialState
	state["_step"] = 0

	for i, step := range w.steps {
		state["_step"] = i
		state["_step_name"] = step.Name

		fn := step.Fn
		if step.MaxRetries > 0 {
			fn = RetryMiddleware(step.MaxRetries, step.RetryDelay, step.MaxDelay, step.IsRetryable)(fn)
		}
		if step.Timeout > 0 {
			fn = TimeoutMiddleware(step.Timeout)(fn)
		}
		for j := len(w.middlewares) - 1; j >= 0; j-- {
			fn = w.middlewares[j](fn)
		}

		newState, err := fn(ctx, state)
		if callback != nil {
			if cbErr := callback(step.Name, state, err); cbErr != nil {
				return state, cbErr
			}
		}
		if err != nil {
			if w.onError != nil {
				errState, errErr := w.onError(ctx, state)
				if errErr == nil {
					state = errState
				}
			}
			return state, fmt.Errorf("workflow step %q failed: %w", step.Name, err)
		}
		if newState != nil {
			state = newState
		}
	}

	state["_step"] = len(w.steps)
	state["_step_name"] = "done"
	if callback != nil {
		_ = callback("done", state, nil)
	}
	return state, nil
}

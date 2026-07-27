package pattern

import (
	"context"
	"fmt"
	"sync"

	"github.com/Effortful-lion/agent-study/llmLib/state"
)

// Parallel 是并行模式：多个步骤同时执行，最后聚合结果。
//
// 适用场景：
//   - 同时查询多个数据源
//   - 同时调用多个工具获取不同维度的信息
//   - 并行处理独立子任务
//
// 使用示例：
//
//	parallel := pattern.NewParallel().
//	    AddStep(state.NewStep("weather", weatherFn)).
//	    AddStep(state.NewStep("news", newsFn)).
//	    Merge(func(results map[string]any) (map[string]any, error) {
//	        // 聚合所有结果
//	        return results, nil
//	    })
type Parallel struct {
	steps   []*state.Step
	mergeFn func(results map[string]any) (map[string]any, error)
	onError state.StepFunc
}

// NewParallel 创建一个新的并行模式。
func NewParallel() *Parallel {
	return &Parallel{}
}

// Name 返回模式名称。
func (p *Parallel) Name() string {
	return "parallel"
}

// Description 返回模式描述。
func (p *Parallel) Description() string {
	return "并行模式：多个步骤同时执行，结果聚合"
}

// AddStep 添加一个并行步骤。
func (p *Parallel) AddStep(step *state.Step) *Parallel {
	p.steps = append(p.steps, step)
	return p
}

// Merge 设置结果聚合函数。
func (p *Parallel) Merge(fn func(results map[string]any) (map[string]any, error)) *Parallel {
	p.mergeFn = fn
	return p
}

// OnError 设置错误处理回调。
func (p *Parallel) OnError(fn state.StepFunc) *Parallel {
	p.onError = fn
	return p
}

// Execute 执行并行模式。
func (p *Parallel) Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) {
	if len(p.steps) == 0 {
		return nil, fmt.Errorf("parallel: 至少需要一个步骤")
	}

	out := make(chan Event, 32)
	go func() {
		defer close(out)

		results := make(map[string]any)
		var mu sync.Mutex
		var wg sync.WaitGroup
		errCh := make(chan error, len(p.steps))

		for _, step := range p.steps {
			wg.Add(1)
			go func(s *state.Step) {
				defer wg.Done()

				fn := s.Fn
				if s.MaxRetries > 0 {
					fn = state.RetryMiddleware(s.MaxRetries, s.RetryDelay, s.MaxDelay, s.IsRetryable)(fn)
				}
				if s.Timeout > 0 {
					fn = state.TimeoutMiddleware(s.Timeout)(fn)
				}

				emitPatternEvent(out, Event{Type: "step_start", Name: s.Name})

				result, err := fn(ctx, initialState)
				if err != nil {
					errCh <- fmt.Errorf("step %q: %w", s.Name, err)
					return
				}

				mu.Lock()
				results[s.Name] = result
				mu.Unlock()

				emitPatternEvent(out, Event{Type: "step_end", Name: s.Name})
			}(step)
		}

		wg.Wait()
		close(errCh)

		// 收集错误
		var firstErr error
		for err := range errCh {
			if firstErr == nil {
				firstErr = err
			}
		}

		if firstErr != nil {
			emitPatternEvent(out, Event{Type: "error", Error: firstErr})
		} else if p.mergeFn != nil {
			merged, err := p.mergeFn(results)
			if err != nil {
				emitPatternEvent(out, Event{Type: "error", Error: err})
			} else {
				_ = merged
			}
		}

		emitPatternEvent(out, Event{Type: "done"})
	}()

	return out, nil
}

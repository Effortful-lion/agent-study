package pattern

import (
	"context"
	"fmt"
	"sync"

	"github.com/Effortful-lion/agent-study/llmLib/state"
)

// Orchestrator 是编排者-工作者模式：一个中央编排者动态分解任务，分发给多个工作者并行执行，最后合成结果。
//
// 适用场景：
//   - 复杂的多步骤任务（先规划再执行）
//   - 需要动态决策子任务的场景
//   - 类似 Plan-and-Execute 的工作模式
type Orchestrator struct {
	planner     state.StepFunc // 规划步骤：将输入分解为子任务列表
	worker      state.StepFunc // 工作者步骤：执行单个子任务
	synthesizer state.StepFunc // 合成步骤：汇总所有子任务结果
	onError     state.StepFunc
}

// NewOrchestrator 创建一个新的编排者模式。
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

// Name 返回模式名称。
func (o *Orchestrator) Name() string {
	return "orchestrator"
}

// Description 返回模式描述。
func (o *Orchestrator) Description() string {
	return "编排者-工作者模式：动态分解任务，并行执行，结果合成"
}

// Planner 设置规划步骤。fn 应返回包含 "subtasks" 键的状态，值为 []map[string]any。
func (o *Orchestrator) Planner(fn state.StepFunc) *Orchestrator {
	o.planner = fn
	return o
}

// Worker 设置工作者步骤。每个子任务作为独立状态传入。
func (o *Orchestrator) Worker(fn state.StepFunc) *Orchestrator {
	o.worker = fn
	return o
}

// Synthesizer 设置合成步骤。fn 应接收包含 "sub_results" 键的状态。
func (o *Orchestrator) Synthesizer(fn state.StepFunc) *Orchestrator {
	o.synthesizer = fn
	return o
}

// OnError 设置错误处理回调。
func (o *Orchestrator) OnError(fn state.StepFunc) *Orchestrator {
	o.onError = fn
	return o
}

// Execute 执行编排者模式。
func (o *Orchestrator) Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) {
	if o.planner == nil || o.worker == nil || o.synthesizer == nil {
		return nil, fmt.Errorf("orchestrator: Planner, Worker, Synthesizer 都必须设置")
	}

	out := make(chan Event, 32)
	go func() {
		defer close(out)

		// Step 1: Plan — 分解任务
		select {
		case out <- Event{Type: "step_start", Name: "orchestrator.plan"}:
		case <-ctx.Done():
			return
		}

		planState, err := o.planner(ctx, initialState)
		if err != nil {
			select {
			case out <- Event{Type: "error", Name: "orchestrator.plan", Error: err}:
			case <-ctx.Done():
			}
			return
		}

		select {
		case out <- Event{Type: "step_end", Name: "orchestrator.plan"}:
		case <-ctx.Done():
			return
		}

		// 获取子任务列表
		subtasksRaw, ok := planState["subtasks"]
		if !ok {
			select {
			case out <- Event{Type: "error", Name: "orchestrator.plan", Error: fmt.Errorf("规划结果缺少 subtasks")}:
			case <-ctx.Done():
			}
			return
		}

		subtasks, ok := subtasksRaw.([]map[string]any)
		if !ok {
			// 尝试 []any 类型
			rawList, ok := subtasksRaw.([]any)
			if !ok {
				select {
				case out <- Event{Type: "error", Name: "orchestrator.plan", Error: fmt.Errorf("subtasks 类型错误: %T", subtasksRaw)}:
				case <-ctx.Done():
				}
				return
			}
			for i, item := range rawList {
				subtasks = append(subtasks, map[string]any{
					"index": i,
					"task":  item,
				})
			}
		}

		// Step 2: Execute — 并行执行子任务
		select {
		case out <- Event{Type: "step_start", Name: "orchestrator.execute"}:
		case <-ctx.Done():
			return
		}

		subResults := make([]any, len(subtasks))
		var mu sync.Mutex
		var wg sync.WaitGroup
		errCh := make(chan error, len(subtasks))

		for i, subtask := range subtasks {
			wg.Add(1)
			go func(idx int, st map[string]any) {
				defer wg.Done()

				result, err := o.worker(ctx, st)
				if err != nil {
					errCh <- fmt.Errorf("subtask %d: %w", idx, err)
					return
				}

				mu.Lock()
				subResults[idx] = result
				mu.Unlock()
			}(i, subtask)
		}

		wg.Wait()
		close(errCh)

		var firstErr error
		for err := range errCh {
			if firstErr == nil {
				firstErr = err
			}
		}

		if firstErr != nil {
			select {
			case out <- Event{Type: "error", Name: "orchestrator.execute", Error: firstErr}:
			case <-ctx.Done():
			}
			return
		}

		select {
		case out <- Event{Type: "step_end", Name: "orchestrator.execute"}:
		case <-ctx.Done():
			return
		}

		// Step 3: Synthesize — 合成结果
		select {
		case out <- Event{Type: "step_start", Name: "orchestrator.synthesize"}:
		case <-ctx.Done():
			return
		}

		synthState := map[string]any{
			"sub_results": subResults,
			"plan":        planState,
		}
		_, err = o.synthesizer(ctx, synthState)
		if err != nil {
			select {
			case out <- Event{Type: "error", Name: "orchestrator.synthesize", Error: err}:
			case <-ctx.Done():
			}
			return
		}

		select {
		case out <- Event{Type: "step_end", Name: "orchestrator.synthesize"}:
		case <-ctx.Done():
			return
		}

		select {
		case out <- Event{Type: "done"}:
		case <-ctx.Done():
		}
	}()

	return out, nil
}

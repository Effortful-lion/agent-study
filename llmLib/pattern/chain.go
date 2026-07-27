package pattern

import (
	"context"
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib/state"
)

// Chain 是链式模式：按顺序执行一系列步骤，上一步的输出作为下一步的输入。
//
// 适用场景：
//   - 多步推理（先分析问题，再查资料，最后写答案）
//   - 数据流水线（提取 → 转换 → 加载）
//   - 固定的多步流程
//
// 使用示例：
//
//	chain := pattern.NewChain().
//	    AddStep(state.NewStep("analyze", analyzeFn)).
//	    AddStep(state.NewStep("research", researchFn)).
//	    AddStep(state.NewStep("write", writeFn))
//
//	events, _ := chain.Execute(ctx, initialData)
//	for event := range events {
//	    fmt.Printf("[%s] %s\n", event.Type, event.Name)
//	}
type Chain struct {
	steps   []*state.Step
	onError state.StepFunc
}

// NewChain 创建一个新的链式模式。
func NewChain() *Chain {
	return &Chain{}
}

// Name 返回模式名称。
func (c *Chain) Name() string {
	return "chain"
}

// Description 返回模式描述。
func (c *Chain) Description() string {
	return "链式模式：按顺序执行一系列步骤"
}

// AddStep 添加一个步骤到链末尾，返回自身以支持链式调用。
func (c *Chain) AddStep(step *state.Step) *Chain {
	c.steps = append(c.steps, step)
	return c
}

// OnError 设置错误处理回调。
func (c *Chain) OnError(fn state.StepFunc) *Chain {
	c.onError = fn
	return c
}

// Execute 执行链式模式。
// 每个步骤开始和结束时都会发出事件，步骤内部的额外事件通过 _events 字段传递。
func (c *Chain) Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) {
	if len(c.steps) == 0 {
		return nil, fmt.Errorf("chain: 至少需要一个步骤")
	}

	out := make(chan Event, 32)
	go func() {
		defer close(out)

		wf := state.Do(c.steps...)
		if c.onError != nil {
			wf.OnError(c.onError)
		}

		_, err := wf.RunWithCallback(ctx, initialState, func(stepName string, state map[string]any, err error) error {
			if err != nil {
				emitPatternEvent(out, Event{Type: "error", Name: stepName, Error: err})
				return nil
			}
			step := 0
			if v, ok := state["_step"].(int); ok {
				step = v
			}
			emitPatternEvent(out, Event{Type: "step_end", Step: step, Name: stepName})
			return nil
		})

		if err != nil {
			emitPatternEvent(out, Event{Type: "error", Name: "chain", Error: err})
		}

		emitPatternEvent(out, Event{Type: "done"})
	}()

	return out, nil
}

// emitPatternEvent 安全地发送事件。
func emitPatternEvent(out chan<- Event, event Event) {
	select {
	case out <- event:
	default:
		// channel 满了，跳过（调用方应该及时消费）
	}
}

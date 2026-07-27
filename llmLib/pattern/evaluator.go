package pattern

import (
	"context"
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib/state"
)

// Evaluator 是评估器-优化器模式：生成→评估→反馈→改进的循环。
//
// 适用场景：
//   - 代码生成 + 审查 + 修改
//   - 内容生成 + 质量检查 + 润色
//   - 自我纠正 / Reflexion 模式
type Evaluator struct {
	generator     state.StepFunc // 生成步骤
	evaluator     state.StepFunc // 评估步骤：返回 "score" 和 "feedback"
	maxIterations int            // 最大迭代次数
	threshold     float64        // 质量阈值，达到后停止
	onError       state.StepFunc
}

// NewEvaluator 创建一个新的评估器-优化器模式。
func NewEvaluator() *Evaluator {
	return &Evaluator{
		maxIterations: 3,
		threshold:     0.8,
	}
}

// Name 返回模式名称。
func (e *Evaluator) Name() string {
	return "evaluator"
}

// Description 返回模式描述。
func (e *Evaluator) Description() string {
	return "评估器-优化器模式：生成→评估→反馈→改进的迭代循环"
}

// Generator 设置生成步骤。
func (e *Evaluator) Generator(fn state.StepFunc) *Evaluator {
	e.generator = fn
	return e
}

// EvaluatorFn 设置评估步骤。fn 应返回包含 "score"(float64) 和 "feedback"(string) 的状态。
func (e *Evaluator) EvaluatorFn(fn state.StepFunc) *Evaluator {
	e.evaluator = fn
	return e
}

// MaxIterations 设置最大迭代次数。
func (e *Evaluator) MaxIterations(n int) *Evaluator {
	e.maxIterations = n
	return e
}

// Threshold 设置质量阈值。
func (e *Evaluator) Threshold(t float64) *Evaluator {
	e.threshold = t
	return e
}

// OnError 设置错误处理回调。
func (e *Evaluator) OnError(fn state.StepFunc) *Evaluator {
	e.onError = fn
	return e
}

// Execute 执行评估器-优化器模式。
func (e *Evaluator) Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) {
	if e.generator == nil || e.evaluator == nil {
		return nil, fmt.Errorf("evaluator: Generator 和 EvaluatorFn 都必须设置")
	}

	out := make(chan Event, 32)
	go func() {
		defer close(out)

		currentState := initialState
		for i := 0; i < e.maxIterations; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Step 1: Generate
			select {
			case out <- Event{Type: "step_start", Step: i + 1, Name: "evaluator.generate"}:
			case <-ctx.Done():
				return
			}

			genResult, err := e.generator(ctx, currentState)
			if err != nil {
				select {
				case out <- Event{Type: "error", Name: "evaluator.generate", Error: err}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case out <- Event{Type: "step_end", Step: i + 1, Name: "evaluator.generate"}:
			case <-ctx.Done():
				return
			}

			// Step 2: Evaluate
			select {
			case out <- Event{Type: "step_start", Step: i + 1, Name: "evaluator.evaluate"}:
			case <-ctx.Done():
				return
			}

			evalResult, err := e.evaluator(ctx, genResult)
			if err != nil {
				select {
				case out <- Event{Type: "error", Name: "evaluator.evaluate", Error: err}:
				case <-ctx.Done():
				}
				return
			}

			score := 0.0
			if s, ok := evalResult["score"].(float64); ok {
				score = s
			}

			feedback := ""
			if f, ok := evalResult["feedback"].(string); ok {
				feedback = f
			}

			select {
			case out <- Event{Type: "step_end", Step: i + 1, Name: "evaluator.evaluate", Content: fmt.Sprintf("score=%.2f feedback=%s", score, feedback)}:
			case <-ctx.Done():
				return
			}

			if score >= e.threshold {
				// 质量达标，输出最终结果
				select {
				case out <- Event{Type: "answer", Step: i + 1, Content: fmt.Sprintf("质量达标 (score=%.2f >= threshold=%.2f)", score, e.threshold)}:
				case <-ctx.Done():
				}
				break
			}

			// 将 feedback 合并到当前状态，进入下一轮迭代
			currentState = genResult
			currentState["feedback"] = feedback
			currentState["score"] = score
		}

		select {
		case out <- Event{Type: "done"}:
		case <-ctx.Done():
		}
	}()

	return out, nil
}

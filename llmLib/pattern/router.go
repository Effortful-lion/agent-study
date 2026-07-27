package pattern

import (
	"context"
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib/state"
)

// Router 是路由模式：根据输入条件选择不同的执行分支。
//
// 适用场景：
//   - 根据问题类型选择不同的处理流程
//   - 意图识别 + 分支处理
//   - 基于规则的流程分发
//
// 使用示例：
//
//	router := pattern.NewRouter(func(ctx context.Context, state map[string]any) (string, error) {
//	    query := state["query"].(string)
//	    if strings.Contains(query, "计算") {
//	        return "math", nil
//	    }
//	    return "general", nil
//	}).
//	    Branch("math", mathStep).
//	    Branch("general", generalStep)
type Router struct {
	routerFn     func(ctx context.Context, state map[string]any) (string, error)
	branches     map[string][]*state.Step
	defaultSteps []*state.Step
}

// NewRouter 创建一个新的路由模式。
func NewRouter(routerFn func(ctx context.Context, state map[string]any) (string, error)) *Router {
	return &Router{
		routerFn: routerFn,
		branches: make(map[string][]*state.Step),
	}
}

// Name 返回模式名称。
func (r *Router) Name() string {
	return "router"
}

// Description 返回模式描述。
func (r *Router) Description() string {
	return "路由模式：根据输入选择不同分支执行"
}

// Branch 添加一个分支，routeKey 是路由函数返回的键。
func (r *Router) Branch(routeKey string, steps ...*state.Step) *Router {
	r.branches[routeKey] = steps
	return r
}

// Default 设置默认分支（当路由函数返回未注册的键时使用）。
func (r *Router) Default(steps ...*state.Step) *Router {
	r.defaultSteps = steps
	return r
}

// Execute 执行路由模式。
func (r *Router) Execute(ctx context.Context, initialState map[string]any) (<-chan Event, error) {
	if r.routerFn == nil {
		return nil, fmt.Errorf("router: routerFn 不能为 nil")
	}

	out := make(chan Event, 32)
	go func() {
		defer close(out)

		// 路由决策
		routeKey, err := r.routerFn(ctx, initialState)
		if err != nil {
			emitPatternEvent(out, Event{Type: "error", Name: "router", Error: fmt.Errorf("路由决策失败: %w", err)})
			return
		}

		steps, ok := r.branches[routeKey]
		if !ok {
			steps = r.defaultSteps
		}

		if len(steps) == 0 {
			emitPatternEvent(out, Event{Type: "error", Name: "router", Error: fmt.Errorf("未找到路由键 %q 对应的分支", routeKey)})
			return
		}

		emitPatternEvent(out, Event{Type: "step_start", Name: fmt.Sprintf("router → %s", routeKey)})

		wf := state.Do(steps...)
		_, runErr := wf.RunWithCallback(ctx, initialState, func(stepName string, state map[string]any, err error) error {
			if err != nil {
				emitPatternEvent(out, Event{Type: "error", Name: stepName, Error: err})
			}
			return nil
		})

		if runErr != nil {
			emitPatternEvent(out, Event{Type: "error", Name: "router", Error: runErr})
		}

		emitPatternEvent(out, Event{Type: "done"})
	}()

	return out, nil
}

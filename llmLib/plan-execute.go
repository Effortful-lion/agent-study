// 文件职责：
// - 实现 Plan-and-Execute 架构模式，支持任务的拓扑排序和并行执行。
// - Plan 是任务的有序集合，通过依赖关系控制执行顺序。
// - Execute 函数使用 goroutine 并发执行可并行的任务，提高执行效率。

package llmlib

import (
	"context"
	"errors"
	"sync"
)

// Task 表示一个可执行的任务，包含唯一标识、描述、依赖关系和执行函数。
// 任务之间通过依赖关系形成有向无环图（DAG），支持并行执行。
type Task struct {
	ID          string   // 任务唯一标识
	Description string   // 任务描述，用于日志和调试
	DependsOn   []string // 依赖的任务 ID 列表
	Action      func(ctx context.Context, inputs map[string]any) (any, error)
}

// Plan 是任务的有序集合，通过依赖关系控制执行顺序。
// 执行时先进行拓扑排序，确保依赖任务先完成。
type Plan struct {
	Tasks []Task
}

// Levels 对任务进行拓扑排序，将依赖关系转化为层级列表。
// 返回的每一层任务都可以并行执行，同一层任务之间没有依赖关系。
// 支持检测重复任务 ID、缺失依赖和循环依赖。
func Levels(plan Plan) ([][]Task, error) {
	degree := make(map[string]int)
	graph := make(map[string][]string)
	taskMap := make(map[string]Task)
	for _, t := range plan.Tasks {
		if _, exists := taskMap[t.ID]; exists {
			return nil, errors.New("存在重复任务 ID: " + t.ID)
		}
		taskMap[t.ID] = t
		degree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			graph[dep] = append(graph[dep], t.ID)
		}
	}
	for id := range degree {
		for _, dep := range taskMap[id].DependsOn {
			if _, exists := taskMap[dep]; !exists {
				return nil, errors.New("任务 " + id + " 依赖不存在的任务: " + dep)
			}
		}
	}
	var levels [][]Task
	for len(taskMap) > 0 {
		var level []Task
		for id, t := range taskMap {
			if degree[id] == 0 {
				level = append(level, t)
				delete(taskMap, id)
			}
		}
		if len(level) == 0 {
			return nil, errors.New("存在循环依赖")
		}
		levels = append(levels, level)
		for _, t := range level {
			for _, next := range graph[t.ID] {
				degree[next]--
			}
		}
	}
	return levels, nil
}

// Execute 执行计划，按照拓扑排序的层级并行执行任务。
// ctx: 上下文，用于取消和超时控制
// plan: 待执行的计划
// 返回: 任务执行结果映射，键为任务 ID
func Execute(ctx context.Context, plan Plan) (map[string]any, error) {
	levels, err := Levels(plan)
	if err != nil {
		return nil, err
	}
	results := make(map[string]any)
	for _, level := range levels {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		var wg sync.WaitGroup
		levelResults := make(map[string]any)
		levelErrors := make(map[string]error)
		for _, task := range level {
			wg.Add(1)
			go func(t Task) {
				defer wg.Done()
				inputs := make(map[string]any)
				for _, dep := range t.DependsOn {
					if v, ok := results[dep]; ok {
						inputs[dep] = v
					}
				}
				if err := ctx.Err(); err != nil {
					return
				}
				result, err := t.Action(ctx, inputs)
				if err != nil {
					levelErrors[t.ID] = err
					return
				}
				levelResults[t.ID] = result
			}(task)
		}
		wg.Wait()
		if len(levelErrors) > 0 {
			var firstErr error
			for _, err := range levelErrors {
				firstErr = err
				break
			}
			return results, firstErr
		}
		for k, v := range levelResults {
			results[k] = v
		}
	}
	return results, nil
}

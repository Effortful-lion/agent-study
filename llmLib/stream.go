// 文件职责：
// - 提供流式响应的数据块结构和通用 channel 消费辅助函数。
// - 供模型流式接口、路由转发和上层调用方统一处理流式数据。

package llmlib

import "context"

// StreamChunk 表示模型流式返回中的单个片段或错误事件。
type StreamChunk struct {
	Content string // 文本片段，来自上游流式事件解析结果。
	Err     error  // 流式处理中的错误，非空时表示该事件为失败出口。
}

// Process 按顺序消费 channel 数据，直到通道关闭、上下文取消或处理函数报错。
func Process[T any](
	ctx context.Context,
	ch <-chan T,
	handler func(T) error,
) error {
	for {
		select {
		// 上游取消后立即结束消费，避免继续阻塞等待数据。
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-ch:
			if !ok {
				return nil
			}
			// 将每个元素交给调用方处理，并把处理错误原样返回。
			if err := handler(item); err != nil {
				return err
			}
		}
	}
}

// Collect 将整个 channel 的输出收集为切片，适合测试或一次性消费场景。
func Collect[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	var result []T
	err := Process(ctx, ch, func(item T) error {
		result = append(result, item)
		return nil
	})
	return result, err
}

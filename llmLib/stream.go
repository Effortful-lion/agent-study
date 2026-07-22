package llmlib

import "context"

// StreamChunk 表示流式响应的一个数据块
type StreamChunk struct {
	Content string // 内容片段
	Err     error  // 错误信息，如果有
}

// Process 处理一个 channel 中的所有元素，直到 channel 关闭或出错
func Process[T any](
	ctx context.Context,
	ch <-chan T,
	handler func(T) error,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-ch:
			if !ok {
				return nil
			}
			if err := handler(item); err != nil {
				return err
			}
		}
	}
}

// Collect 将 channel 中的所有元素收集到一个切片中
func Collect[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	var result []T
	err := Process(ctx, ch, func(item T) error {
		result = append(result, item)
		return nil
	})
	return result, err
}

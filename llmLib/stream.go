package llmlib

import "context"

type StreamChunk struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Err       error      `json:"-"`
}

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

func Collect[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	var result []T
	err := Process(ctx, ch, func(item T) error {
		result = append(result, item)
		return nil
	})
	return result, err
}
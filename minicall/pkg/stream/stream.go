package stream

import "context"

// Process 流式消费通道通用处理函数
// ctx: 上下文，用于控制取消、超时
// ch: 只读输入通道，源源不断产出 T 类型数据
// handler: 单条数据处理回调，返回 error 则终止整个消费流程
// 逻辑：循环读取通道数据，依次执行 handler；
//  1. ctx 取消：直接返回 ctx 错误
//  2. 通道关闭（ok=false）：正常结束，返回 nil
//  3. handler 返回错误：立即终止并返回该错误
func Process[T any](ctx context.Context, ch <-chan T, handler func(T) error) error {
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

// Collect 收集通道内所有元素到切片
// ctx: 上下文，支持超时/取消中断收集
// ch: 待读取的只读数据流通道
// 返回：
//
//	[]T: 通道内全部元素（中断前已读到的数据都会保留）
//	error: 上下文取消错误 / Process 处理回调抛出的错误
//
// 底层复用 Process，把每条数据 append 到切片，适合一次性拉取完整流结果
func Collect[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	var result []T
	err := Process(ctx, ch, func(item T) error {
		result = append(result, item)
		return nil
	})
	return result, err
}

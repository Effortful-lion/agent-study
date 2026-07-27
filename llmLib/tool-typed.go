// 文件职责：
// - 定义并实现 TypedTool[TInput, TOutput]：泛型、类型安全的生产级工具。
// - 自动从 Go 结构体类型生成 JSON Schema（通过 json/desc/omitempty 标签）。
// - 自动将 map[string]any 参数反序列化为 TInput，调用类型化函数。
// - 实现 Tool + SchemaTool 接口，无缝集成到现有 Registry / Agent 体系。
//
// 核心理念：把「定义一个工具」简化为「写一个类型化 Go 函数」。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// ============================================================================
// TypedTool — 泛型、类型安全的工具实现
// ============================================================================

// TypedTool 通过泛型将「定义一个工具」简化为「写一个类型化 Go 函数」。
//
// 特性：
//   - 自动从 TInput 生成 JSON Schema（支持 json/desc/omitempty 标签）
//   - 自动将 map[string]any 参数反序列化为 TInput
//   - 类型安全的 fn：不需要手写类型断言
//   - 实现 Tool + SchemaTool 接口，与 Registry/Agent 无缝集成
//
// 使用示例：
//
//	type WeatherInput struct {
//	    City string `json:"city" desc:"城市名称"`
//	    Days int    `json:"days" desc:"预报天数"`
//	}
//
//	tool := NewTypedTool("weather", "查询天气",
//	    func(ctx context.Context, in WeatherInput) (string, error) {
//	        return fetchWeather(in.City, in.Days)
//	    },
//	)
type TypedTool[TInput, TOutput any] struct {
	name        string
	description string
	fn          func(ctx context.Context, input TInput) (TOutput, error)
	schema      json.RawMessage
}

// NewTypedTool 创建一个类型安全的生产级工具。
//
// 参数：
//   - name:        工具名称，LLM 通过此名称识别并调用工具
//   - description: 工具描述，帮助 LLM 理解工具的用途和使用场景
//   - fn:          带类型的业务逻辑函数，接收 ctx + TInput，返回 TOutput 或 error
//
// 自动行为：
//   - 从 TInput 类型反射生成 JSON Schema（使用 json/desc/omitempty 标签）
//   - Call() 自动将 map[string]any 反序列化为 TInput
func NewTypedTool[TInput, TOutput any](
	name, description string,
	fn func(ctx context.Context, input TInput) (TOutput, error),
) *TypedTool[TInput, TOutput] {
	return &TypedTool[TInput, TOutput]{
		name:        name,
		description: description,
		fn:          fn,
		schema:      buildSchema[TInput](),
	}
}

// Name 返回工具名称。
func (t *TypedTool[TInput, TOutput]) Name() string { return t.name }

// Description 返回工具描述。
func (t *TypedTool[TInput, TOutput]) Description() string { return t.description }

// Parameters 返回 nil；TypedTool 通过 SchemaTool 接口提供 JSON Schema。
func (t *TypedTool[TInput, TOutput]) Parameters() map[string]string { return nil }

// ParametersSchema 返回从 TInput 类型自动生成的 JSON Schema。
// 实现 SchemaTool 接口，被 BuildToolDefs 优先使用。
func (t *TypedTool[TInput, TOutput]) ParametersSchema() json.RawMessage { return t.schema }

// Call 实现 Tool 接口。
// 自动将 map[string]any 参数反序列化为 TInput，调用业务函数，返回结果。
func (t *TypedTool[TInput, TOutput]) Call(ctx context.Context, args map[string]any) (any, error) {
	var input TInput
	if err := deserializeArgs(args, &input); err != nil {
		return nil, fmt.Errorf("typed tool %q: %w", t.name, err)
	}

	if err := validateInput(input); err != nil {
		return nil, fmt.Errorf("typed tool %q: %w", t.name, err)
	}

	output, err := t.fn(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("typed tool %q: %w", t.name, err)
	}

	return output, nil
}

// TypedCall 提供类型安全的直接调用方式，用于在 Go 代码中调用此工具。
// 与 Call 不同，TypedCall 不经过 map[string]any 反序列化，直接使用强类型参数。
func (t *TypedTool[TInput, TOutput]) TypedCall(ctx context.Context, input TInput) (TOutput, error) {
	return t.fn(ctx, input)
}

// Schema 返回缓存的 JSON Schema（json.RawMessage），用于外部检查或调试。
func (t *TypedTool[TInput, TOutput]) Schema() json.RawMessage { return t.schema }

// ============================================================================
// Validator — 可选输入验证接口
// ============================================================================

// Validator 是可选的输入验证接口。
// 如果 TInput 实现了此接口，Call() 会在调用业务函数前自动执行验证。
type Validator interface {
	Validate() error
}

// validateInput 如果 input 实现了 Validator 接口，执行验证。
func validateInput(input any) error {
	if v, ok := input.(Validator); ok {
		return v.Validate()
	}
	return nil
}

// ============================================================================
// Formatter — 可选输出格式化接口
// ============================================================================

// Formatter 是可选的输出格式化接口。
// 如果 TOutput 实现了此接口，Agent 可调用 Format() 获取适合返回给 LLM 的字符串表示。
// 当 TOutput 是复杂结构体时，实现此接口可提供更好的 LLM 可读性。
type Formatter interface {
	Format() string
}

// ============================================================================
// 内部辅助函数
// ============================================================================

// buildSchema 从 TInput 类型生成 JSON Schema 并序列化为 json.RawMessage。
func buildSchema[T any]() json.RawMessage {
	var zero T
	schema := Generate(instantiateForSchema(zero))
	if schema == nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	result, err := json.Marshal(schema)
	if err != nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	return result
}

// instantiateForSchema 为 schema 生成创建一个合适实例。
// 当 T 的零值是 nil（指针、接口等），通过反射创建底层类型的零值实例。
func instantiateForSchema[T any](zero T) any {
	if any(zero) != nil {
		return zero
	}
	rt := reflect.TypeOf(zero)
	if rt == nil {
		return nil
	}
	if rt.Kind() == reflect.Ptr {
		// T 是指针类型（如 *MyStruct），创建底层 Elem 的零值实例
		return reflect.New(rt.Elem()).Elem().Interface()
	}
	return nil
}

// deserializeArgs 将 map[string]any 参数通过 JSON 往返转换为目标类型。
// 利用 Go JSON 解码器自动处理 float64→int、string→bool 等常见类型转换。
func deserializeArgs(args map[string]any, target any) error {
	if args == nil {
		return nil
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("序列化参数失败: %w", err)
	}
	if err := json.Unmarshal(argsJSON, target); err != nil {
		return fmt.Errorf("参数校验失败: %w", err)
	}
	return nil
}

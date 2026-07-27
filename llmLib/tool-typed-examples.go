// 文件职责：
// - TypedTool 使用示例：展示如何用泛型编程定义生产级工具。
// - 对比：TypedTool vs 传统 JSONSchemaTool vs 旧式 Tool 接口。
// - 包含参数验证 (Validator)、输出格式化 (Formatter) 的使用演示。

package llmlib

import (
	"context"
	"fmt"
)

// ============================================================================
// 示例 1：带输入验证的天气查询工具
// ============================================================================

// WeatherInput 天气查询参数。
// json tag：LLM 传递参数时的字段名
// desc tag：JSON Schema 中的字段描述，帮助 LLM 理解参数含义
// omitempty tag：可选字段，不加入 required 列表
type WeatherInput struct {
	City string `json:"city" desc:"城市名称，如 Beijing、Shanghai"`
	Days int    `json:"days,omitempty" desc:"预报天数，默认 1 天"`
}

// Validate 实现 Validator 接口，在工具调用前自动校验参数。
func (w WeatherInput) Validate() error {
	if w.City == "" {
		return NewAgentError(ErrCategoryValidation, "城市名称不能为空", nil, false)
	}
	if w.Days < 0 || w.Days > 7 {
		return NewAgentError(ErrCategoryValidation, "预报天数必须在 0-7 之间", nil, false)
	}
	return nil
}

// WeatherOutput 天气查询结果，实现 Formatter 接口以提供 LLM 可读的输出。
type WeatherOutput struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
}

// Format 实现 Formatter 接口，返回适合 LLM 阅读的格式化字符串。
func (w WeatherOutput) Format() string {
	return fmt.Sprintf("%s 天气：%s，温度 %.1f°C", w.City, w.Condition, w.Temperature)
}

// NewWeatherTool 创建一个天气查询工具（TypedTool 版本）。
// 只需要定义一个类型化函数，参数 Schema 自动生成，参数解析自动处理。
func NewWeatherTool() *TypedTool[WeatherInput, WeatherOutput] {
	return NewTypedTool(
		"weather",
		"查询指定城市的天气情况，返回天气状况和温度",
		func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
			// 实际项目中使用此函数调用天气 API
			// 这里假设调用了真实的天气服务
			return WeatherOutput{
				City:        in.City,
				Temperature: 26.5,
				Condition:   "晴",
			}, nil
		},
	)
}

// ============================================================================
// 示例 2：无参数工具
// ============================================================================

// NewTimeTool 创建一个获取当前时间的工具（TypedTool 版本）。
// 无参数工具使用 struct{} 作为 TInput，LLM 调用时不需要传任何参数。
func NewTimeTool() *TypedTool[struct{}, string] {
	return NewTypedTool(
		"get_current_time",
		"获取当前系统时间，返回 RFC3339 格式的时间字符串",
		func(ctx context.Context, _ struct{}) (string, error) {
			return "2026-07-28T10:30:00+08:00", nil
		},
	)
}

// ============================================================================
// 示例 3：嵌套结构体 — 数据库查询工具
// ============================================================================

// QueryFilter 数据库查询过滤条件。
type QueryFilter struct {
	Field    string `json:"field" desc:"过滤字段名"`
	Operator string `json:"operator" desc:"比较操作符: eq, gt, lt, like"`
	Value    string `json:"value" desc:"过滤值"`
}

// DBQueryInput 数据库查询参数，展示嵌套结构体的 Schema 自动生成。
type DBQueryInput struct {
	Table   string        `json:"table" desc:"数据表名称"`
	Columns []string      `json:"columns" desc:"要查询的列名列表"`
	Filters []QueryFilter `json:"filters,omitempty" desc:"过滤条件列表"`
	Limit   int           `json:"limit,omitempty" desc:"返回记录数，默认 10"`
}

// Validate 实现参数校验：表名必填、Limit 范围检查。
func (d DBQueryInput) Validate() error {
	if d.Table == "" {
		return NewAgentError(ErrCategoryValidation, "表名不能为空", nil, false)
	}
	if d.Limit < 0 || d.Limit > 100 {
		return NewAgentError(ErrCategoryValidation, "记录数必须在 0-100 之间", nil, false)
	}
	return nil
}

// QueryResult 查询结果。
type QueryResult struct {
	Rows  []map[string]any `json:"rows"`
	Total int              `json:"total"`
}

// Format 返回摘要信息供 LLM 阅读。
func (q QueryResult) Format() string {
	return fmt.Sprintf("%d 条记录", q.Total)
}

// NewDBQueryTool 创建一个数据库查询工具，展示嵌套结构体的 Schema 自动生成。
func NewDBQueryTool() *TypedTool[DBQueryInput, QueryResult] {
	return NewTypedTool(
		"db_query",
		"查询数据库表，支持字段过滤和列选择",
		func(ctx context.Context, in DBQueryInput) (QueryResult, error) {
			// 实际项目中使用此函数执行 SQL 查询
			return QueryResult{Rows: nil, Total: 0}, nil
		},
	)
}

// ============================================================================
// 示例 4：多工具注册 — 完整的初始化和注册流程
// ============================================================================

// SetupTools 演示如何使用 TypedTool 批量注册工具到 Registry。
func SetupTools() *Registry {
	reg := NewRegistryToolSet()

	// 注册三个 TypedTool 到同一 Registry
	RegisterTyped(reg, NewWeatherTool())
	RegisterTyped(reg, NewTimeTool())
	RegisterTyped(reg, NewDBQueryTool())

	// 与旧式工具混用
	reg.Register(&CalculatorTool{})
	reg.Register(&TimeTool{})

	// ToolDefs() 自动检测 SchemaTool，优先使用 JSON Schema
	// 生成的 ToolDef 可传给任何 Provider.ChatStreamWithTools
	_ = reg.ToolDefs()

	return reg
}

// ============================================================================
// 对比：TypedTool vs 旧式写法
// ============================================================================

// TypedTool 写法（新，推荐）：
//
//	type MyInput struct {
//	    Name string `json:"name" desc:"名称"`
//	}
//	tool := NewTypedTool("my_tool", "描述", func(ctx, MyInput) (string, error) { ... })
//	reg.Register(tool)  // 6 行代码，类型安全
//
// JSONSchemaTool 写法（旧，手写 Schema）：
//
//	schema := `{"type":"object","properties":{"name":{"type":"string"}}}`
//	tool := NewJSONSchemaTool("my_tool", "描述", []byte(schema),
//	    func(ctx, args) (any, error) {
//	        name, _ := args["name"].(string)  // 需要手动类型断言
//	        ...
//	    })
//	reg.Register(tool)  // Schema 手写、参数手动断言
//
// Tool 接口写法（最旧，map[string]string 参数）：
//
//	type MyTool struct{}
//	func (m MyTool) Parameters() map[string]string { return map[string]string{"name": "string, 名称"} }
//	func (m MyTool) Call(ctx, args) (any, error) {
//	    name, _ := args["name"].(string)  // 需要手动类型断言
//	    ...
//	}
//	reg.Register(&MyTool{})  // 写死参数类型、容易出错

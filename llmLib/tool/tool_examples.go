// 文件职责：
// - 提供基于 JSON Schema 的 Tool 接口实现示例（Calculator、Time）。
// - 同时保留旧的参数接口供向后兼容。
// - 推荐使用 NewTool 创建新工具，自动将 map[string]string 转为 JSON Schema。

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/core"
)

// ============================================================================
// JSONSchemaTool — 使用 JSON Schema 定义参数的标准 Tool 实现
// ============================================================================

// JSONSchemaTool 是使用 JSON Schema 定义参数的 Tool 实现。
// Parameters() 返回 json.RawMessage（即 JSON Schema），由 ToolDefs() 直接使用。
type JSONSchemaTool struct {
	name        string
	description string
	parameters  json.RawMessage
	callFn      func(ctx context.Context, args map[string]any) (any, error)
}

// Name 返回工具名称。
func (t *JSONSchemaTool) Name() string { return t.name }

// Description 返回工具描述。
func (t *JSONSchemaTool) Description() string { return t.description }

// Parameters 返回 JSON Schema 格式的参数定义。
func (t *JSONSchemaTool) Parameters() map[string]string { return nil }

// ParametersSchema 返回 JSON Schema 格式的参数定义（新接口）。
func (t *JSONSchemaTool) ParametersSchema() json.RawMessage { return t.parameters }

// Call 执行工具调用。
func (t *JSONSchemaTool) Call(ctx context.Context, args map[string]any) (any, error) {
	return t.callFn(ctx, args)
}

// SchemaTool 接口：支持 JSON Schema 参数定义的工具。
type SchemaTool interface {
	Tool
	ParametersSchema() json.RawMessage
}

// NewJSONSchemaTool 创建一个使用 JSON Schema 定义参数的工具。
// name: 工具名称
// description: 工具描述
// parametersJSON: JSON Schema 格式的参数定义
// callFn: 工具执行函数
func NewJSONSchemaTool(name, description string, parametersJSON json.RawMessage, callFn func(ctx context.Context, args map[string]any) (any, error)) *JSONSchemaTool {
	return &JSONSchemaTool{
		name:        name,
		description: description,
		parameters:  parametersJSON,
		callFn:      callFn,
	}
}

// ============================================================================
// ToolDefs — 改进版，自动检测 SchemaTool
// ============================================================================

// BuildToolDefs 从 Registry 构建 ToolDef 列表，自动检测 SchemaTool 接口。
// 优先使用 ParametersSchema() 返回的 JSON Schema，fallback 到 Parameters() 的 map[string]string。
// 对 map[string]string 格式的参数，自动转换为合法的 JSON Schema。
func BuildToolDefs(registry *Registry) []core.ToolDef {
	if registry == nil {
		return nil
	}
	var defs []core.ToolDef
	for _, tool := range registry.Tools {
		var params json.RawMessage
		if st, ok := tool.(SchemaTool); ok {
			params = st.ParametersSchema()
		} else {
			params = mapToJSONSchema(tool.Parameters())
		}
		defs = append(defs, core.ToolDef{
			Type: "function",
			Function: core.ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}
	return defs
}

// mapToJSONSchema 将 map[string]string 格式的参数定义转换为合法的 JSON Schema。
// 输入格式：{"expression": "string, 数学表达式"}
// 输出格式：{"type":"object","properties":{"expression":{"type":"string","description":"数学表达式"}},"required":["expression"]}
func mapToJSONSchema(params map[string]string) json.RawMessage {
	properties := make(map[string]any)
	required := make([]string, 0, len(params))

	for name, desc := range params {
		// 解析类型和描述：格式为 "type, description" 或 "type"
		propType := "string"
		propDesc := desc

		if idx := strings.Index(desc, ","); idx > 0 {
			propType = strings.TrimSpace(desc[:idx])
			propDesc = strings.TrimSpace(desc[idx+1:])
		}

		properties[name] = map[string]any{
			"type":        propType,
			"description": propDesc,
		}
		required = append(required, name)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	result, _ := json.Marshal(schema)
	return result
}

// ============================================================================
// CalculatorTool — 计算器工具
// ============================================================================

type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
	return "calculator"
}

func (t *CalculatorTool) Description() string {
	return "执行数学运算，支持加减乘除"
}

func (t *CalculatorTool) Parameters() map[string]string {
	return map[string]string{
		"expression": "string, 数学表达式，如 \"2+3*4\"",
	}
}

func (t *CalculatorTool) Call(ctx context.Context, args map[string]any) (any, error) {
	expr, ok := args["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少 expression 参数")
	}
	result, err := evaluateExpression(expr)
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("计算结果: %v", result), nil
}

func evaluateExpression(expr string) (float64, error) {
	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty expression")
	}
	return parseExpression(tokens)
}

func tokenize(expr string) []string {
	var tokens []string
	var num strings.Builder
	for _, ch := range expr {
		switch {
		case ch == '+' || ch == '-' || ch == '*' || ch == '/':
			if num.Len() > 0 {
				tokens = append(tokens, num.String())
				num.Reset()
			}
			tokens = append(tokens, string(ch))
		case ch == '(' || ch == ')':
			if num.Len() > 0 {
				tokens = append(tokens, num.String())
				num.Reset()
			}
			tokens = append(tokens, string(ch))
		case ch >= '0' && ch <= '9' || ch == '.':
			num.WriteRune(ch)
		case ch == ' ':
			continue
		default:
			return nil
		}
	}
	if num.Len() > 0 {
		tokens = append(tokens, num.String())
	}
	return tokens
}

func parseExpression(tokens []string) (float64, error) {
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty expression")
	}
	result, _, err := parseAdditive(tokens, 0)
	return result, err
}

func parseAdditive(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parseMultiplicative(tokens, pos)
	if err != nil {
		return 0, pos, err
	}

	for pos < len(tokens) {
		token := tokens[pos]
		if token != "+" && token != "-" {
			break
		}
		pos++
		right, pos, err := parseMultiplicative(tokens, pos)
		if err != nil {
			return 0, pos, err
		}
		if token == "+" {
			left += right
		} else {
			left -= right
		}
	}
	return left, pos, nil
}

func parseMultiplicative(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parsePrimary(tokens, pos)
	if err != nil {
		return 0, pos, err
	}

	for pos < len(tokens) {
		token := tokens[pos]
		if token != "*" && token != "/" {
			break
		}
		pos++
		right, pos, err := parsePrimary(tokens, pos)
		if err != nil {
			return 0, pos, err
		}
		if token == "*" {
			left *= right
		} else {
			if right == 0 {
				return 0, pos, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, pos, nil
}

func parsePrimary(tokens []string, pos int) (float64, int, error) {
	if pos >= len(tokens) {
		return 0, pos, fmt.Errorf("unexpected end of expression")
	}
	token := tokens[pos]

	if token == "(" {
		pos++
		result, pos, err := parseAdditive(tokens, pos)
		if err != nil {
			return 0, pos, err
		}
		if pos >= len(tokens) || tokens[pos] != ")" {
			return 0, pos, fmt.Errorf("mismatched parentheses")
		}
		pos++
		return result, pos, nil
	}

	num, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, pos, err
	}
	pos++
	return num, pos, nil
}

// ============================================================================
// TimeTool — 时间工具
// ============================================================================

type TimeTool struct{}

func (t *TimeTool) Name() string {
	return "get_current_time"
}

func (t *TimeTool) Description() string {
	return "获取当前时间"
}

func (t *TimeTool) Parameters() map[string]string {
	return map[string]string{}
}

func (t *TimeTool) Call(ctx context.Context, args map[string]any) (any, error) {
	return time.Now().Format(time.RFC3339), nil
}

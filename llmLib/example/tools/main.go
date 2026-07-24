package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

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
	result, err := evaluate(expr)
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("计算结果: %v", result), nil
}

func evaluate(expr string) (float64, error) {
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

func precedence(op string) int {
	switch op {
	case "*", "/":
		return 2
	case "+", "-":
		return 1
	default:
		return 0
	}
}

func applyOperator(values *[]float64, op string) error {
	if len(*values) < 2 {
		return fmt.Errorf("not enough operands")
	}
	b := (*values)[len(*values)-1]
	*values = (*values)[:len(*values)-1]
	a := (*values)[len(*values)-1]
	*values = (*values)[:len(*values)-1]

	var result float64
	switch op {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return fmt.Errorf("division by zero")
		}
		result = a / b
	default:
		return fmt.Errorf("unknown operator: %s", op)
	}
	*values = append(*values, result)
	return nil
}

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

func main() {
	providerName := llmlib.ProviderDoubao
	apiKey := os.Getenv(llmlib.DOUBAO_API_KEY)
	if apiKey == "" {
		fmt.Println("请设置 DOUBAO_API_KEY 环境变量")
		return
	}
	baseURL := os.Getenv(llmlib.DOUBAO_BASE_URL)
	if baseURL == "" {
		baseURL = llmlib.DOUBAO_BASEURL
	}

	registry := llmlib.NewRegistryToolSet()
	registry.Register(&CalculatorTool{})
	registry.Register(&TimeTool{})

	p, err := llmlib.NewProvider(providerName)
	if err != nil {
		fmt.Printf("创建 provider 失败: %v\n", err)
		return
	}

	tcpr, ok := p.(llmlib.ToolCallProvider)
	if !ok {
		fmt.Println("provider 不支持工具调用")
		return
	}

	messages := []llmlib.Message{
		llmlib.NewUserMessage("计算 2*(3+5) 的结果"),
	}

	resp, err := tcpr.ChatWithTools(context.Background(), llmlib.LLMConfig{APIKey: apiKey, BaseURL: baseURL, Model: llmlib.DOUBAO_DEFAULT_MODEL}, messages, registry.ToolDefs())
	if err != nil {
		fmt.Printf("工具调用失败: %v\n", err)
		return
	}

	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			fmt.Printf("工具调用: %s, 参数: %s\n", tc.Name, string(tc.Args))
			var args map[string]any
			if err := json.Unmarshal(tc.Args, &args); err != nil {
				var argsStr string
				if json.Unmarshal(tc.Args, &argsStr) == nil {
					if json.Unmarshal([]byte(argsStr), &args) != nil {
						fmt.Printf("参数解析失败: %v\n", err)
						continue
					}
				} else {
					fmt.Printf("参数解析失败: %v\n", err)
					continue
				}
			}
			result, err := registry.Call(context.Background(), tc.Name, args)
			if err != nil {
				fmt.Printf("工具执行失败: %v\n", err)
				continue
			}
			fmt.Printf("工具结果: %v\n", result)
		}
	} else {
		fmt.Printf("响应内容: %s\n", resp.Content)
	}
}

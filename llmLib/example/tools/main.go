package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib"
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

func (t *CalculatorTool) Call(ctx context.Context, args map[string]interface{}) (interface{}, error) {
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

	values := make([]float64, 0)
	operators := make([]string, 0)

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "+" || token == "-" || token == "*" || token == "/" {
			for len(operators) > 0 && precedence(operators[len(operators)-1]) >= precedence(token) {
				if err := applyOperator(&values, operators[len(operators)-1]); err != nil {
					return 0, err
				}
				operators = operators[:len(operators)-1]
			}
			operators = append(operators, token)
		} else {
			num, err := strconv.ParseFloat(token, 64)
			if err != nil {
				return 0, err
			}
			values = append(values, num)
		}
	}

	for len(operators) > 0 {
		if err := applyOperator(&values, operators[len(operators)-1]); err != nil {
			return 0, err
		}
		operators = operators[:len(operators)-1]
	}

	if len(values) != 1 {
		return 0, fmt.Errorf("invalid expression")
	}
	return values[0], nil
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

func (t *TimeTool) Call(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return time.Now().Format(time.RFC3339), nil
}

func main() {
	providerName := "doubao"
	apiKey := os.Getenv("DOUBAO_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置 DOUBAO_API_KEY 环境变量")
		return
	}

	registry := llmlib.NewRegistry()
	registry.Register(&CalculatorTool{})
	registry.Register(&TimeTool{})

	p, err := llmlib.NewProvider(providerName)
	if err != nil {
		fmt.Printf("创建 provider 失败: %v\n", err)
		return
	}

	tcp, ok := p.(llmlib.ToolCallProvider)
	if !ok {
		fmt.Println("provider 不支持工具调用")
		return
	}

	messages := []llmlib.Message{
		llmlib.NewUserMessage("计算 2*(3+5) 的结果"),
	}

	resp, err := tcp.ChatWithTools(context.Background(), llmlib.LLMConfig{APIKey: apiKey}, messages, registry.ToolDefs())
	if err != nil {
		fmt.Printf("工具调用失败: %v\n", err)
		return
	}

	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			fmt.Printf("工具调用: %s, 参数: %s\n", tc.Name, string(tc.Args))
			var args map[string]interface{}
			if err := json.Unmarshal(tc.Args, &args); err != nil {
				fmt.Printf("参数解析失败: %v\n", err)
				continue
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
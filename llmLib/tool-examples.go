package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
	return "calculator"
}

func (t *CalculatorTool) Description() string {
	return "执行数学运算，支持加减乘除"
}

func (t *CalculatorTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"expression": {
				"type": "string",
				"description": "数学表达式，如 \"2+3*4\""
			}
		},
		"required": ["expression"]
	}`)
}

func (t *CalculatorTool) Call(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	result, err := evaluateExpression(params.Expression)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("计算结果: %v", result), nil
}

func evaluateExpression(expr string) (float64, error) {
	return evalSimpleExpression(expr)
}

func evalSimpleExpression(expr string) (float64, error) {
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

func (t *TimeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`)
}

func (t *TimeTool) Call(ctx context.Context, args json.RawMessage) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}
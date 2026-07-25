package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

// CalculatorTool 数学计算工具
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string { return "calculator" }
func (t *CalculatorTool) Description() string {
	return "执行数学运算，支持加减乘除和括号"
}
func (t *CalculatorTool) Parameters() map[string]string {
	return map[string]string{"expression": "string, 数学表达式，如 \"3+5*2\""}
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

// TimeTool 获取当前时间工具
type TimeTool struct{}

func (t *TimeTool) Name() string        { return "get_current_time" }
func (t *TimeTool) Description() string { return "获取当前日期和时间" }
func (t *TimeTool) Parameters() map[string]string {
	return map[string]string{}
}
func (t *TimeTool) Call(ctx context.Context, args map[string]any) (any, error) {
	return time.Now().Format("2006-01-02 15:04:05"), nil
}

// 表达式求值器（递归下降，支持括号和正确优先级）
func evaluate(expr string) (float64, error) {
	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty expression")
	}
	result, _, err := parseExpr(tokens, 0)
	return result, err
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

func parseExpr(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parseTerm(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) {
		op := tokens[pos]
		if op != "+" && op != "-" {
			break
		}
		pos++
		right, nextPos, err := parseTerm(tokens, pos)
		if err != nil {
			return 0, nextPos, err
		}
		if op == "+" {
			left += right
		} else {
			left -= right
		}
		pos = nextPos
	}
	return left, pos, nil
}

func parseTerm(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parseFactor(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) {
		op := tokens[pos]
		if op != "*" && op != "/" {
			break
		}
		pos++
		right, nextPos, err := parseFactor(tokens, pos)
		if err != nil {
			return 0, nextPos, err
		}
		if op == "*" {
			left *= right
		} else {
			if right == 0 {
				return 0, nextPos, fmt.Errorf("division by zero")
			}
			left /= right
		}
		pos = nextPos
	}
	return left, pos, nil
}

func parseFactor(tokens []string, pos int) (float64, int, error) {
	if pos >= len(tokens) {
		return 0, pos, fmt.Errorf("unexpected end of expression")
	}
	if tokens[pos] == "(" {
		pos++
		result, nextPos, err := parseExpr(tokens, pos)
		if err != nil {
			return 0, nextPos, err
		}
		if nextPos >= len(tokens) || tokens[nextPos] != ")" {
			return 0, nextPos, fmt.Errorf("mismatched parentheses")
		}
		return result, nextPos + 1, nil
	}
	num, err := strconv.ParseFloat(tokens[pos], 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid number: %s", tokens[pos])
	}
	return num, pos + 1, nil
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
	modelName := llmlib.DOUBAO_DEFAULT_MODEL

	p, err := llmlib.NewProvider(providerName)
	if err != nil {
		fmt.Printf("创建 provider 失败: %v\n", err)
		return
	}

	toolSet := llmlib.NewRegistryToolSet()
	toolSet.Register(&CalculatorTool{})
	toolSet.Register(&TimeTool{})

	budget := llmlib.DefaultAgentBudgetConfig()
	budget.MaxSteps = 10

	agent := llmlib.New(p, modelName, toolSet,
		llmlib.WithSystemPrompt("你是一个严格按用户目标执行任务的 AI 助手。如果目标包含计算和查询时间等多个子任务，必须分别调用 calculator 和 get_current_time 工具获取真实结果，然后汇总给出最终答案。"),
		llmlib.WithAgentBudgetConfig(budget),
		llmlib.WithAgentAPIKey(apiKey),
		llmlib.WithAgentBaseURL(baseURL),
	)

	goal := "计算 3+5*2 的结果，然后告诉我现在的时间"

	fmt.Printf("Agent 目标: %s\n", goal)
	fmt.Println("--- 事件流 ---")

	events, err := agent.Run(context.Background(), goal)
	if err != nil {
		fmt.Printf("Agent 启动失败: %v\n", err)
		return
	}

	for event := range events {
		switch event.Type {
		case llmlib.EventStepStart:
			fmt.Printf("[Step %d 开始] %s\n", event.Step, event.Text)
		case llmlib.EventStepEnd:
			fmt.Printf("[Step %d 结束] %s\n", event.Step, event.Text)

		case llmlib.EventModelCall:
			fmt.Printf("[模型调用] %s\n", event.Text)
		case llmlib.EventModelResponse:
			fmt.Printf("[模型返回] %s\n", event.Text)

		case llmlib.EventThought:
			fmt.Printf("[思考] %s\n", event.Text)
		case llmlib.EventAnswer:
			fmt.Printf("[答案] %s\n", event.Text)

		case llmlib.EventToolCall:
			fmt.Printf("[工具调用] %s(%s)\n", event.Tool, event.Args)
		case llmlib.EventToolResult:
			fmt.Printf("[工具结果] %s: %s\n", event.Tool, event.Text)

		case llmlib.EventError:
			fmt.Printf("[错误] %s\n", event.Text)
		case llmlib.EventDone:
			fmt.Printf("[完成] Step %d\n", event.Step)
		}
	}
}

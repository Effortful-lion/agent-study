package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
)

const maxRounds = 3

func main() {
	if err := llmlib.LoadDotEnv(); err != nil {
		fmt.Printf("加载 .env 失败: %v\n", err)
	}

	providerName := llmlib.ProviderDoubao
	apiKey := os.Getenv(llmlib.DOUBAO_API_KEY)
	if apiKey == "" {
		apiKey = os.Getenv(llmlib.API_KEY)
	}
	if apiKey == "" {
		fmt.Println("请设置 API_KEY 或 DOUBAO_API_KEY 环境变量")
		return
	}

	baseURL := os.Getenv(llmlib.DOUBAO_BASE_URL)
	if baseURL == "" {
		baseURL = os.Getenv(llmlib.BASE_URL)
	}
	if baseURL == "" {
		baseURL = llmlib.DOUBAO_BASEURL
	}

	model := os.Getenv(llmlib.DOUBAO_MODEL_ENV)
	if model == "" {
		model = os.Getenv(llmlib.MODEL)
	}
	if model == "" {
		model = llmlib.DOUBAO_DEFAULT_MODEL
	}

	p, err := llmlib.NewProvider(providerName)
	if err != nil {
		fmt.Printf("创建 provider 失败: %v\n", err)
		return
	}

	cfg := llmlib.LLMConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}

	ctx := context.Background()

	code := sampleGoCode()

	fmt.Println("=== 代码审查开始 ===")
	fmt.Println()
	fmt.Println("待审查代码:")
	fmt.Println(code)
	fmt.Println()

	intent, err := detectIntent(ctx, p, cfg, code)
	if err != nil {
		fmt.Printf("意图检测失败: %v\n", err)
		return
	}

	if !intent.IsCode {
		fmt.Printf("输入不是 Go 代码: %s\n", intent.Reason)
		return
	}

	fmt.Println("--- 步骤 1: Planner - 决定审查维度 ---")

	plan, err := planReview(ctx, p, cfg, code)
	if err != nil {
		fmt.Printf("Planner 失败: %v\n", err)
		return
	}

	fmt.Printf("确定审查维度: %s\n", strings.Join(plan.Dimensions, ", "))
	fmt.Println()

	var finalResults reviewResults
	var finalScore float64

	for round := 1; round <= maxRounds; round++ {
		fmt.Printf("--- 步骤 2: Generator - 并行审查 (Round %d/%d) ---\n", round, maxRounds)

		results, err := generateReviews(ctx, p, cfg, code, plan.Dimensions)
		if err != nil {
			fmt.Printf("Generator 失败: %v\n", err)
			return
		}

		for _, finding := range results.Findings {
			fmt.Printf("  - %s: %d 个问题\n", finding.Dimension, len(finding.Issues))
		}
		fmt.Println()

		fmt.Println("--- 步骤 3: Evaluator - 评估报告 ---")

		evalResult, err := evaluateReport(ctx, p, cfg, code, results)
		if err != nil {
			fmt.Printf("Evaluator 失败: %v\n", err)
			return
		}

		fmt.Printf("评分: %.2f/1.0\n", evalResult.Score)
		fmt.Printf("反馈: %s\n", evalResult.Feedback)
		fmt.Println()

		finalResults = results
		finalScore = evalResult.Score

		if evalResult.Score >= 0.8 {
			fmt.Println("✓ 报告质量达标，结束审查")
			break
		}

		fmt.Println("✗ 报告需要补充，进行下一轮审查")
		fmt.Println()

		if round < maxRounds {
			newPlan, err := refinePlan(ctx, p, cfg, plan, evalResult.Feedback)
			if err != nil {
				fmt.Printf("优化维度失败: %v\n", err)
				return
			}
			plan = newPlan
			fmt.Printf("优化后的审查维度: %s\n", strings.Join(plan.Dimensions, ", "))
			fmt.Println()
		}
	}

	fmt.Println("=== 审查报告 ===")
	fmt.Println()
	printReport(finalResults)

	fmt.Println()
	fmt.Printf("最终评分: %.2f/1.0\n", finalScore)
}

func refinePlan(ctx context.Context, p llmlib.Provider, cfg llmlib.LLMConfig, plan reviewPlan, feedback string) (reviewPlan, error) {
	system := `基于评估反馈优化审查维度清单。
输出格式：只输出 JSON {"dimensions":["...","..."]}

原有维度: ` + strings.Join(plan.Dimensions, ", ")

	messages := []llmlib.Message{
		llmlib.NewSystemMessage(system),
		llmlib.NewUserMessage(feedback),
	}

	out, err := p.Chat(ctx, cfg, messages)
	if err != nil {
		return reviewPlan{}, err
	}

	return parseReviewPlan(out.Content)
}

func printReport(results reviewResults) {
	for _, finding := range results.Findings {
		fmt.Printf("## %s\n", finding.Dimension)
		fmt.Println()

		if len(finding.Issues) == 0 {
			fmt.Println("  - 未发现问题")
			fmt.Println()
			continue
		}

		for i, issue := range finding.Issues {
			fmt.Printf("  %d. **[%s]** %s\n", i+1, issue.Severity, issue.Description)
			if issue.Location != "" {
				fmt.Printf("     位置: %s\n", issue.Location)
			}
			if issue.Suggestion != "" {
				fmt.Printf("     建议: %s\n", issue.Suggestion)
			}
			fmt.Println()
		}
	}

	jsonOutput, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println("--- JSON 输出 ---")
	fmt.Println(string(jsonOutput))
}

func sampleGoCode() string {
	return `package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	mu    sync.Mutex
	count int
}

func (c *Counter) Inc() {
	c.count++
}

func (c *Counter) Get() int {
	return c.count
}

func main() {
	c := &Counter{}
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}

	wg.Wait()
	fmt.Println("Count:", c.Get())
}`
}
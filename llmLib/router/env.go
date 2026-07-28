package router

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/provider"
)

type providerMeta struct {
	name         string
	apiKeyEnv    string
	baseURLEnv   string
	modelEnv     string
	defaultModel string
	defaultURL   string
	inputPrice   float64
	outputPrice  float64
}

var providerMetas = []providerMeta{
	{core.ProviderDoubao, core.DOUBAO_API_KEY, core.DOUBAO_BASE_URL, core.DOUBAO_MODEL_ENV, core.DOUBAO_DEFAULT_MODEL, core.DOUBAO_BASEURL, 0.002, 0.004},
	{core.ProviderDeepSeek, core.DEEPSEEK_API_KEY, core.DEEPSEEK_BASE_URL, core.DEEPSEEK_MODEL_ENV, core.DEEPSEEK_DEFAULT_MODEL, core.DEEPSEEK_BASEURL, 0.001, 0.002},
	{core.ProviderZhipu, core.ZHIPU_API_KEY, core.ZHIPU_BASE_URL, core.ZHIPU_MODEL_ENV, core.ZHIPU_DEFAULT_MODEL, core.ZHIPU_BASEURL, 0.001, 0.002},
	{core.ProviderTongyi, core.TONGYI_API_KEY, core.TONGYI_BASE_URL, core.TONGYI_MODEL_ENV, core.TONGYI_DEFAULT_MODEL, core.TONGYI_BASEURL, 0.002, 0.004},
	{core.ProviderKimi, core.KIMI_API_KEY, core.KIMI_BASE_URL, core.KIMI_MODEL_ENV, core.KIMI_DEFAULT_MODEL, core.KIMI_BASEURL, 0.002, 0.004},
	{core.ProviderClaude, core.CLAUDE_API_KEY, core.CLAUDE_BASE_URL, core.CLAUDE_MODEL_ENV, core.CLAUDE_DEFAULT_MODEL, core.CLAUDE_BASEURL, 0.003, 0.015},
	{core.ProviderOpenAI, core.OPENAI_API_KEY, core.OPENAI_BASE_URL, core.OPENAI_MODEL_ENV, core.OPENAI_DEFAULT_MODEL, core.OPENAI_BASEURL, 0.003, 0.015},
	{core.ProviderQwen, core.QWEN_API_KEY, core.QWEN_BASE_URL, core.QWEN_MODEL_ENV, core.QWEN_DEFAULT_MODEL, core.QWEN_BASEURL, 0.002, 0.004},
}

func LoadAll() ([]LLMService, error) {
	return loadAllFromEnv()
}

func LoadAllWithEnv(envPath string) ([]LLMService, error) {
	if err := loadDotEnvFile(envPath); err != nil {
		return nil, err
	}
	return loadAllFromEnv()
}

func LoadDotEnv() error {
	return loadDotEnvFile(".env")
}

func LoadDotEnvFromPath(path string) error {
	return loadDotEnvFile(path)
}

func loadDotEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	for _, line := range splitLines(string(data)) {
		if line == "" || line[0] == '#' {
			continue
		}
		for i := 0; i < len(line); i++ {
			if line[i] == '=' {
				key := trimSpace(line[:i])
				val := trimSpace(line[i+1:])
				if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"') {
					val = val[1 : len(val)-1]
				}
				os.Setenv(key, val)
				break
			}
		}
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func loadAllFromEnv() ([]LLMService, error) {
	var services []LLMService

	for _, meta := range providerMetas {
		apiKey := os.Getenv(meta.apiKeyEnv)
		if apiKey == "" {
			continue
		}

		baseURL := os.Getenv(meta.baseURLEnv)
		if baseURL == "" {
			baseURL = meta.defaultURL
		}

		model := os.Getenv(meta.modelEnv)
		if model == "" {
			model = meta.defaultModel
		}

		p, err := provider.NewProvider(meta.name)
		if err != nil {
			continue
		}

		cfg := core.LLMConfig{
			BaseURL:               baseURL,
			APIKey:                apiKey,
			Model:                 model,
			InputPricePerMillion:  meta.inputPrice,
			OutputPricePerMillion: meta.outputPrice,
		}

		services = append(services, LLMService{
			Provider: p,
			Config:   cfg,
		})
	}

	return services, nil
}

func ReadStrategyFromEnv() Strategy {
	s := os.Getenv("LLM_ROUTER_STRATEGY")
	switch s {
	case "cheapest":
		return StrategyCheapestFirst
	case "latency":
		return StrategyLowestLatency
	default:
		return StrategyDefault
	}
}

func ProviderConfigHelp() string {
	help := "环境变量配置说明：\n"
	help += "  LLM_ROUTER_STRATEGY=cheapest|latency|default  # 路由策略\n\n"
	help += "支持的 Provider 环境变量：\n"
	for _, meta := range providerMetas {
		help += fmt.Sprintf("  %s  - %s 模型 (默认: %s)\n", meta.apiKeyEnv, meta.name, meta.defaultModel)
		help += fmt.Sprintf("    %s  - 自定义地址 (默认: %s)\n", meta.baseURLEnv, meta.defaultURL)
		help += fmt.Sprintf("    %s  - 自定义模型 (默认: %s)\n", meta.modelEnv, meta.defaultModel)
	}
	help += "\n示例：\n"
	help += "  export DOUBAO_API_KEY=xxx\n"
	help += "  export DEEPSEEK_API_KEY=xxx\n"
	help += "  LLM_ROUTER_STRATEGY=cheapest go run .\n"
	return help
}

var _ = strconv.Itoa
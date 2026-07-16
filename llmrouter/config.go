package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type LLMConfig struct {
	BaseURL               string
	APIKey                string
	Model                 string
	InputPricePerMillion  float64
	OutputPricePerMillion float64
	LatencyMS             int
}

type LLMService struct {
	Provider Provider
	Config   LLMConfig
}

type providerEnv struct {
	Name           string
	APIKeyEnv      string
	BaseURLEnv     string
	ModelEnv       string
	DefaultBaseURL string
	DefaultModel   string
	InputPrice     float64
	OutputPrice    float64
	LatencyMS      int
}

var supportedProviders = []providerEnv{
	{
		Name:           "doubao",
		APIKeyEnv:      "DOUBAO_API_KEY",
		BaseURLEnv:     "DOUBAO_BASE_URL",
		ModelEnv:       "DOUBAO_MODEL",
		DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		DefaultModel:   "doubao-seed-2-0-code-preview-260215",
		InputPrice:     0.20,
		OutputPrice:    0.80,
		LatencyMS:      300,
	},
	{
		Name:           "deepseek",
		APIKeyEnv:      "DEEPSEEK_API_KEY",
		BaseURLEnv:     "DEEPSEEK_BASE_URL",
		ModelEnv:       "DEEPSEEK_MODEL",
		DefaultBaseURL: "https://api.deepseek.com",
		DefaultModel:   "deepseek-chat",
		InputPrice:     0.27,
		OutputPrice:    1.10,
		LatencyMS:      500,
	},
	{
		Name:           "claude",
		APIKeyEnv:      "CLAUDE_API_KEY",
		BaseURLEnv:     "CLAUDE_BASE_URL",
		ModelEnv:       "CLAUDE_MODEL",
		DefaultBaseURL: "https://api.anthropic.com",
		DefaultModel:   "claude-3-5-sonnet-latest",
		InputPrice:     3.00,
		OutputPrice:    15.00,
		LatencyMS:      800,
	},
}

// BuildAll 通过环境变量批量初始化已配置的模型服务商。
//
// 可选环境变量：
//   - LLM_PROVIDERS=doubao,deepseek 指定加载顺序；不设置时按内置顺序加载全部支持的服务商
//   - DOUBAO_API_KEY / DOUBAO_BASE_URL / DOUBAO_MODEL
//   - DEEPSEEK_API_KEY / DEEPSEEK_BASE_URL / DEEPSEEK_MODEL
//   - CLAUDE_API_KEY / CLAUDE_BASE_URL / CLAUDE_MODEL
//   - LLM_ROUTING_STRATEGY=default|cheapest_first|lowest_latency 指定路由排序策略
//
// API Key 为空表示该服务商未启用；BaseURL 和 Model 为空时使用默认值。
func BuildAll() ([]LLMService, error) {
	if err := LoadDotEnv(); err != nil {
		return nil, err
	}

	providerNames := loadProviderNames()
	services := make([]LLMService, 0, len(providerNames))
	explicitProviders := strings.TrimSpace(os.Getenv("LLM_PROVIDERS")) != ""
	var missingConfigs []string

	for _, name := range providerNames {
		meta, ok := findProviderEnv(name)
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s\n\n%s", name, providerConfigHelp())
		}

		cfg, ok := loadProviderConfig(meta)
		if !ok {
			if explicitProviders {
				missingConfigs = append(missingConfigs, fmt.Sprintf("provider %q 已启用，但缺少 %s", meta.Name, meta.APIKeyEnv))
			}
			continue
		}

		p, err := NewProvider(meta.Name)
		if err != nil {
			return nil, err
		}
		services = append(services, LLMService{
			Provider: p,
			Config:   cfg,
		})
	}

	if len(missingConfigs) > 0 {
		return nil, fmt.Errorf("%s\n\n%s", strings.Join(missingConfigs, "\n"), providerConfigHelp())
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("未配置任何模型服务商。\n\n%s", providerConfigHelp())
	}
	return services, nil
}

// 加载所有提供商的名字
func loadProviderNames() []string {
	raw := os.Getenv("LLM_PROVIDERS")
	if raw == "" {
		names := make([]string, 0, len(supportedProviders))
		for _, provider := range supportedProviders {
			names = append(names, provider.Name)
		}
		return names
	}

	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func findProviderEnv(name string) (providerEnv, bool) {
	for _, provider := range supportedProviders {
		if provider.Name == name {
			return provider, true
		}
	}
	return providerEnv{}, false
}

func loadProviderConfig(provider providerEnv) (LLMConfig, bool) {
	apiKey := strings.TrimSpace(os.Getenv(provider.APIKeyEnv)) // 必选
	if apiKey == "" {
		return LLMConfig{}, false
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(provider.BaseURLEnv)), "/")
	if baseURL == "" {
		baseURL = provider.DefaultBaseURL
	}

	model := strings.TrimSpace(os.Getenv(provider.ModelEnv))
	if model == "" {
		model = provider.DefaultModel
	}

	return LLMConfig{
		BaseURL:               baseURL,
		APIKey:                apiKey,
		Model:                 model,
		InputPricePerMillion:  provider.InputPrice,
		OutputPricePerMillion: provider.OutputPrice,
		LatencyMS:             loadLatencyMS(provider),
	}, true
}

func loadLatencyMS(provider providerEnv) int {
	raw := strings.TrimSpace(os.Getenv(strings.ToUpper(provider.Name) + "_LATENCY_MS"))
	if raw == "" {
		return provider.LatencyMS
	}

	latency, err := strconv.Atoi(raw)
	if err != nil || latency < 0 {
		return provider.LatencyMS
	}
	return latency
}

func LoadDotEnv() error {
	file, err := os.Open(".env")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func providerConfigHelp() string {
	var b strings.Builder
	b.WriteString("支持的 providers: ")
	for i, provider := range supportedProviders {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(provider.Name)
	}
	b.WriteString("\n\n配置规则：\n")
	b.WriteString("- LLM_PROVIDERS 控制启用顺序，例如：export LLM_PROVIDERS=doubao,deepseek\n")
	b.WriteString("- LLM_ROUTING_STRATEGY 控制调度策略：default、cheapest_first、lowest_latency\n")
	b.WriteString("- *_LATENCY_MS 控制最低延迟策略使用的静态延迟指标\n")
	b.WriteString("- 每个 provider 使用自己独立的 API Key，不能共用 LLM_API_KEY\n\n")
	b.WriteString("各 provider 配置：\n")
	for _, provider := range supportedProviders {
		fmt.Fprintf(&b, "- %s: %s（必填）, %s（可选，默认 %s）, %s（可选，默认 %s）, %s_LATENCY_MS（可选，默认 %d）\n",
			provider.Name,
			provider.APIKeyEnv,
			provider.BaseURLEnv,
			provider.DefaultBaseURL,
			provider.ModelEnv,
			provider.DefaultModel,
			strings.ToUpper(provider.Name),
			provider.LatencyMS,
		)
	}
	b.WriteString("\n总配置示例：\n")
	b.WriteString("export LLM_PROVIDERS=doubao,deepseek\n")
	b.WriteString("export LLM_ROUTING_STRATEGY=cheapest_first\n")
	b.WriteString("export DOUBAO_API_KEY=ark-xxx\n")
	b.WriteString("export DOUBAO_BASE_URL=https://ark.cn-beijing.volces.com/api/v3\n")
	b.WriteString("export DOUBAO_MODEL=doubao-seed-2-0-code-preview-260215\n")
	b.WriteString("export DEEPSEEK_API_KEY=sk-xxx\n")
	b.WriteString("export DEEPSEEK_BASE_URL=https://api.deepseek.com\n")
	b.WriteString("export DEEPSEEK_MODEL=deepseek-chat")
	return b.String()
}

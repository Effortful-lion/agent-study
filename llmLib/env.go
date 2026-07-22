// 文件职责：
// - 从 .env 和进程环境变量装配多个服务商配置。
// - 负责解析启用顺序、默认模型、价格和延迟信息，并生成可路由的服务列表。

package llmlib

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// providerMeta 描述单个服务商在环境变量装配阶段需要的元信息。
type providerMeta struct {
	Name           string  // 服务商名称，最终用于创建 Provider 实例。
	APIKeyEnv      string  // API Key 对应的环境变量名。
	BaseURLEnv     string  // BaseURL 对应的环境变量名，未设置时回退默认值。
	ModelEnv       string  // 模型名称对应的环境变量名，未设置时回退默认值。
	DefaultBaseURL string  // 默认接口地址，用于未显式覆盖的情况。
	DefaultModel   string  // 默认模型名称，用于快速启用 provider。
	InputPrice     float64 // 每百万输入 token 价格，供路由成本估算。
	OutputPrice    float64 // 每百万输出 token 价格，供路由成本估算。
	LatencyMS      int     // 预估延迟毫秒值，供延迟策略排序。
}

// providerMetas 维护内置支持的服务商及其环境变量映射关系。
var providerMetas = []providerMeta{
	{
		Name:           "doubao",
		APIKeyEnv:      "DOUBAO_API_KEY",
		BaseURLEnv:     "DOUBAO_BASE_URL",
		ModelEnv:       "DOUBAO_MODEL",
		DefaultBaseURL: DOUBAO_BASEURL,
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
		DefaultBaseURL: DEEPSEEK_BASEURL,
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
		DefaultBaseURL: CLAUDE_BASEURL,
		DefaultModel:   "claude-3-5-sonnet-latest",
		InputPrice:     3.00,
		OutputPrice:    15.00,
		LatencyMS:      800,
	},
	{
		Name:           "openai",
		APIKeyEnv:      "OPENAI_API_KEY",
		BaseURLEnv:     "OPENAI_BASE_URL",
		ModelEnv:       "OPENAI_MODEL",
		DefaultBaseURL: OPENAI_BASEURL,
		DefaultModel:   "gpt-4o",
		InputPrice:     5.00,
		OutputPrice:    15.00,
		LatencyMS:      600,
	},
	{
		Name:           "zhipu",
		APIKeyEnv:      "ZHIPU_API_KEY",
		BaseURLEnv:     "ZHIPU_BASE_URL",
		ModelEnv:       "ZHIPU_MODEL",
		DefaultBaseURL: ZHIPU_BASEURL,
		DefaultModel:   "glm-4",
		InputPrice:     0.15,
		OutputPrice:    0.60,
		LatencyMS:      400,
	},
	{
		Name:           "tongyi",
		APIKeyEnv:      "TONGYI_API_KEY",
		BaseURLEnv:     "TONGYI_BASE_URL",
		ModelEnv:       "TONGYI_MODEL",
		DefaultBaseURL: TONGYI_BASEURL,
		DefaultModel:   "qwen-plus",
		InputPrice:     0.50,
		OutputPrice:    1.50,
		LatencyMS:      550,
	},
	{
		Name:           "kimi",
		APIKeyEnv:      "KIMI_API_KEY",
		BaseURLEnv:     "KIMI_BASE_URL",
		ModelEnv:       "KIMI_MODEL",
		DefaultBaseURL: KIMI_BASEURL,
		DefaultModel:   "moonshot-v1-8k",
		InputPrice:     0.30,
		OutputPrice:    1.20,
		LatencyMS:      450,
	},
}

// LoadDotEnv 从当前目录的 .env 文件加载环境变量，供本地开发快速启动。
func LoadDotEnv() error {
	return LoadDotEnvFromPath(".env")
}

// LoadDotEnvFromPath 读取指定 .env 文件，并只在变量尚未存在时注入进程环境。
func LoadDotEnvFromPath(path string) error {
	file, err := os.Open(path)
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
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
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
			// 保留已有环境变量优先级，避免 .env 覆盖外部注入的配置。
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// LoadAll 按默认 .env 路径加载所有已配置服务商。
func LoadAll() ([]LLMService, error) {
	return LoadAllWithEnv("")
}

// LoadAllWithEnv 读取环境变量并生成可用于 Router 的服务列表。
func LoadAllWithEnv(envPath string) ([]LLMService, error) {
	if envPath != "" {
		if err := LoadDotEnvFromPath(envPath); err != nil {
			return nil, err
		}
	} else {
		if err := LoadDotEnv(); err != nil {
			return nil, err
		}
	}

	providerNames := loadProviderNames()
	services := make([]LLMService, 0, len(providerNames))
	explicitProviders := strings.TrimSpace(os.Getenv("LLM_PROVIDERS")) != ""
	var missingConfigs []string
	var configuredProviders []string

	for _, name := range providerNames {
		// 先校验 provider 是否为内置支持项，再尝试装配环境变量。
		meta, ok := findProviderMeta(name)
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s\n\n%s", name, ProviderConfigHelp())
		}

		cfg, ok := loadProviderConfig(meta)
		if !ok {
			if explicitProviders {
				missingConfigs = append(missingConfigs, fmt.Sprintf("provider %q 已启用，但缺少 %s", meta.Name, meta.APIKeyEnv))
			}
			continue
		}

		configuredProviders = append(configuredProviders, meta.Name)
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
		return nil, fmt.Errorf("%s\n\n%s", strings.Join(missingConfigs, "\n"), ProviderConfigHelp())
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("未配置任何模型服务商。\n\n%s", ProviderConfigHelp())
	}

	if !explicitProviders && len(configuredProviders) > 0 {
		fmt.Fprintf(os.Stderr, "[llmlib] 警告: 未设置 LLM_PROVIDERS，已自动启用以下已配置的服务商: %s\n", strings.Join(configuredProviders, ", "))
		fmt.Fprintf(os.Stderr, "[llmlib] 建议: 通过 export LLM_PROVIDERS=%s 显式指定服务商顺序\n", strings.Join(configuredProviders, ","))
	}

	return services, nil
}

// loadProviderNames 从 LLM_PROVIDERS 解析服务商顺序，未设置时返回全部内置服务商。
func loadProviderNames() []string {
	raw := os.Getenv("LLM_PROVIDERS")
	if raw == "" {
		names := make([]string, 0, len(providerMetas))
		for _, provider := range providerMetas {
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

// findProviderMeta 根据 provider 名称查找对应的环境变量元信息。
func findProviderMeta(name string) (providerMeta, bool) {
	for _, provider := range providerMetas {
		if provider.Name == name {
			return provider, true
		}
	}
	return providerMeta{}, false
}

// loadProviderConfig 从环境变量装配单个服务商配置，缺少 API Key 时返回未启用。
func loadProviderConfig(provider providerMeta) (LLMConfig, bool) {
	apiKey := strings.TrimSpace(os.Getenv(provider.APIKeyEnv))
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

// loadLatencyMS 读取服务商延迟配置，支持通过环境变量覆盖默认值。
func loadLatencyMS(provider providerMeta) int {
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

// ProviderConfigHelp 返回环境变量配置说明，供错误提示和文档展示场景复用。
func ProviderConfigHelp() string {
	var b strings.Builder
	b.WriteString("支持的 providers: ")
	for i, provider := range providerMetas {
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
	for _, provider := range providerMetas {
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

// ReadStrategyFromEnv 从环境变量读取路由策略，未命中时回退默认策略。
func ReadStrategyFromEnv() Strategy {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_ROUTING_STRATEGY")))
	raw = strings.ReplaceAll(raw, "_", "")
	raw = strings.ReplaceAll(raw, "-", "")

	switch raw {
	case "cheapestfirst":
		return StrategyCheapestFirst
	case "lowestlatency":
		return StrategyLowestLatency
	default:
		return StrategyDefault
	}
}

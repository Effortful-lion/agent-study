package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
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
}

var supportedProviders = []providerEnv{
	{
		Name:           "deepseek",
		APIKeyEnv:      "DEEPSEEK_API_KEY",
		BaseURLEnv:     "DEEPSEEK_BASE_URL",
		ModelEnv:       "DEEPSEEK_MODEL",
		DefaultBaseURL: "https://api.deepseek.com",
		DefaultModel:   "deepseek-chat",
	},
	{
		Name:           "claude",
		APIKeyEnv:      "CLAUDE_API_KEY",
		BaseURLEnv:     "CLAUDE_BASE_URL",
		ModelEnv:       "CLAUDE_MODEL",
		DefaultBaseURL: "https://api.anthropic.com",
		DefaultModel:   "claude-3-5-sonnet-latest",
	},
}

// 读取 LLM_BASE_URL、LLM_API_KEY、LLM_MODEL
func loadConfigFromEnv() (LLMConfig, error) {
	cfg := LLMConfig{
		BaseURL: strings.TrimRight(os.Getenv("LLM_BASE_URL"), "/"),
		APIKey:  os.Getenv("LLM_API_KEY"),
		Model:   os.Getenv("LLM_MODEL"),
	}
	if cfg.BaseURL == "" {
		return cfg, errors.New("请设置 LLM_BASE_URL，例如: export LLM_BASE_URL=https://api.deepseek.com")
	}
	if cfg.APIKey == "" {
		return cfg, errors.New("请设置 LLM_API_KEY，例如: export LLM_API_KEY=sk-xxx")
	}
	if cfg.Model == "" {
		return cfg, errors.New("请设置 LLM_MODEL，例如: export LLM_MODEL=deepseek-chat")
	}
	return cfg, nil
}

// BuildAll 通过环境变量批量初始化已配置的模型服务商。
//
// 可选环境变量：
//   - LLM_PROVIDERS=deepseek,claude 指定加载顺序；不设置时按内置顺序加载全部支持的服务商
//   - DEEPSEEK_API_KEY / DEEPSEEK_BASE_URL / DEEPSEEK_MODEL
//   - CLAUDE_API_KEY / CLAUDE_BASE_URL / CLAUDE_MODEL
//
// API Key 为空表示该服务商未启用；BaseURL 和 Model 为空时使用默认值。
func BuildAll() ([]LLMService, error) {
	providerNames := loadProviderNames()
	services := make([]LLMService, 0, len(providerNames))

	for _, name := range providerNames {
		meta, ok := findProviderEnv(name)
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s", name)
		}

		cfg, ok := loadProviderConfig(meta)
		if !ok {
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

	if len(services) == 0 {
		return nil, errors.New("未配置任何模型服务商，请至少设置 DEEPSEEK_API_KEY 或 CLAUDE_API_KEY")
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
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}, true
}

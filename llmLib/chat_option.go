package llmlib

// ChatOption 用于配置聊天请求的选项
type ChatOption func(*LLMConfig)

// WithModel 设置使用的模型名称
func WithModel(model string) ChatOption {
	return func(c *LLMConfig) {
		c.Model = model
	}
}

// WithBaseURL 设置 API 的基础 URL，覆盖默认值
func WithBaseURL(baseURL string) ChatOption {
	return func(c *LLMConfig) {
		c.BaseURL = baseURL
	}
}

// WithAPIKey 设置 API 密钥
func WithAPIKey(apiKey string) ChatOption {
	return func(c *LLMConfig) {
		c.APIKey = apiKey
	}
}

// WithInputPrice 设置每百万输入 token 的价格
func WithInputPrice(price float64) ChatOption {
	return func(c *LLMConfig) {
		c.InputPricePerMillion = price
	}
}

// WithOutputPrice 设置每百万输出 token 的价格
func WithOutputPrice(price float64) ChatOption {
	return func(c *LLMConfig) {
		c.OutputPricePerMillion = price
	}
}

// WithLatencyMS 设置预估延迟（毫秒）
func WithLatencyMS(latency int) ChatOption {
	return func(c *LLMConfig) {
		c.LatencyMS = latency
	}
}

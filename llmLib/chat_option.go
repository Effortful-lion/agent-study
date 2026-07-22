// 文件职责：
// - 提供函数式选项来补充或覆盖单次聊天调用配置。
// - 供 Chat、ChatStream 等入口在创建 LLMConfig 时逐项应用。

package llmlib

// ChatOption 表示对调用配置的单项补丁函数。
type ChatOption func(*LLMConfig)

// WithModel 为本次调用指定模型名称。
func WithModel(model string) ChatOption {
	return func(c *LLMConfig) {
		c.Model = model
	}
}

// WithBaseURL 覆盖服务商的默认接口地址。
func WithBaseURL(baseURL string) ChatOption {
	return func(c *LLMConfig) {
		c.BaseURL = baseURL
	}
}

// WithAPIKey 为当前请求显式注入 API Key。
func WithAPIKey(apiKey string) ChatOption {
	return func(c *LLMConfig) {
		c.APIKey = apiKey
	}
}

// WithInputPrice 设置输入 token 单价，供路由层成本估算使用。
func WithInputPrice(price float64) ChatOption {
	return func(c *LLMConfig) {
		c.InputPricePerMillion = price
	}
}

// WithOutputPrice 设置输出 token 单价，供路由层成本估算使用。
func WithOutputPrice(price float64) ChatOption {
	return func(c *LLMConfig) {
		c.OutputPricePerMillion = price
	}
}

// WithLatencyMS 设置该服务的预估延迟，供延迟优先策略排序。
func WithLatencyMS(latency int) ChatOption {
	return func(c *LLMConfig) {
		c.LatencyMS = latency
	}
}

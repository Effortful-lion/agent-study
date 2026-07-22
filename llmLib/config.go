package llmlib

// LLMConfig 用于配置 LLM 服务的连接参数
type LLMConfig struct {
	BaseURL               string  // API 基础 URL
	APIKey                string  // API 密钥
	Model                 string  // 使用的模型名称
	InputPricePerMillion  float64 // 每百万输入 token 的价格（元）
	OutputPricePerMillion float64 // 每百万输出 token 的价格（元）
	LatencyMS             int     // 预估延迟（毫秒），用于最低延迟策略
}

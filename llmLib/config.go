// 文件职责：
// - 定义单个模型服务的基础连接配置和路由元数据。
// - 供直接调用、环境变量装配和路由选择流程共享使用。

package llmlib

// LLMConfig 描述一次模型调用所需的连接信息、计费信息和延迟元数据。
type LLMConfig struct {
	BaseURL               string  // 服务入口地址，来自调用参数或环境变量，未传时回退到默认地址。
	APIKey                string  // 服务鉴权密钥，通常来自环境变量或显式配置。
	Model                 string  // 目标模型名称，调用时写入上游请求体。
	InputPricePerMillion  float64 // 每百万输入 token 的单价，供路由层估算成本。
	OutputPricePerMillion float64 // 每百万输出 token 的单价，供路由层估算成本。
	LatencyMS             int     // 预估延迟毫秒值，供最低延迟策略排序使用。
}

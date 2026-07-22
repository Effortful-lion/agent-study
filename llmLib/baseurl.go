// 文件职责：
// - 维护各个 LLM 服务商的默认 API 入口地址。
// - 供配置装配和快捷调用逻辑在未显式传入 BaseURL 时直接复用。

package llmlib

// 默认的 API 基础 URL 常量，作为各服务商未覆盖配置时的回退地址。
const (
	OPENAI_BASEURL   = "https://api.openai.com/v1"                         // OpenAI API
	DOUBAO_BASEURL   = "https://ark.cn-beijing.volces.com/api/v3"          // 火山引擎豆包 API
	DEEPSEEK_BASEURL = "https://api.deepseek.com"                          // DeepSeek API
	ZHIPU_BASEURL    = "https://open.bigmodel.cn/api/paas/v4"              // 智谱 AI API
	TONGYI_BASEURL   = "https://dashscope.aliyuncs.com/compatible-mode/v1" // 阿里云通义 API
	KIMI_BASEURL     = "https://api.moonshot.cn/v1"                        // 月之暗面 Kimi API
	CLAUDE_BASEURL   = "https://api.anthropic.com"                         // Anthropic Claude API
)

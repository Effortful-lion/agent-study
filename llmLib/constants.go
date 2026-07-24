package llmlib

const (
	API_KEY  = "API_KEY"
	BASE_URL = "BASE_URL"
	MODEL    = "MODEL"

	ProviderOpenAI   = "openai"
	ProviderDoubao   = "doubao"
	ProviderDeepSeek = "deepseek"
	ProviderZhipu    = "zhipu"
	ProviderTongyi   = "tongyi"
	ProviderKimi     = "kimi"
	ProviderClaude   = "claude"

	OPENAI_API_KEY       = "OPENAI_API_KEY"
	OPENAI_BASE_URL      = "OPENAI_BASE_URL"
	OPENAI_MODEL_ENV     = "OPENAI_MODEL"
	OPENAI_BASEURL       = "https://api.openai.com/v1"
	OPENAI_DEFAULT_MODEL = "gpt-4o"

	DOUBAO_API_KEY       = "DOUBAO_API_KEY"
	DOUBAO_BASE_URL      = "DOUBAO_BASE_URL"
	DOUBAO_MODEL_ENV     = "DOUBAO_MODEL"
	DOUBAO_BASEURL       = "https://ark.cn-beijing.volces.com/api/v3"
	DOUBAO_DEFAULT_MODEL = "doubao-seed-2-0-code-preview-260215"

	DEEPSEEK_API_KEY       = "DEEPSEEK_API_KEY"
	DEEPSEEK_BASE_URL      = "DEEPSEEK_BASE_URL"
	DEEPSEEK_MODEL_ENV     = "DEEPSEEK_MODEL"
	DEEPSEEK_BASEURL       = "https://api.deepseek.com"
	DEEPSEEK_DEFAULT_MODEL = "deepseek-chat"

	ZHIPU_API_KEY       = "ZHIPU_API_KEY"
	ZHIPU_BASE_URL      = "ZHIPU_BASE_URL"
	ZHIPU_MODEL_ENV     = "ZHIPU_MODEL"
	ZHIPU_BASEURL       = "https://open.bigmodel.cn/api/paas/v4"
	ZHIPU_DEFAULT_MODEL = "glm-4"

	TONGYI_API_KEY       = "TONGYI_API_KEY"
	TONGYI_BASE_URL      = "TONGYI_BASE_URL"
	TONGYI_MODEL_ENV     = "TONGYI_MODEL"
	TONGYI_BASEURL       = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	TONGYI_DEFAULT_MODEL = "qwen-plus"

	KIMI_API_KEY       = "KIMI_API_KEY"
	KIMI_BASE_URL      = "KIMI_BASE_URL"
	KIMI_MODEL_ENV     = "KIMI_MODEL"
	KIMI_BASEURL       = "https://api.moonshot.cn/v1"
	KIMI_DEFAULT_MODEL = "moonshot-v1-8k"

	CLAUDE_API_KEY       = "CLAUDE_API_KEY"
	CLAUDE_BASE_URL      = "CLAUDE_BASE_URL"
	CLAUDE_MODEL_ENV     = "CLAUDE_MODEL"
	CLAUDE_BASEURL       = "https://api.anthropic.com"
	CLAUDE_DEFAULT_MODEL = "claude-3-5-sonnet-latest"

	ProviderQwen   = "qwen"
	QWEN_API_KEY   = "QWEN_API_KEY"
	QWEN_BASE_URL  = "QWEN_BASE_URL"
	QWEN_MODEL_ENV = "QWEN_MODEL"
	// QWEN_BASEURL 默认指向本地部署的 Qwen 模型服务。
	QWEN_BASEURL       = "http://localhost:8095/v1"
	QWEN_DEFAULT_MODEL = "qwen3"
)

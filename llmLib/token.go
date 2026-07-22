package llmlib

// estimateTokens 估算字符串的 token 数量（英文约4字符/token，中文约1.5~2字符/token）
func estimateTokens(s string) int {
	ascii, cjk := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			cjk++
		}
	}
	return ascii/4 + cjk*2/3 + 1
}

// Budget 表示 token 使用的预算分配
type Budget struct {
	Total        int // 总 token 预算
	SystemPrompt int // 系统提示词占用的 token 数
	Tools        int // 工具定义占用的 token 数
	History      int // 对话历史占用的 token 数
	Retrieved    int // 检索内容占用的 token 数
}

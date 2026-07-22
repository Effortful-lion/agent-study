// 文件职责：
// - 提供轻量级 token 粗估逻辑和预算结构。
// - 供流式回填、成本评估和调用方做提示词预算拆分时使用。

package llmlib

// estimateTokens 按中英文字符比例粗略估算 token 数量，适合无精确 tokenizer 的场景。
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

// Budget 描述一次请求中不同内容段预留的 token 预算。
type Budget struct {
	Total        int // 总 token 预算，由调用方按模型上下文窗口预先分配。
	SystemPrompt int // 系统提示词预算，供全局约束和角色设定使用。
	Tools        int // 工具定义预算，供函数调用或工具描述占用。
	History      int // 历史对话预算，供上下文回放使用。
	Retrieved    int // 检索内容预算，供 RAG 等补充上下文使用。
}

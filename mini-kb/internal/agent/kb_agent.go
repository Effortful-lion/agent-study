// Package agent 封装知识库 Agent，负责将 llmLib Agent 与知识库工具集成。
package agent

import (
	"context"
	"fmt"

	llmlib "github.com/Effortful-lion/agent-study/llmLib"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/memory"
	"github.com/Effortful-lion/agent-study/mini-kb/internal/tools"
)

// KBConfig 是知识库 Agent 的配置。
type KBConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
	TopK     int
	MaxSteps int
}

// DefaultKBConfig 返回默认配置。
func DefaultKBConfig() *KBConfig {
	return &KBConfig{
		TopK:     5,
		MaxSteps: 10,
	}
}

// KBAnswer 是一次知识库问答的结果。
type KBAnswer struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
	Tools   []string `json:"tools,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// KnowledgeAgent 封装了 llmLib Agent 和知识库工具。
type KnowledgeAgent struct {
	config    *KBConfig
	retriever *tools.KBRetriever
	memMgr    *memory.Manager
	sessionID string
}

// NewKnowledgeAgent 创建知识库 Agent。
func NewKnowledgeAgent(cfg *KBConfig, retriever *tools.KBRetriever, memDir, sessionID string) *KnowledgeAgent {
	if cfg == nil {
		cfg = DefaultKBConfig()
	}
	if sessionID == "" {
		sessionID = "default"
	}
	return &KnowledgeAgent{
		config:    cfg,
		retriever: retriever,
		memMgr:    memory.NewManager(memDir),
		sessionID: sessionID,
	}
}

// Ask 执行单轮知识库问答。
func (a *KnowledgeAgent) Ask(ctx context.Context, question string) (*KBAnswer, error) {
	if a.retriever.ChunkCount() == 0 {
		return &KBAnswer{
			Answer: "知识库为空，请先运行 'mini-kb ingest' 导入文档。",
		}, nil
	}

	// 加载会话记忆
	session := a.memMgr.LoadSession(a.sessionID)

	// 构建 system prompt
	systemPrompt := buildSystemPrompt(a.config, session, a.memMgr)

	// 创建 llmLib Agent
	registry := llmlib.NewRegistryToolSet()
	registerKBTools(registry, a.retriever)

	p, err := llmlib.NewProvider(a.config.Provider)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM provider 失败: %w", err)
	}

	budget := llmlib.DefaultAgentBudgetConfig()
	budget.MaxSteps = a.config.MaxSteps

	agent := llmlib.New(p, a.config.Model, registry,
		llmlib.WithSystemPrompt(systemPrompt),
		llmlib.WithAgentBudgetConfig(budget),
		llmlib.WithAgentAPIKey(a.config.APIKey),
		llmlib.WithAgentBaseURL(a.config.BaseURL),
	)

	events, err := agent.Run(ctx, question)
	if err != nil {
		return nil, fmt.Errorf("Agent 启动失败: %w", err)
	}

	var answerText string
	var toolsCalled []string
	for event := range events {
		switch event.Type {
		case llmlib.EventToolCall:
			toolsCalled = append(toolsCalled, event.Tool)
		case llmlib.EventAnswer:
			answerText = event.Text
		case llmlib.EventError:
			return &KBAnswer{
				Answer: answerText,
				Tools:  toolsCalled,
				Error:  event.Text,
			}, nil
		}
	}

	// 收集来源
	sources := collectSources(answerText, a.retriever)

	// 保存会话
	a.memMgr.AddTurn(session, memory.SessionTurn{
		Question: question,
		Answer:   answerText,
		Sources:  sources,
		Tools:    toolsCalled,
	})
	if err := a.memMgr.SaveSession(session); err != nil {
		// 非致命错误
		fmt.Printf("保存会话失败: %v\n", err)
	}

	return &KBAnswer{
		Answer:  answerText,
		Sources: sources,
		Tools:   toolsCalled,
	}, nil
}

// Chat 进入连续对话模式。
func (a *KnowledgeAgent) Chat(ctx context.Context) error {
	if a.retriever.ChunkCount() == 0 {
		fmt.Println("知识库为空，请先运行 'mini-kb ingest' 导入文档。")
		return nil
	}

	session := a.memMgr.LoadSession(a.sessionID)
	fmt.Printf("会话 %s 已加载（已有 %d 轮对话）\n", a.sessionID, session.TurnCount())
	fmt.Println("输入问题开始问答，输入 'quit' 或 'exit' 退出。")
	fmt.Println()

	registry := llmlib.NewRegistryToolSet()
	registerKBTools(registry, a.retriever)

	p, err := llmlib.NewProvider(a.config.Provider)
	if err != nil {
		return fmt.Errorf("创建 LLM provider 失败: %w", err)
	}

	budget := llmlib.DefaultAgentBudgetConfig()
	budget.MaxSteps = a.config.MaxSteps

	for {
		fmt.Print("> ")
		var question string
		if _, err := fmt.Scanln(&question); err != nil {
			continue
		}
		if question == "" {
			continue
		}
		if question == "quit" || question == "exit" {
			break
		}

		systemPrompt := buildSystemPrompt(a.config, session, a.memMgr)

		agent := llmlib.New(p, a.config.Model, registry,
			llmlib.WithSystemPrompt(systemPrompt),
			llmlib.WithAgentBudgetConfig(budget),
			llmlib.WithAgentAPIKey(a.config.APIKey),
			llmlib.WithAgentBaseURL(a.config.BaseURL),
		)

		events, err := agent.Run(ctx, question)
		if err != nil {
			fmt.Printf("Agent 启动失败: %v\n", err)
			continue
		}

		var answerText string
		var toolsCalled []string
		for event := range events {
			switch event.Type {
			case llmlib.EventStepStart:
				fmt.Printf("[Step %d 开始] %s\n", event.Step, event.Text)
			case llmlib.EventToolCall:
				fmt.Printf("[工具] %s\n", event.Tool)
				toolsCalled = append(toolsCalled, event.Tool)
			case llmlib.EventToolResult:
				fmt.Printf("[工具结果] %s: %s\n", event.Tool, truncate(event.Text, 200))
			case llmlib.EventAnswer:
				answerText = event.Text
				fmt.Printf("\n%s\n\n", answerText)
			case llmlib.EventError:
				fmt.Printf("[错误] %s\n", event.Text)
			}
		}

		if answerText == "" {
			answerText = "(无回答)"
		}

		sources := collectSources(answerText, a.retriever)
		a.memMgr.AddTurn(session, memory.SessionTurn{
			Question: question,
			Answer:   answerText,
			Sources:  sources,
			Tools:    toolsCalled,
		})
		if err := a.memMgr.SaveSession(session); err != nil {
			fmt.Printf("保存会话失败: %v\n", err)
		}
	}

	return nil
}

func buildSystemPrompt(cfg *KBConfig, session *memory.Session, memMgr *memory.Manager) string {
	prompt := "你是一个知识库问答助手。回答问题时，请遵循以下规则：\n" +
		"1. 优先使用 search_knowledge 工具搜索知识库获取准确信息\n" +
		"2. 如果需要查看完整文档，使用 read_document 工具\n" +
		"3. 如果需要查看特定文本块，使用 get_chunk 工具\n" +
		"4. 如果需要了解已索引的文档，使用 list_documents 工具\n" +
		"5. 回答必须基于知识库内容，不能凭空编造\n" +
		"6. 如果知识库没有相关信息，明确告知用户\n" +
		"7. 在回答末尾注明信息来源文件"

	// 注入对话记忆
	if history := memory.ConversationMemory(session.History(5), 5); history != "" {
		prompt += "\n\n以下是最近5轮的对话历史，请参考上下文回答问题：" + history
	}

	// 注入用户偏好
	if prefs := memory.PreferencePrompt(memMgr.GetPreferences()); prefs != "" {
		prompt += prefs
	}

	return prompt
}

func registerKBTools(registry *llmlib.Registry, retriever *tools.KBRetriever) {
	registry.Register(&KBTool{name: "search_knowledge", desc: "搜索知识库，根据关键词查找相关内容。参数: query(string, 必填), top_k(int, 可选)", retriever: retriever, fn: retriever.SearchKnowledge})
	registry.Register(&KBTool{name: "read_document", desc: "读取指定文档的完整内容。参数: doc_id(string, 必填)", retriever: retriever, fn: retriever.ReadDocument})
	registry.Register(&KBTool{name: "get_chunk", desc: "获取指定文本块的完整内容。参数: chunk_id(string, 必填)", retriever: retriever, fn: retriever.GetChunk})
	registry.Register(&KBTool{name: "list_documents", desc: "列出所有已索引的文档。无需参数", retriever: retriever, fn: retriever.ListDocuments})
}

// KBTool 是一个知识库工具。
type KBTool struct {
	name      string
	desc      string
	retriever *tools.KBRetriever
	fn        func(ctx context.Context, args map[string]any) (any, error)
}

func (t *KBTool) Name() string        { return t.name }
func (t *KBTool) Description() string { return t.desc }
func (t *KBTool) Parameters() map[string]string {
	return map[string]string{}
}
func (t *KBTool) Call(ctx context.Context, args map[string]any) (any, error) {
	return t.fn(ctx, args)
}

func collectSources(answer string, retriever *tools.KBRetriever) []string {
	seen := make(map[string]bool)
	chunks := retriever.Chunks()
	for _, c := range chunks {
		if !seen[c.FilePath] && contains(answer, c.Content) {
			seen[c.FilePath] = true
		}
	}
	var sources []string
	for p := range seen {
		sources = append(sources, p)
	}
	return sources
}

func contains(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	if len(haystack) < len(needle) {
		return false
	}
	h := []rune(haystack)
	n := []rune(needle)
	for i := 0; i <= len(h)-len(n); i++ {
		match := true
		for j := 0; j < len(n); j++ {
			if h[i+j] != n[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

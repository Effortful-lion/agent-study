// 文件职责：
// - Agent 核心运行时：基于状态机的 Think-Act-Observe 循环。
// - 参考 eino 设计：组件(ChatModel/ToolNode) + 编排(Chain) + 执行(Agent)。
// - 每个阶段（thinking/acting）都是独立的 Step，通过状态机驱动转换。
// - 清晰的事件分层：StepStart → ModelCall → ToolCall → ToolResult → StepEnd → Answer。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/lg"
)

// ============================================================================
// 常量与日志模块
// ============================================================================

const defaultSystemPrompt = "你是一个命令行 AI 助手。当用户问题中包含需要真实计算或当前时间等事实性查询时，必须调用相应工具获取结果，不要依赖自身知识。如果问题包含多个子任务，必须逐一调用所有相关工具完成，不要遗漏。"

// ============================================================================
// Agent 结构体
// ============================================================================

// Agent 是 Agent 运行时，负责编排 ChatModel 和 ToolNode 的交互循环。
//
// 调用层次（从上到下）：
//
//	Agent.Run()
//	  ├── Step: thinking   → callModel() → 发 EventModelCall
//	  │    ├── 有 ToolCalls → 进入 acting 阶段
//	  │    └── 无 ToolCalls → 结束，发 EventAnswer
//	  ├── Step: acting     → executeTools() → 发 EventToolCall + EventToolResult
//	  │    └── 记录结果 → 回到 thinking
//	  └── Step: done       → 发送 EventDone
//
// 这种设计让调用方（尤其是初学者）能清楚看到：
//   - 我们在调用 ChatModel（EventModelCall）
//   - 我们在执行 Tool（EventToolCall → EventToolResult）
//   - 我们在哪个 Step 阶段（EventStepStart → EventStepEnd）
type Agent struct {
	provider     Provider // 模型出口
	model        string   // 模型名
	apiKey       string
	baseURL      string
	tools        *Registry // 工具集
	systemPrompt string
	budget       AgentBudgetConfig
	store        Store
	sessionID    string
	memory       *State
}

// ============================================================================
// 构造函数 + Option
// ============================================================================

func New(provider Provider, model string, registry *Registry, opts ...Option) *Agent {
	agent := &Agent{
		provider:     provider,
		model:        model,
		tools:        registry,
		systemPrompt: defaultSystemPrompt,
		budget:       DefaultAgentBudgetConfig(),
	}
	for _, opt := range opts {
		opt(agent)
	}
	return agent
}

type Option func(*Agent)

func WithSystemPrompt(prompt string) Option {
	return func(agent *Agent) { agent.systemPrompt = prompt }
}

func WithAgentBudgetConfig(budget AgentBudgetConfig) Option {
	return func(agent *Agent) { agent.budget = budget }
}

func WithAgentAPIKey(apiKey string) Option {
	return func(agent *Agent) { agent.apiKey = apiKey }
}

func WithAgentBaseURL(baseURL string) Option {
	return func(agent *Agent) { agent.baseURL = baseURL }
}

func WithStore(store Store, sessionID string) Option {
	return func(agent *Agent) {
		agent.store = store
		agent.sessionID = strings.TrimSpace(sessionID)
	}
}

// ============================================================================
// Run — Agent 主循环
// ============================================================================

// Run 启动 Agent，返回事件流。
// 内部使用状态机驱动 thinking → acting → thinking → ... 的循环。
func (agent *Agent) Run(ctx context.Context, goal string) (<-chan AgentEvent, error) {
	state, err := agent.loadState(ctx)
	if err != nil {
		lg.Frame.Error("agent: 加载状态失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("load state failed: %w", err)
	}

	if state.StartedAt.IsZero() {
		state.StartedAt = time.Now()
	}
	state.Goal = goal
	state.Phase = PhaseThinking

	out := make(chan AgentEvent, 64) // 缓冲 64 个事件，防止 goroutine 阻塞
	go agent.runLoop(ctx, state, out)
	return out, nil
}

// runLoop 是 Agent 的主循环 goroutine。
// 不再使用手动 for 循环，而是通过状态驱动的方式组织每一步。
func (agent *Agent) runLoop(ctx context.Context, state *State, out chan<- AgentEvent) {
	defer close(out)

	for {
		// 1. 检查上下文取消
		if ctx.Err() != nil {
			return
		}

		// 2. 根据当前 Phase 决定下一步
		switch state.Phase {
		case PhaseThinking:
			if !agent.stepThink(ctx, state, out) {
				return // 出错或完成，退出循环
			}

		case PhaseActing:
			if !agent.stepAct(ctx, state, out) {
				return
			}

		case PhaseDone:
			agent.emit(out, AgentEvent{Type: EventDone, Step: state.Step})
			return

		case PhaseError:
			agent.emit(out, AgentEvent{Type: EventDone, Step: state.Step})
			return

		default:
			lg.Frame.Error("agent: 未知的 Phase", lg.Fields{"phase": state.Phase})
			return
		}
	}
}

// ============================================================================
// Step: Thinking — 调用 ChatModel
// ============================================================================

// stepThink 执行 Thinking 阶段：构建消息 → 调用模型 → 判断是否有工具调用。
// 返回 false 表示循环应该终止（出错或完成）。
func (agent *Agent) stepThink(ctx context.Context, state *State, out chan<- AgentEvent) bool {
	// 检查预算
	if agent.budget.ShouldStop(state) {
		state.Phase = PhaseError
		agent.emit(out, AgentEvent{Type: EventError, Text: "预算耗尽", Step: state.Step})
		return false
	}

	state.Step++
	agent.emit(out, AgentEvent{Type: EventStepStart, Step: state.Step, Text: "thinking"})

	// 构建消息
	messages := agent.buildMessages(state)

	// 调用模型
	agent.emit(out, AgentEvent{Type: EventModelCall, Step: state.Step, Text: fmt.Sprintf("调用模型 %s", agent.model)})
	resp, err := agent.callModel(ctx, messages)
	if err != nil {
		state.Phase = PhaseError
		lg.Frame.Error("agent: 模型调用失败", lg.Fields{"error": err, "step": state.Step})
		agent.emit(out, AgentEvent{Type: EventError, Text: fmt.Sprintf("模型调用失败: %v", err), Step: state.Step})
		return false
	}

	// 累计 token
	state.Usage.InputTokens += resp.InputTokens
	state.Usage.OutputTokens += resp.OutputTokens

	// 把模型原始响应发出去，方便调试
	if resp.Content != "" {
		agent.emit(out, AgentEvent{Type: EventModelResponse, Text: resp.Content, Step: state.Step})
	}
	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			agent.emit(out, AgentEvent{Type: EventModelResponse, Text: fmt.Sprintf("[tool_call] %s(%s)", tc.Name, string(tc.Args)), Step: state.Step})
		}
	}

	// 检测工具调用
	toolCalls := resp.ToolCalls
	if len(toolCalls) == 0 && resp.Content != "" && agent.tools != nil {
		toolCalls = agent.parseReActToolCalls(resp.Content)
	}

	if len(toolCalls) > 0 {
		// 有工具调用：记录思考内容，保存待执行的 toolCalls 到 state
		if resp.Content != "" {
			agent.emit(out, AgentEvent{Type: EventThought, Text: resp.Content, Step: state.Step})
		}
		agent.saveModelResponse(state, resp, toolCalls)
		state.Phase = PhaseActing
		agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → acting"})
	} else {
		// 无工具调用。
		// 框架检查：如果这是首轮且还没用过任何工具，可能是模型偷懒了，
		// 追加提示要求模型继续处理，不直接结束。
		if state.Step == 1 && agent.hasTools() && !agent.hasToolHistory(state) {
			agent.emit(out, AgentEvent{Type: EventModelResponse, Text: "(框架检测到模型未调用工具，追加提示要求继续)", Step: state.Step})
			state.Messages = append(state.Messages,
				Message{Role: Assistant, Content: resp.Content},
				Message{Role: User, Content: "请使用工具完成上述任务，不要直接给出答案。"},
			)
			state.UpdatedAt = time.Now()
			agent.checkpoint(ctx, state)
			state.Phase = PhaseThinking // 继续循环
			agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → thinking (retry)"})
			return true
		}

		// 新增：如果已经调用过部分工具，但本轮没有继续调用工具而是直接给出答案，
		// 且还有未调用过的工具，则追加提示要求模型检查是否需要继续调用。
		// 这能防止模型在多子任务场景中过早结束，遗漏剩余工具。
		if agent.hasTools() && agent.hasToolHistory(state) {
			uncalled := agent.uncalledToolNames(state)
			if len(uncalled) > 0 {
				count := agent.noToolResponseCount(state)
				if count < 2 {
					called := agent.calledToolNames(state)
					agent.emit(out, AgentEvent{Type: EventModelResponse, Text: "(框架检测到还有未调用工具，追加提示检查是否需要继续)", Step: state.Step})
					state.Messages = append(state.Messages,
						Message{Role: Assistant, Content: resp.Content},
						Message{Role: User, Content: fmt.Sprintf("原始目标是：%s。你已经调用过工具：%s。但还有以下工具未使用：%s。请根据目标判断是否需要继续调用它们来完成任务。如果确实不需要，请直接给出最终答案。", state.Goal, strings.Join(called, ", "), strings.Join(uncalled, ", "))},
					)
					agent.setNoToolResponseCount(state, count+1)
					state.UpdatedAt = time.Now()
					agent.checkpoint(ctx, state)
					state.Phase = PhaseThinking
					agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → thinking (continue)"})
					return true
				}
			}
		}

		// 正常结束
		state.Phase = PhaseDone
		state.Answer = resp.Content
		state.UpdatedAt = time.Now()
		agent.checkpoint(ctx, state)
		agent.emit(out, AgentEvent{Type: EventAnswer, Text: resp.Content, Step: state.Step})
		agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → done"})
	}
	return true
}

// saveModelResponse 将模型响应暂存到 state，供 acting 阶段使用。
// 使用 Metadata 避免在 State 结构体中添加临时字段。
func (agent *Agent) saveModelResponse(state *State, resp *ChatResponse, toolCalls []ToolCall) {
	if state.Metadata == nil {
		state.Metadata = make(map[string]string)
	}
	if resp.Content != "" {
		state.Metadata["_last_content"] = resp.Content
	}
	// 序列化 toolCalls 到 metadata（acting 阶段需要）
	if data, err := json.Marshal(toolCalls); err == nil {
		state.Metadata["_last_tool_calls"] = string(data)
	}
}

// loadModelResponse 从 state 恢复暂存的模型响应。
func (agent *Agent) loadModelResponse(state *State) (content string, toolCalls []ToolCall) {
	if state.Metadata == nil {
		return "", nil
	}
	content = state.Metadata["_last_content"]
	if raw, ok := state.Metadata["_last_tool_calls"]; ok && raw != "" {
		json.Unmarshal([]byte(raw), &toolCalls)
	}
	// 清理临时数据
	delete(state.Metadata, "_last_content")
	delete(state.Metadata, "_last_tool_calls")
	return
}

// ============================================================================
// Step: Acting — 执行工具调用
// ============================================================================

// stepAct 执行 Acting 阶段：从 state 读取待执行的工具调用，逐一执行。
func (agent *Agent) stepAct(ctx context.Context, state *State, out chan<- AgentEvent) bool {
	content, toolCalls := agent.loadModelResponse(state)

	agent.emit(out, AgentEvent{Type: EventStepStart, Step: state.Step, Text: fmt.Sprintf("acting (%d tools)", len(toolCalls))})

	executedTCs, results, fatal := agent.executeToolCalls(ctx, out, toolCalls, state)
	if fatal {
		state.Phase = PhaseError
		agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "acting → error"})
		return false
	}

	// 记录工具调用结果到消息历史
	if len(executedTCs) > 0 {
		state.Messages = append(state.Messages,
			Message{Role: Assistant, Content: content, ToolCalls: executedTCs},
		)
		for i, tc := range executedTCs {
			state.Messages = append(state.Messages,
				Message{Role: ToolRole, Content: results[i], ToolCallID: tc.ID},
			)
		}
	}

	// 如果所有工具都失败了，也把失败信息作为 tool result 反馈给模型
	if len(executedTCs) == 0 && len(toolCalls) > 0 {
		// 工具全部失败：构造一个错误消息让模型知道
		errMsg := fmt.Sprintf("工具调用全部失败，请尝试其他方式或直接给出你能提供的最佳答案。")
		state.Messages = append(state.Messages,
			Message{Role: Assistant, Content: content},
			Message{Role: ToolRole, Content: errMsg, ToolCallID: "error"},
		)
	}

	// 提示模型继续完成剩余任务：如果还有需要工具才能回答的子任务，继续调用工具。
	// 这条消息显式提醒模型在拿到部分工具结果后不要过早结束，对包含多个子任务的目标尤为重要。
	if agent.hasTools() {
		state.Messages = append(state.Messages,
			Message{Role: User, Content: fmt.Sprintf("请继续完成目标：%s。如果还有需要工具才能回答的部分，请继续调用工具；如果所有子任务都已完成，请直接给出最终答案。", state.Goal)},
		)
	}

	state.UpdatedAt = time.Now()
	agent.checkpoint(ctx, state)

	// 回到 thinking 阶段
	state.Phase = PhaseThinking
	agent.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "acting → thinking"})
	return true
}

// ============================================================================
// 工具执行
// ============================================================================

// executeToolCalls 执行工具调用列表。
// 返回：成功执行的工具调用、结果列表、是否致命错误。
// 致命错误：上下文取消或重复检测失败。
func (agent *Agent) executeToolCalls(ctx context.Context, out chan<- AgentEvent,
	toolCalls []ToolCall, state *State) (executedTCs []ToolCall, results []string, fatal bool) {

	for _, tc := range toolCalls {
		select {
		case <-ctx.Done():
			return executedTCs, results, true
		default:
		}

		// 发送工具调用事件
		agent.emit(out, AgentEvent{
			Type: EventToolCall, Tool: tc.Name,
			Args: string(tc.Args), Step: state.Step,
		})

		// 重复动作检测
		actionKey := fmt.Sprintf("%s:%s", tc.Name, string(tc.Args))
		state.ActionCounts[actionKey]++
		if !agent.budget.ShouldRetry(actionKey, state.ActionCounts) {
			lg.Frame.Warn("agent: 动作重复次数过多", lg.Fields{"tool": tc.Name, "key": actionKey})
			// 不 fatal，只是跳过这个工具调用
			errMsg := fmt.Sprintf("工具 %s 重复调用次数过多，已跳过", tc.Name)
			executedTCs = append(executedTCs, ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID})
			results = append(results, errMsg)
			agent.emit(out, AgentEvent{Type: EventToolResult, Tool: tc.Name, Text: errMsg, Step: state.Step})
			continue
		}

		// 执行工具
		result, err := agent.executeTool(ctx, tc)
		if err != nil {
			lg.Frame.Error("agent: 工具执行失败", lg.Fields{"tool": tc.Name, "error": err})
			// 关键改进：工具失败时，把错误信息作为 tool result 反馈给模型
			errMsg := fmt.Sprintf("工具 %s 执行失败: %v。请尝试其他方式。", tc.Name, err)
			executedTCs = append(executedTCs, ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID})
			results = append(results, errMsg)
			agent.emit(out, AgentEvent{
				Type: EventToolResult, Tool: tc.Name,
				Text: errMsg, Step: state.Step,
			})
			continue
		}

		// 成功
		executedTCs = append(executedTCs, ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID, Result: result})
		results = append(results, result)

		agent.emit(out, AgentEvent{
			Type: EventToolResult, Tool: tc.Name,
			Text: result, Step: state.Step,
		})
	}

	return executedTCs, results, false
}

// ============================================================================
// 辅助方法
// ============================================================================

// emit 安全地向事件通道发送事件。
// 使用阻塞发送；因为有缓冲区（64），正常情况下不会阻塞。
func (agent *Agent) emit(out chan<- AgentEvent, event AgentEvent) {
	out <- event
}

// loadState 加载 Agent 状态。
func (agent *Agent) loadState(ctx context.Context) (*State, error) {
	if agent.store != nil && agent.sessionID != "" {
		state, err := agent.store.Load(ctx, agent.sessionID)
		if err != nil {
			return nil, err
		}
		return state, nil
	}
	if agent.memory != nil {
		return agent.memory, nil
	}
	return &State{
		ActionCounts: make(map[string]int),
	}, nil
}

// buildMessages 构建发送给模型的消息列表。
// 框架自动完成以下工作，调用方无需在 prompt 中操心：
//  1. 追加工具列表（名称+描述）
//  2. 追加工具调用指令（有工具时强制要求使用工具）
func (agent *Agent) buildMessages(state *State) []Message {
	var messages []Message

	systemContent := agent.systemPrompt

	if agent.tools != nil && len(agent.tools.tools) > 0 {
		// 自动追加工具列表
		systemContent += "\n\n你可以使用以下工具：\n"
		for name, tool := range agent.tools.tools {
			systemContent += fmt.Sprintf("- %s: %s\n", name, tool.Description())
		}
		// 自动追加工具调用指令 — 框架保证行为，不依赖 prompt
		systemContent += "\n使用规则：\n" +
			"1. 不要自己计算或猜测，必须调用工具获取真实结果，如果没有对应工具就输出无可用工具\n" +
			"2. 用户可能要求完成多个子任务，请调用工具完成所有子任务，不要遗漏\n" +
			"3. 如果你已经调用了部分工具，收到结果后仍要检查是否还有其他子任务需要工具\n" +
			"4. 所有子任务完成后，汇总给出最终答案"
	}

	messages = append(messages, Message{Role: System, Content: systemContent})
	// 每次调用模型都重新附加原始目标，避免模型在多轮工具调用后遗忘任务。
	messages = append(messages, Message{Role: User, Content: state.Goal})
	messages = append(messages, state.Messages...)
	return messages
}

// callModel 调用模型。
func (agent *Agent) callModel(ctx context.Context, messages []Message) (*ChatResponse, error) {
	cfg := LLMConfig{
		Model:   agent.model,
		APIKey:  agent.apiKey,
		BaseURL: agent.baseURL,
	}
	tcp, ok := agent.provider.(ToolCallProvider)
	if ok && agent.tools != nil {
		return tcp.ChatWithTools(ctx, cfg, messages, agent.tools.ToolDefs())
	}
	return agent.provider.Chat(ctx, cfg, messages)
}

// parseReActToolCalls 解析 ReAct 格式的工具调用。
func (agent *Agent) parseReActToolCalls(content string) []ToolCall {
	paradigm := DetectParadigm(content)
	if paradigm == nil {
		paradigm = &ReActParadigm{}
	}
	calls, err := paradigm.Parse(content)
	if err != nil {
		return nil
	}
	return calls
}

// executeTool 执行单个工具调用。
func (agent *Agent) executeTool(ctx context.Context, tc ToolCall) (string, error) {
	if agent.tools == nil {
		return "", NewAgentError(ErrCategoryTool, "工具注册表为空", nil, false)
	}
	tool, ok := agent.tools.Get(tc.Name)
	if !ok {
		return "", NewAgentError(ErrCategoryToolNotFound, fmt.Sprintf("工具 %s 不存在", tc.Name), nil, false)
	}

	var args map[string]interface{}
	if tc.Args != nil {
		if err := json.Unmarshal(tc.Args, &args); err != nil {
			var argsStr string
			if json.Unmarshal(tc.Args, &argsStr) == nil {
				if json.Unmarshal([]byte(argsStr), &args) != nil {
					return "", NewAgentError(ErrCategoryTool, fmt.Sprintf("工具 %s 参数解析失败", tc.Name), err, false)
				}
			} else {
				return "", NewAgentError(ErrCategoryTool, fmt.Sprintf("工具 %s 参数解析失败", tc.Name), err, false)
			}
		}
	}

	var result string
	err := RetryWithBackoff(time.Second, time.Minute, agent.budget.MaxRetries, func() error {
		var err error
		var res interface{}
		res, err = tool.Call(ctx, args)
		if err == nil {
			result = fmt.Sprintf("%v", res)
		}
		return err
	})
	if err != nil {
		return "", NewAgentError(ErrCategoryTool, fmt.Sprintf("工具 %s 执行失败", tc.Name), err, false)
	}
	return result, nil
}

// hasTools 检查 Agent 是否注册了工具。
func (agent *Agent) hasTools() bool {
	return agent.tools != nil && len(agent.tools.tools) > 0
}

// hasToolHistory 检查消息历史中是否已经有工具调用记录。
func (agent *Agent) hasToolHistory(state *State) bool {
	for _, msg := range state.Messages {
		if msg.Role == ToolRole || (msg.Role == Assistant && len(msg.ToolCalls) > 0) {
			return true
		}
	}
	return false
}

// calledToolNames 返回消息历史中已经被调用过的工具名称集合（去重）。
func (agent *Agent) calledToolNames(state *State) []string {
	seen := make(map[string]bool)
	for _, msg := range state.Messages {
		if msg.Role == Assistant {
			for _, tc := range msg.ToolCalls {
				seen[tc.Name] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// uncalledToolNames 返回注册在 Agent 中但尚未被调用过的工具名称列表。
func (agent *Agent) uncalledToolNames(state *State) []string {
	called := make(map[string]bool)
	for _, name := range agent.calledToolNames(state) {
		called[name] = true
	}
	var uncalled []string
	if agent.tools != nil {
		for name := range agent.tools.tools {
			if !called[name] {
				uncalled = append(uncalled, name)
			}
		}
	}
	return uncalled
}

// noToolResponseCount 读取“无工具调用直接返回答案”的继续提示次数。
func (agent *Agent) noToolResponseCount(state *State) int {
	if state.Metadata == nil {
		return 0
	}
	raw, ok := state.Metadata["_no_tool_response_count"]
	if !ok || raw == "" {
		return 0
	}
	var count int
	if _, err := fmt.Sscanf(raw, "%d", &count); err != nil {
		return 0
	}
	return count
}

// setNoToolResponseCount 设置“无工具调用直接返回答案”的继续提示次数。
func (agent *Agent) setNoToolResponseCount(state *State, count int) {
	if state.Metadata == nil {
		state.Metadata = make(map[string]string)
	}
	state.Metadata["_no_tool_response_count"] = fmt.Sprintf("%d", count)
}

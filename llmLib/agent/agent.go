// 文件职责：
// - Agent 核心运行时：基于状态机的 Think-Act-Observe 循环。

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/errutil"
	"github.com/Effortful-lion/agent-study/llmLib/lg"
	"github.com/Effortful-lion/agent-study/llmLib/provider"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

const defaultSystemPrompt = "你是一个命令行 AI 助手。当用户问题中包含需要真实计算或当前时间等事实性查询时，必须调用相应工具获取结果，不要依赖自身知识。如果问题包含多个子任务，必须逐一调用所有相关工具完成，不要遗漏。"

// Agent 是 Agent 运行时，负责编排 ChatModel 和 ToolNode 的交互循环。
type Agent struct {
	p            provider.Provider
	model        string
	apiKey       string
	baseURL      string
	tools        *tool.Registry
	systemPrompt string
	budget       AgentBudgetConfig
	store        Store
	sessionID    string
	memory       *State
}

// Option 是 Agent 的配置函数。
type Option func(*Agent)

// New 创建一个新的 Agent 实例。
func New(p provider.Provider, model string, registry *tool.Registry, opts ...Option) *Agent {
	a := &Agent{
		p:            p,
		model:        model,
		tools:        registry,
		systemPrompt: defaultSystemPrompt,
		budget:       DefaultAgentBudgetConfig(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithSystemPrompt(prompt string) Option {
	return func(a *Agent) { a.systemPrompt = prompt }
}

func WithAgentBudgetConfig(budget AgentBudgetConfig) Option {
	return func(a *Agent) { a.budget = budget }
}

func WithAgentAPIKey(apiKey string) Option {
	return func(a *Agent) { a.apiKey = apiKey }
}

func WithAgentBaseURL(baseURL string) Option {
	return func(a *Agent) { a.baseURL = baseURL }
}

func WithStore(store Store, sessionID string) Option {
	return func(a *Agent) {
		a.store = store
		a.sessionID = strings.TrimSpace(sessionID)
	}
}

// Run 启动 Agent，返回事件流。
func (a *Agent) Run(ctx context.Context, goal string) (<-chan AgentEvent, error) {
	state, err := a.loadState(ctx)
	if err != nil {
		lg.Frame.Error("agent: 加载状态失败", lg.Fields{"error": err})
		return nil, fmt.Errorf("load state failed: %w", err)
	}

	if state.StartedAt.IsZero() {
		state.StartedAt = time.Now()
	}
	state.Goal = goal
	state.Phase = PhaseThinking

	out := make(chan AgentEvent, 64)
	go a.runLoop(ctx, state, out)
	return out, nil
}

func (a *Agent) runLoop(ctx context.Context, state *State, out chan<- AgentEvent) {
	defer close(out)

	for {
		if ctx.Err() != nil {
			return
		}

		switch state.Phase {
		case PhaseThinking:
			if !a.stepThink(ctx, state, out) {
				return
			}

		case PhaseActing:
			if !a.stepAct(ctx, state, out) {
				return
			}

		case PhaseDone:
			a.emit(out, AgentEvent{Type: EventDone, Step: state.Step})
			return

		case PhaseError:
			a.emit(out, AgentEvent{Type: EventDone, Step: state.Step})
			return

		default:
			lg.Frame.Error("agent: 未知的 Phase", lg.Fields{"phase": state.Phase})
			return
		}
	}
}

func (a *Agent) stepThink(ctx context.Context, state *State, out chan<- AgentEvent) bool {
	if a.budget.ShouldStop(state) {
		state.Phase = PhaseError
		a.emit(out, AgentEvent{Type: EventError, Text: "预算耗尽", Step: state.Step})
		return false
	}

	state.Step++
	a.emit(out, AgentEvent{Type: EventStepStart, Step: state.Step, Text: "thinking"})

	messages := a.buildMessages(state)
	a.emit(out, AgentEvent{Type: EventModelCall, Step: state.Step, Text: fmt.Sprintf("调用模型 %s", a.model)})
	resp, err := a.callModel(ctx, messages)
	if err != nil {
		state.Phase = PhaseError
		lg.Frame.Error("agent: 模型调用失败", lg.Fields{"error": err, "step": state.Step})
		a.emit(out, AgentEvent{Type: EventError, Text: fmt.Sprintf("模型调用失败: %v", err), Step: state.Step})
		return false
	}

	state.Usage.InputTokens += resp.InputTokens
	state.Usage.OutputTokens += resp.OutputTokens

	if resp.Content != "" {
		a.emit(out, AgentEvent{Type: EventModelResponse, Text: resp.Content, Step: state.Step})
	}
	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			a.emit(out, AgentEvent{Type: EventModelResponse, Text: fmt.Sprintf("[tool_call] %s(%s)", tc.Name, string(tc.Args)), Step: state.Step})
		}
	}

	toolCalls := resp.ToolCalls
	if len(toolCalls) == 0 && resp.Content != "" && a.tools != nil {
		toolCalls = a.parseReActToolCalls(resp.Content)
	}

	if len(toolCalls) > 0 {
		if resp.Content != "" {
			a.emit(out, AgentEvent{Type: EventThought, Text: resp.Content, Step: state.Step})
		}
		a.saveModelResponse(state, resp, toolCalls)
		state.Phase = PhaseActing
		a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → acting"})
	} else {
		if state.Step == 1 && a.hasTools() && !a.hasToolHistory(state) {
			a.emit(out, AgentEvent{Type: EventModelResponse, Text: "(框架检测到模型未调用工具，追加提示要求继续)", Step: state.Step})
			state.Messages = append(state.Messages,
				core.Message{Role: core.Assistant, Content: resp.Content},
				core.Message{Role: core.User, Content: "请使用工具完成上述任务，不要直接给出答案。"},
			)
			state.UpdatedAt = time.Now()
			a.checkpoint(ctx, state)
			state.Phase = PhaseThinking
			a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → thinking (retry)"})
			return true
		}

		if a.hasTools() && a.hasToolHistory(state) {
			uncalled := a.uncalledToolNames(state)
			if len(uncalled) > 0 {
				count := a.noToolResponseCount(state)
				if count < 2 {
					called := a.calledToolNames(state)
					a.emit(out, AgentEvent{Type: EventModelResponse, Text: "(框架检测到还有未调用工具，追加提示检查是否需要继续)", Step: state.Step})
					state.Messages = append(state.Messages,
						core.Message{Role: core.Assistant, Content: resp.Content},
						core.Message{Role: core.User, Content: fmt.Sprintf("原始目标是：%s。你已经调用过工具：%s。但还有以下工具未使用：%s。请根据目标判断是否需要继续调用它们来完成任务。如果确实不需要，请直接给出最终答案。", state.Goal, strings.Join(called, ", "), strings.Join(uncalled, ", "))},
					)
					a.setNoToolResponseCount(state, count+1)
					state.UpdatedAt = time.Now()
					a.checkpoint(ctx, state)
					state.Phase = PhaseThinking
					a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → thinking (continue)"})
					return true
				}
			}
		}

		state.Phase = PhaseDone
		state.Answer = resp.Content
		state.UpdatedAt = time.Now()
		a.checkpoint(ctx, state)
		a.emit(out, AgentEvent{Type: EventAnswer, Text: resp.Content, Step: state.Step})
		a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "thinking → done"})
	}
	return true
}

func (a *Agent) saveModelResponse(state *State, resp *core.ChatResponse, toolCalls []core.ToolCall) {
	if state.Metadata == nil {
		state.Metadata = make(map[string]string)
	}
	if resp.Content != "" {
		state.Metadata["_last_content"] = resp.Content
	}
	if data, err := json.Marshal(toolCalls); err == nil {
		state.Metadata["_last_tool_calls"] = string(data)
	}
}

func (a *Agent) loadModelResponse(state *State) (content string, toolCalls []core.ToolCall) {
	if state.Metadata == nil {
		return "", nil
	}
	content = state.Metadata["_last_content"]
	if raw, ok := state.Metadata["_last_tool_calls"]; ok && raw != "" {
		json.Unmarshal([]byte(raw), &toolCalls)
	}
	delete(state.Metadata, "_last_content")
	delete(state.Metadata, "_last_tool_calls")
	return
}

func (a *Agent) stepAct(ctx context.Context, state *State, out chan<- AgentEvent) bool {
	content, toolCalls := a.loadModelResponse(state)
	a.emit(out, AgentEvent{Type: EventStepStart, Step: state.Step, Text: fmt.Sprintf("acting (%d tools)", len(toolCalls))})

	executedTCs, results, fatal := a.executeToolCalls(ctx, out, toolCalls, state)
	if fatal {
		state.Phase = PhaseError
		a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "acting → error"})
		return false
	}

	if len(executedTCs) > 0 {
		state.Messages = append(state.Messages,
			core.Message{Role: core.Assistant, Content: content, ToolCalls: executedTCs},
		)
		for i, tc := range executedTCs {
			state.Messages = append(state.Messages,
				core.Message{Role: core.ToolRole, Content: results[i], ToolCallID: tc.ID},
			)
		}
	}

	if len(executedTCs) == 0 && len(toolCalls) > 0 {
		errMsg := fmt.Sprintf("工具调用全部失败，请尝试其他方式或直接给出你能提供的最佳答案。")
		state.Messages = append(state.Messages,
			core.Message{Role: core.Assistant, Content: content},
			core.Message{Role: core.ToolRole, Content: errMsg, ToolCallID: "error"},
		)
	}

	if a.hasTools() {
		state.Messages = append(state.Messages,
			core.Message{Role: core.User, Content: fmt.Sprintf("请继续完成目标：%s。如果还有需要工具才能回答的部分，请继续调用工具；如果所有子任务都已完成，请直接给出最终答案。", state.Goal)},
		)
	}

	state.UpdatedAt = time.Now()
	a.checkpoint(ctx, state)
	state.Phase = PhaseThinking
	a.emit(out, AgentEvent{Type: EventStepEnd, Step: state.Step, Text: "acting → thinking"})
	return true
}

func (a *Agent) executeToolCalls(ctx context.Context, out chan<- AgentEvent,
	toolCalls []core.ToolCall, state *State) (executedTCs []core.ToolCall, results []string, fatal bool) {

	for _, tc := range toolCalls {
		select {
		case <-ctx.Done():
			return executedTCs, results, true
		default:
		}

		a.emit(out, AgentEvent{
			Type: EventToolCall, Tool: tc.Name,
			Args: string(tc.Args), Step: state.Step,
		})

		actionKey := fmt.Sprintf("%s:%s", tc.Name, string(tc.Args))
		state.ActionCounts[actionKey]++
		if !a.budget.ShouldRetry(actionKey, state.ActionCounts) {
			lg.Frame.Warn("agent: 动作重复次数过多", lg.Fields{"tool": tc.Name, "key": actionKey})
			errMsg := fmt.Sprintf("工具 %s 重复调用次数过多，已跳过", tc.Name)
			executedTCs = append(executedTCs, core.ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID})
			results = append(results, errMsg)
			a.emit(out, AgentEvent{Type: EventToolResult, Tool: tc.Name, Text: errMsg, Step: state.Step})
			continue
		}

		result, err := a.executeTool(ctx, tc)
		if err != nil {
			lg.Frame.Error("agent: 工具执行失败", lg.Fields{"tool": tc.Name, "error": err})
			errMsg := fmt.Sprintf("工具 %s 执行失败: %v。请尝试其他方式。", tc.Name, err)
			executedTCs = append(executedTCs, core.ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID})
			results = append(results, errMsg)
			a.emit(out, AgentEvent{
				Type: EventToolResult, Tool: tc.Name,
				Text: errMsg, Step: state.Step,
			})
			continue
		}

		executedTCs = append(executedTCs, core.ToolCall{Name: tc.Name, Args: tc.Args, ID: tc.ID, Result: result})
		results = append(results, result)
		a.emit(out, AgentEvent{
			Type: EventToolResult, Tool: tc.Name,
			Text: result, Step: state.Step,
		})
	}

	return executedTCs, results, false
}

func (a *Agent) emit(out chan<- AgentEvent, event AgentEvent) {
	out <- event
}

func (a *Agent) loadState(ctx context.Context) (*State, error) {
	if a.store != nil && a.sessionID != "" {
		state, err := a.store.Load(ctx, a.sessionID)
		if err != nil {
			return nil, err
		}
		return state, nil
	}
	if a.memory != nil {
		return a.memory, nil
	}
	return &State{
		ActionCounts: make(map[string]int),
	}, nil
}

func (a *Agent) buildMessages(state *State) []core.Message {
	var messages []core.Message
	systemContent := a.systemPrompt

	if a.tools != nil && len(a.tools.Tools) > 0 {
		systemContent += "\n\n你可以使用以下工具：\n"
		for name, t := range a.tools.Tools {
			systemContent += fmt.Sprintf("- %s: %s\n", name, t.Description())
		}
		systemContent += "\n使用规则：\n" +
			"1. 不要自己计算或猜测，必须调用工具获取真实结果，如果没有对应工具就输出无可用工具\n" +
			"2. 用户可能要求完成多个子任务，请调用工具完成所有子任务，不要遗漏\n" +
			"3. 如果你已经调用了部分工具，收到结果后仍要检查是否还有其他子任务需要工具\n" +
			"4. 所有子任务完成后，汇总给出最终答案"
	}

	messages = append(messages, core.Message{Role: core.System, Content: systemContent})
	messages = append(messages, core.Message{Role: core.User, Content: state.Goal})
	messages = append(messages, state.Messages...)
	return messages
}

func (a *Agent) callModel(ctx context.Context, messages []core.Message) (*core.ChatResponse, error) {
	cfg := core.LLMConfig{
		Model:   a.model,
		APIKey:  a.apiKey,
		BaseURL: a.baseURL,
	}
	tcp, ok := a.p.(provider.ToolCallProvider)
	if ok && a.tools != nil {
		return tcp.ChatWithTools(ctx, cfg, messages, a.tools.ToolDefs())
	}
	return a.p.Chat(ctx, cfg, messages)
}

func (a *Agent) parseReActToolCalls(content string) []core.ToolCall {
	paradigm := tool.DetectParadigm(content)
	if paradigm == nil {
		paradigm = &tool.ReActParadigm{}
	}
	calls, err := paradigm.Parse(content)
	if err != nil {
		return nil
	}
	return calls
}

func (a *Agent) executeTool(ctx context.Context, tc core.ToolCall) (string, error) {
	if a.tools == nil {
		return "", errutil.NewAgentError(errutil.ErrCategoryTool, "工具注册表为空", nil, false)
	}
	t, ok := a.tools.Get(tc.Name)
	if !ok {
		return "", errutil.NewAgentError(errutil.ErrCategoryToolNotFound, fmt.Sprintf("工具 %s 不存在", tc.Name), nil, false)
	}

	var args map[string]interface{}
	if tc.Args != nil {
		if err := json.Unmarshal(tc.Args, &args); err != nil {
			var argsStr string
			if json.Unmarshal(tc.Args, &argsStr) == nil {
				if json.Unmarshal([]byte(argsStr), &args) != nil {
					return "", errutil.NewAgentError(errutil.ErrCategoryTool, fmt.Sprintf("工具 %s 参数解析失败", tc.Name), err, false)
				}
			} else {
				return "", errutil.NewAgentError(errutil.ErrCategoryTool, fmt.Sprintf("工具 %s 参数解析失败", tc.Name), err, false)
			}
		}
	}

	var result string
	err := errutil.RetryWithBackoff(time.Second, time.Minute, a.budget.MaxRetries, func() error {
		var err error
		var res interface{}
		res, err = t.Call(ctx, args)
		if err == nil {
			result = fmt.Sprintf("%v", res)
		}
		return err
	})
	if err != nil {
		return "", errutil.NewAgentError(errutil.ErrCategoryTool, fmt.Sprintf("工具 %s 执行失败", tc.Name), err, false)
	}
	return result, nil
}

func (a *Agent) hasTools() bool {
	return a.tools != nil && len(a.tools.Tools) > 0
}

func (a *Agent) hasToolHistory(state *State) bool {
	for _, msg := range state.Messages {
		if msg.Role == core.ToolRole || (msg.Role == core.Assistant && len(msg.ToolCalls) > 0) {
			return true
		}
	}
	return false
}

func (a *Agent) calledToolNames(state *State) []string {
	seen := make(map[string]bool)
	for _, msg := range state.Messages {
		if msg.Role == core.Assistant {
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

func (a *Agent) uncalledToolNames(state *State) []string {
	called := make(map[string]bool)
	for _, name := range a.calledToolNames(state) {
		called[name] = true
	}
	var uncalled []string
	if a.tools != nil {
		for name := range a.tools.Tools {
			if !called[name] {
				uncalled = append(uncalled, name)
			}
		}
	}
	return uncalled
}

func (a *Agent) noToolResponseCount(state *State) int {
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

func (a *Agent) setNoToolResponseCount(state *State, count int) {
	if state.Metadata == nil {
		state.Metadata = make(map[string]string)
	}
	state.Metadata["_no_tool_response_count"] = fmt.Sprintf("%d", count)
}

func (a *Agent) checkpoint(ctx context.Context, state *State) {
	state.Messages = dropEmptyAssistantMessages(state.Messages)
	a.memory = state
	if a.store == nil || a.sessionID == "" {
		return
	}
	if err := a.store.Save(ctx, a.sessionID, state); err != nil {
		lg.Frame.Error("checkpoint: 保存状态失败", lg.Fields{"error": err, "session": a.sessionID})
	}
}

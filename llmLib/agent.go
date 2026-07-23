// 文件职责：
// - 实现 Agent 核心运行时，包含 Think-Act-Observe 循环、工具调用、事件流和状态管理。
// - Agent 是一种让模型在运行时持续决策、调用工具、观察结果并继续推进任务的程序结构。
// - 支持 ReAct 和 Function Calling 两种工具调用范式，以及可选的状态持久化。

package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const defaultSystemPrompt = "你是一个命令行 AI 助手。需要真实计算或查询当前时间时，请调用工具。"

// Agent 是 Agent 运行时的核心结构体，包含模型出口、工具集合、预算控制和状态管理。
// Agent = LLM(决策器) + Tools(感知与行动接口) + State(状态) + Loop(自主循环) + Stop(停止条件)
type Agent struct {
	provider     Provider          // 模型出口；传入单个 Provider 或 router 适配器
	model        string            // 模型名
	tools        *Registry         // 可用工具集合
	systemPrompt string            // 系统提示词
	budget       AgentBudgetConfig // 停止条件
	store        Store             // 状态持久化，可选
	sessionID    string            // 会话 ID，配合 store 使用
	memory       *State            // 无持久化存储时的进程内会话状态
}

// New 创建一个新的 Agent 实例，使用函数式选项配置可选项。
// provider: 模型出口，可传入单个 Provider 或 RouterAdapter
// model: 模型名称
// registry: 工具注册表，可为 nil
// opts: 可选配置项，如 WithSystemPrompt、WithAgentBudget、WithStore
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

// Option 是函数式选项模式，用于配置 Agent 的可选项。
type Option func(*Agent)

// WithSystemPrompt 设置系统提示词，用于引导模型行为。
func WithSystemPrompt(prompt string) Option {
	return func(agent *Agent) {
		agent.systemPrompt = prompt
	}
}

// WithAgentBudgetConfig 设置 Agent 的停止条件预算。
func WithAgentBudgetConfig(budget AgentBudgetConfig) Option {
	return func(agent *Agent) {
		agent.budget = budget
	}
}

// WithStore 设置状态持久化存储和会话 ID。
// 当 store 非 nil 且 sessionID 非空时，Agent 会在每次状态更新时自动保存。
func WithStore(store Store, sessionID string) Option {
	return func(agent *Agent) {
		agent.store = store
		agent.sessionID = strings.TrimSpace(sessionID)
	}
}

// Run 启动 Agent 的 Think-Act-Observe 循环，返回事件流供调用方消费。
// ctx: 上下文，用于取消和超时控制
// goal: 用户目标，Agent 将通过多步工具调用来完成此目标
// 返回: 事件流通道，包含思考、工具调用、工具结果、答案增量和完成事件
func (agent *Agent) Run(ctx context.Context, goal string) (<-chan AgentEvent, error) {
	state, err := agent.loadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("load state failed: %w", err)
	}

	if state.StartedAt.IsZero() {
		state.StartedAt = time.Now()
	}
	state.Goal = goal
	state.GoalAdded = true
	state.Phase = PhaseThinking

	out := make(chan AgentEvent)
	go func() {
		defer close(out)

		for !agent.budget.ShouldStop(state) {
			if ctx.Err() != nil {
				return
			}

			messages := agent.buildMessages(state)
			// TODO 完整请求 URL 为空，只拼接了路径 `/chat/completions`，缺少 `baseURL`（域名 + https://）
			resp, err := agent.callModel(ctx, messages)
			if err != nil {
				select {
				case out <- AgentEvent{Type: EventError, Text: err.Error()}:
				case <-ctx.Done():
				}
				return
			}

			state.Usage.InputTokens += resp.InputTokens
			state.Usage.OutputTokens += resp.OutputTokens

			toolCalls := resp.ToolCalls
			if len(toolCalls) == 0 && resp.Content != "" && agent.tools != nil {
				toolCalls = agent.parseReActToolCalls(resp.Content)
			}

			if len(toolCalls) > 0 {
				if resp.Content != "" {
					select {
					case out <- AgentEvent{Type: EventThought, Text: resp.Content, Step: state.Step}:
					case <-ctx.Done():
						return
					}
				}

				state.Phase = PhaseActing
				state.Step++

				for _, tc := range toolCalls {
					select {
					case out <- AgentEvent{
						Type: EventToolCall,
						Tool: tc.Name,
						Args: string(tc.Args),
						Step: state.Step,
					}:
					case <-ctx.Done():
						return
					}

					actionKey := fmt.Sprintf("%s:%s", tc.Name, string(tc.Args))
					state.ActionCounts[actionKey]++

					if !agent.budget.ShouldRetry(actionKey, state.ActionCounts) {
						select {
						case out <- AgentEvent{Type: EventError, Text: fmt.Sprintf("动作 %s 重复次数过多", tc.Name)}:
						case <-ctx.Done():
						}
						return
					}

					result, err := agent.executeTool(ctx, tc)
					if err != nil {
						select {
						case out <- AgentEvent{
							Type: EventError,
							Text: fmt.Sprintf("工具 %s 执行失败: %v", tc.Name, err),
						}:
						case <-ctx.Done():
						}
						continue
					}

					tc.Result = result
					select {
					case out <- AgentEvent{
						Type: EventToolResult,
						Tool: tc.Name,
						Text: result,
						Step: state.Step,
					}:
					case <-ctx.Done():
						return
					}

					state.Messages = append(state.Messages,
						Message{Role: Assistant, Content: resp.Content, ToolCalls: []ToolCall{tc}},
						Message{Role: User, Content: fmt.Sprintf("工具 %s 执行结果: %s", tc.Name, result)},
					)
				}
			} else {
				state.Phase = PhaseDone
				state.Answer = resp.Content
				state.UpdatedAt = time.Now()
				agent.checkpoint(ctx, state)

				select {
				case out <- AgentEvent{Type: EventAnswerDelta, Text: resp.Content, Step: state.Step}:
				case <-ctx.Done():
					return
				}
				goto done
			}

			state.UpdatedAt = time.Now()
			agent.checkpoint(ctx, state)
		}

		if agent.budget.ShouldStop(state) {
			select {
			case out <- AgentEvent{Type: EventError, Text: "预算耗尽"}:
			case <-ctx.Done():
			}
		}

	done:
		select {
		case out <- AgentEvent{Type: EventDone, Step: state.Step}:
		case <-ctx.Done():
		}
	}()

	return out, nil
}

// loadState 加载 Agent 状态，优先从持久化存储加载，其次从内存加载，最后创建新状态。
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

// buildMessages 构建发送给模型的消息列表，包含系统提示词、历史消息和目标。
func (agent *Agent) buildMessages(state *State) []Message {
	var messages []Message
	messages = append(messages, Message{Role: System, Content: agent.systemPrompt})
	messages = append(messages, state.Messages...)
	if !state.GoalAdded {
		messages = append(messages, Message{Role: User, Content: state.Goal})
	}
	return messages
}

// callModel 调用模型，如果 provider 支持工具调用则使用 ChatWithTools，否则使用普通 Chat。
func (agent *Agent) callModel(ctx context.Context, messages []Message) (*ChatResponse, error) {
	tcp, ok := agent.provider.(ToolCallProvider)
	if ok && agent.tools != nil {
		return tcp.ChatWithTools(ctx, LLMConfig{Model: agent.model}, messages, agent.tools.ToolDefs())
	}
	return agent.provider.Chat(ctx, LLMConfig{Model: agent.model}, messages)
}

// parseReActToolCalls 解析 ReAct 格式的工具调用，支持 Action/Action Input 和 <function> 标签两种格式。
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

// executeTool 执行工具调用，支持重试机制，失败时返回 AgentError。
func (agent *Agent) executeTool(ctx context.Context, tc ToolCall) (string, error) {
	tool, ok := agent.tools.Get(tc.Name)
	if !ok {
		return "", NewAgentError(ErrCategoryToolNotFound, fmt.Sprintf("工具 %s 不存在", tc.Name), nil, false)
	}

	var args map[string]interface{}
	if tc.Args != nil {
		if err := json.Unmarshal(tc.Args, &args); err != nil {
			return "", NewAgentError(ErrCategoryTool, fmt.Sprintf("工具 %s 参数解析失败", tc.Name), err, false)
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

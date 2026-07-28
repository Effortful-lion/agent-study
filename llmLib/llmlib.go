package llmlib

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/agent"
	"github.com/Effortful-lion/agent-study/llmLib/command"
	"github.com/Effortful-lion/agent-study/llmLib/core"
	"github.com/Effortful-lion/agent-study/llmLib/errutil"
	"github.com/Effortful-lion/agent-study/llmLib/lg"
	"github.com/Effortful-lion/agent-study/llmLib/pattern"
	"github.com/Effortful-lion/agent-study/llmLib/provider"
	"github.com/Effortful-lion/agent-study/llmLib/router"
	"github.com/Effortful-lion/agent-study/llmLib/signalx"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 类型别名 — 指向 core 子包
// ============================================================================

type LLMConfig = core.LLMConfig
type ChatOption = core.ChatOption
type Message = core.Message
type Role = core.Role
type ToolCall = core.ToolCall
type ToolDef = core.ToolDef
type ToolFunction = core.ToolFunction
type ChatRequest = core.ChatRequest
type ChatResponse = core.ChatResponse
type Usage = core.Usage
type StreamChunk = core.StreamChunk

const (
	User      = core.User
	System    = core.System
	Assistant = core.Assistant
	ToolRole  = core.ToolRole

	API_KEY  = core.API_KEY
	BASE_URL = core.BASE_URL
	MODEL    = core.MODEL

	ProviderOpenAI   = core.ProviderOpenAI
	ProviderDoubao   = core.ProviderDoubao
	ProviderDeepSeek = core.ProviderDeepSeek
	ProviderZhipu    = core.ProviderZhipu
	ProviderTongyi   = core.ProviderTongyi
	ProviderKimi     = core.ProviderKimi
	ProviderClaude   = core.ProviderClaude
	ProviderQwen     = core.ProviderQwen

	OPENAI_API_KEY       = core.OPENAI_API_KEY
	OPENAI_BASE_URL      = core.OPENAI_BASE_URL
	OPENAI_MODEL_ENV     = core.OPENAI_MODEL_ENV
	OPENAI_BASEURL       = core.OPENAI_BASEURL
	OPENAI_DEFAULT_MODEL = core.OPENAI_DEFAULT_MODEL

	DOUBAO_API_KEY       = core.DOUBAO_API_KEY
	DOUBAO_BASE_URL      = core.DOUBAO_BASE_URL
	DOUBAO_MODEL_ENV     = core.DOUBAO_MODEL_ENV
	DOUBAO_BASEURL       = core.DOUBAO_BASEURL
	DOUBAO_DEFAULT_MODEL = core.DOUBAO_DEFAULT_MODEL

	DEEPSEEK_API_KEY       = core.DEEPSEEK_API_KEY
	DEEPSEEK_BASE_URL      = core.DEEPSEEK_BASE_URL
	DEEPSEEK_MODEL_ENV     = core.DEEPSEEK_MODEL_ENV
	DEEPSEEK_BASEURL       = core.DEEPSEEK_BASEURL
	DEEPSEEK_DEFAULT_MODEL = core.DEEPSEEK_DEFAULT_MODEL

	ZHIPU_API_KEY       = core.ZHIPU_API_KEY
	ZHIPU_BASE_URL      = core.ZHIPU_BASE_URL
	ZHIPU_MODEL_ENV     = core.ZHIPU_MODEL_ENV
	ZHIPU_BASEURL       = core.ZHIPU_BASEURL
	ZHIPU_DEFAULT_MODEL = core.ZHIPU_DEFAULT_MODEL

	TONGYI_API_KEY       = core.TONGYI_API_KEY
	TONGYI_BASE_URL      = core.TONGYI_BASE_URL
	TONGYI_MODEL_ENV     = core.TONGYI_MODEL_ENV
	TONGYI_BASEURL       = core.TONGYI_BASEURL
	TONGYI_DEFAULT_MODEL = core.TONGYI_DEFAULT_MODEL

	KIMI_API_KEY       = core.KIMI_API_KEY
	KIMI_BASE_URL      = core.KIMI_BASE_URL
	KIMI_MODEL_ENV     = core.KIMI_MODEL_ENV
	KIMI_BASEURL       = core.KIMI_BASEURL
	KIMI_DEFAULT_MODEL = core.KIMI_DEFAULT_MODEL

	CLAUDE_API_KEY       = core.CLAUDE_API_KEY
	CLAUDE_BASE_URL      = core.CLAUDE_BASE_URL
	CLAUDE_MODEL_ENV     = core.CLAUDE_MODEL_ENV
	CLAUDE_BASEURL       = core.CLAUDE_BASEURL
	CLAUDE_DEFAULT_MODEL = core.CLAUDE_DEFAULT_MODEL

	QWEN_API_KEY       = core.QWEN_API_KEY
	QWEN_BASE_URL      = core.QWEN_BASE_URL
	QWEN_MODEL_ENV     = core.QWEN_MODEL_ENV
	QWEN_BASEURL       = core.QWEN_BASEURL
	QWEN_DEFAULT_MODEL = core.QWEN_DEFAULT_MODEL
)

// ============================================================================
// 类型别名 — 指向 provider 子包
// ============================================================================

type Provider = provider.Provider
type ToolCallProvider = provider.ToolCallProvider

func NewProvider(name string) (Provider, error) {
	return provider.NewProvider(name)
}

// ============================================================================
// 类型别名 — 指向 errutil 子包
// ============================================================================

type AgentError = errutil.AgentError
type ErrorCategory = errutil.ErrorCategory

const (
	ErrCategoryNetwork       = errutil.ErrCategoryNetwork
	ErrCategoryAuth          = errutil.ErrCategoryAuth
	ErrCategoryRateLimited   = errutil.ErrCategoryRateLimited
	ErrCategoryModel         = errutil.ErrCategoryModel
	ErrCategoryTool          = errutil.ErrCategoryTool
	ErrCategoryToolNotFound  = errutil.ErrCategoryToolNotFound
	ErrCategoryTimeout       = errutil.ErrCategoryTimeout
	ErrCategoryCanceled      = errutil.ErrCategoryCanceled
	ErrCategoryNotFound      = errutil.ErrCategoryNotFound
	ErrCategoryProviderError = errutil.ErrCategoryProviderError
	ErrCategoryUnknown       = errutil.ErrCategoryUnknown
)

func NewAgentError(category ErrorCategory, message string, err error, retryable bool) *AgentError {
	return errutil.NewAgentError(category, message, err, retryable)
}

func ClassifyError(err error, statusCode int) (ErrorCategory, bool) {
	return errutil.ClassifyError(err, statusCode)
}

func RetryWithBackoff(baseDelay, maxDelay time.Duration, maxRetries int, fn func() error) error {
	return errutil.RetryWithBackoff(baseDelay, maxDelay, maxRetries, fn)
}

// ============================================================================
// 类型别名 — 指向 signalx 子包
// ============================================================================

func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signalx.SignalContext(parent)
}

// ============================================================================
// 类型别名 — 指向 tool 子包
// ============================================================================

type Tool = tool.Tool
type Registry = tool.Registry
type ToolCallingParadigm = tool.ToolCallingParadigm
type CalculatorTool = tool.CalculatorTool
type TimeTool = tool.TimeTool

func NewRegistryToolSet() *Registry {
	return tool.NewRegistryToolSet()
}

func DetectParadigm(content string) ToolCallingParadigm {
	return tool.DetectParadigm(content)
}

type ReActParadigm = tool.ReActParadigm
type FunctionCallingParadigm = tool.FunctionCallingParadigm
type JSONSchemaTool = tool.JSONSchemaTool
type SchemaTool = tool.SchemaTool
type Schema = tool.Schema

func NewJSONSchemaTool(name, description string, parametersJSON json.RawMessage, callFn func(ctx context.Context, args map[string]any) (any, error)) *JSONSchemaTool {
	return tool.NewJSONSchemaTool(name, description, parametersJSON, callFn)
}

func BuildToolDefs(registry *Registry) []ToolDef {
	return tool.BuildToolDefs(registry)
}

func BuildArgs(argsJSON string) (map[string]any, error) {
	return tool.BuildArgs(argsJSON)
}

func StructToMap(v any) map[string]any {
	return tool.StructToMap(v)
}

func GenerateSchema(v any) *Schema {
	return tool.Generate(v)
}

func Generate(v any) *Schema {
	return tool.Generate(v)
}

// ============================================================================
// 类型别名 — 指向 agent 子包
// ============================================================================

type Agent = agent.Agent
type AgentEvent = agent.AgentEvent
type EventType = agent.EventType
type Phase = agent.Phase
type State = agent.State
type Store = agent.Store
type FileStore = agent.FileStore
type AgentBudgetConfig = agent.AgentBudgetConfig
type Option = agent.Option
type Task = agent.Task
type Plan = agent.Plan

const (
	EventStepStart     = agent.EventStepStart
	EventStepEnd       = agent.EventStepEnd
	EventModelCall     = agent.EventModelCall
	EventModelResponse = agent.EventModelResponse
	EventToolCall      = agent.EventToolCall
	EventToolResult    = agent.EventToolResult
	EventThought       = agent.EventThought
	EventAnswer        = agent.EventAnswer
	EventError         = agent.EventError
	EventDone          = agent.EventDone

	PhaseThinking = agent.PhaseThinking
	PhaseActing   = agent.PhaseActing
	PhaseDone     = agent.PhaseDone
	PhaseError    = agent.PhaseError
)

func New(p Provider, model string, registry *Registry, opts ...Option) *Agent {
	return agent.New(p, model, registry, opts...)
}

func WithSystemPrompt(prompt string) Option {
	return agent.WithSystemPrompt(prompt)
}

func WithAgentBudgetConfig(budget AgentBudgetConfig) Option {
	return agent.WithAgentBudgetConfig(budget)
}

func WithAgentAPIKey(apiKey string) Option {
	return agent.WithAgentAPIKey(apiKey)
}

func WithAgentBaseURL(baseURL string) Option {
	return agent.WithAgentBaseURL(baseURL)
}

func WithStore(store Store, sessionID string) Option {
	return agent.WithStore(store, sessionID)
}

func DefaultAgentBudgetConfig() AgentBudgetConfig {
	return agent.DefaultAgentBudgetConfig()
}

func NewFileStore(dir string) *FileStore {
	return agent.NewFileStore(dir)
}

func Levels(plan Plan) ([][]Task, error) {
	return agent.Levels(plan)
}

func ExecutePlan(ctx context.Context, plan Plan) (map[string]any, error) {
	return agent.Execute(ctx, plan)
}

// ============================================================================
// 类型别名 — 指向 pattern 子包
// ============================================================================

type Pattern = pattern.Pattern
type PatternEvent = pattern.Event
type Chain = pattern.Chain
type Evaluator = pattern.Evaluator
type Parallel = pattern.Parallel
type Orchestrator = pattern.Orchestrator

func NewChain() *Chain {
	return pattern.NewChain()
}

func NewEvaluator() *Evaluator {
	return pattern.NewEvaluator()
}

func NewParallel() *Parallel {
	return pattern.NewParallel()
}

func NewOrchestrator() *Orchestrator {
	return pattern.NewOrchestrator()
}

func NewPatternRouter(routerFn func(ctx context.Context, state map[string]any) (string, error)) *pattern.Router {
	return pattern.NewRouter(routerFn)
}

// ============================================================================
// 类型别名 — 指向 router 子包
// ============================================================================

type Strategy = router.Strategy
type LLMService = router.LLMService
type RouteResult = router.RouteResult
type RouteStreamChunk = router.RouteStreamChunk
type Router = router.Router
type RouterAdapter = router.RouterAdapter
type LatencySnapshot = router.LatencySnapshot
type LatencyMetrics = router.LatencyMetrics

const (
	StrategyDefault       = router.StrategyDefault
	StrategyCheapestFirst = router.StrategyCheapestFirst
	StrategyLowestLatency = router.StrategyLowestLatency
)

func NewRouter(services []LLMService, strategy Strategy) *Router {
	return router.NewRouter(services, strategy)
}

func NewRouterAdapter(r *Router) *RouterAdapter {
	return router.NewRouterAdapter(r)
}

func NewLatencyMetrics() *LatencyMetrics {
	return router.NewLatencyMetrics()
}

func LoadAll() ([]LLMService, error) {
	return router.LoadAll()
}

func LoadAllWithEnv(envPath string) ([]LLMService, error) {
	return router.LoadAllWithEnv(envPath)
}

func LoadDotEnv() error {
	return router.LoadDotEnv()
}

func LoadDotEnvFromPath(path string) error {
	return router.LoadDotEnvFromPath(path)
}

func ProviderConfigHelp() string {
	return router.ProviderConfigHelp()
}

func ReadStrategyFromEnv() Strategy {
	return router.ReadStrategyFromEnv()
}

// ============================================================================
// 类型别名 — 指向 command 子包
// ============================================================================

type CommandBuilder = command.CommandBuilder

func NewCommandBuilder() *CommandBuilder {
	return command.NewCommandBuilder()
}

func LoadCommands() *CommandBuilder {
	return command.LoadCommands()
}

// ============================================================================
// 兼容层 — 保留向后兼容的函数
// ============================================================================

func SafeJSON(v any) string {
	return core.SafeJSON(v)
}

func WithModel(model string) ChatOption {
	return core.WithModel(model)
}

func WithBaseURL(baseURL string) ChatOption {
	return core.WithBaseURL(baseURL)
}

func WithAPIKey(apiKey string) ChatOption {
	return core.WithAPIKey(apiKey)
}

func WithInputPrice(price float64) ChatOption {
	return core.WithInputPrice(price)
}

func WithOutputPrice(price float64) ChatOption {
	return core.WithOutputPrice(price)
}

func WithLatencyMS(latency int) ChatOption {
	return core.WithLatencyMS(latency)
}

func NewMessage(role Role, content string) Message {
	return core.NewMessage(role, content)
}

func NewUserMessage(content string) Message {
	return core.NewUserMessage(content)
}

func NewSystemMessage(content string) Message {
	return core.NewSystemMessage(content)
}

func NewAssistantMessage(content string) Message {
	return core.NewAssistantMessage(content)
}

func Process[T any](ctx context.Context, ch <-chan T, handler func(T) error) error {
	return core.Process(ctx, ch, handler)
}

func Collect[T any](ctx context.Context, ch <-chan T) ([]T, error) {
	return core.Collect(ctx, ch)
}

// ============================================================================
// 主调用入口
// ============================================================================

func Chat(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (*ChatResponse, error) {
	p, err := NewProvider(providerName)
	if err != nil {
		lg.Frame.Error("Chat: 创建 provider 失败", lg.Fields{"provider": providerName, "error": err})
		return nil, fmt.Errorf("chat: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}
	if cfg.Model == "" {
		cfg.Model = getDefaultModel(providerName)
	}

	return p.Chat(ctx, cfg, messages)
}

func ChatStream(ctx context.Context, providerName string, apiKey string, messages []Message, opts ...ChatOption) (<-chan StreamChunk, error) {
	p, err := NewProvider(providerName)
	if err != nil {
		lg.Frame.Error("ChatStream: 创建 provider 失败", lg.Fields{"provider": providerName, "error": err})
		return nil, fmt.Errorf("chat stream: %w", err)
	}

	cfg := LLMConfig{
		APIKey: apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = getDefaultBaseURL(providerName)
	}
	if cfg.Model == "" {
		cfg.Model = getDefaultModel(providerName)
	}

	return p.ChatStream(ctx, cfg, messages)
}

func getDefaultBaseURL(providerName string) string {
	switch providerName {
	case ProviderOpenAI:
		return OPENAI_BASEURL
	case ProviderDoubao:
		return DOUBAO_BASEURL
	case ProviderDeepSeek:
		return DEEPSEEK_BASEURL
	case ProviderClaude:
		return CLAUDE_BASEURL
	case ProviderZhipu:
		return ZHIPU_BASEURL
	case ProviderTongyi:
		return TONGYI_BASEURL
	case ProviderKimi:
		return KIMI_BASEURL
	case ProviderQwen:
		return QWEN_BASEURL
	default:
		return ""
	}
}

func getDefaultModel(providerName string) string {
	switch providerName {
	case ProviderOpenAI:
		return OPENAI_DEFAULT_MODEL
	case ProviderDoubao:
		return DOUBAO_DEFAULT_MODEL
	case ProviderDeepSeek:
		return DEEPSEEK_DEFAULT_MODEL
	case ProviderClaude:
		return CLAUDE_DEFAULT_MODEL
	case ProviderZhipu:
		return ZHIPU_DEFAULT_MODEL
	case ProviderTongyi:
		return TONGYI_DEFAULT_MODEL
	case ProviderKimi:
		return KIMI_DEFAULT_MODEL
	case ProviderQwen:
		return QWEN_DEFAULT_MODEL
	default:
		return ""
	}
}

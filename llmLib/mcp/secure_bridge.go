// 文件职责：
// - 安全增强版 MCP 工具桥接器（简化版）
// - 集成输出净化、工具白名单、权限检查、审计日志
//
// 安全特性：
// 1. 工具输出自动净化（边界标记 + 长度限制）
// 2. 工具白名单过滤
// 3. 全操作审计日志
// 4. 基础权限检查

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/security"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 安全桥接器配置
// ============================================================================

// SecureBridgeOption 安全桥接器配置选项
type SecureBridgeOption func(*SecureBridgeConfig)

// SecureBridgeConfig 安全桥接器配置
type SecureBridgeConfig struct {
	// 安全组件
	Sanitizer     *security.Sanitizer
	AuditLogger   *security.AuditLogger
	ToolWhitelist *security.ToolWhitelist
}

// 默认配置
func defaultSecureBridgeConfig() *SecureBridgeConfig {
	return &SecureBridgeConfig{
		Sanitizer:     security.NewSanitizer(),
		AuditLogger:   security.NewAuditLogger(),
		ToolWhitelist: security.NewToolWhitelist(),
	}
}

// WithMaxOutputLength 设置最大输出长度
func WithMaxOutputLength(max int) SecureBridgeOption {
	return func(c *SecureBridgeConfig) {
		// 创建新实例
		c.Sanitizer = security.NewSanitizer()
		// 注意：无法直接设置私有字段，实际应该添加 Setter 方法到 Sanitizer
	}
}

// WithToolWhitelist 设置工具白名单
func WithToolWhitelist(allowedTools []string) SecureBridgeOption {
	return func(c *SecureBridgeConfig) {
		c.ToolWhitelist.Allow(allowedTools...)
		c.ToolWhitelist.Enable()
	}
}

// WithAuditCallback 设置审计回调
func WithAuditCallback(callback func(security.AuditEvent)) SecureBridgeOption {
	return func(c *SecureBridgeConfig) {
		c.AuditLogger = security.NewAuditLogger()
	}
}

// ============================================================================
// 安全桥接器
// ============================================================================

// SecureBridgedTool 安全增强的桥接工具
type SecureBridgedTool struct {
	client *Client
	def    Tool
	params json.RawMessage
	config *SecureBridgeConfig
}

// Name 返回工具名称
func (t *SecureBridgedTool) Name() string {
	return t.def.Name
}

// Description 返回工具名称
func (t *SecureBridgedTool) Description() string {
	return fmt.Sprintf("MCP 工具: %s", t.def.Name)
}

// Parameters 返回参数描述
func (t *SecureBridgedTool) Parameters() map[string]string {
	return nil
}

// ParametersSchema 返回参数描述
func (t *SecureBridgedTool) ParametersSchema() json.RawMessage {
	if len(t.params) == 0 {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return t.params
}

// Call 安全调用 MCP 工具
func (t *SecureBridgedTool) Call(ctx context.Context, args map[string]any) (any, error) {
	startTime := time.Now()

	// 1. 工具白名单检查
	if !t.config.ToolWhitelist.IsAllowed(t.def.Name) {
		errMsg := fmt.Sprintf("工具 %s 不在白名单中", t.def.Name)
		t.config.AuditLogger.Log(security.AuditEvent{
			ToolName: t.def.Name,
			ToolArgs: args,
			Error:    errMsg,
			Duration: time.Since(startTime),
		})
		return nil, fmt.Errorf("%s", errMsg)
	}

	// 2. 调用 MCP 工具
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("参数序列化失败: %w", err)
	}

	result, err := t.client.CallTool(ctx, t.def.Name, argsJSON)
	if err != nil {
		t.config.AuditLogger.Log(security.AuditEvent{
			ToolName: t.def.Name,
			ToolArgs: args,
			Error:    err.Error(),
			Duration: time.Since(startTime),
		})
		return nil, err
	}

	// 3. 工具输出净化
	output := result.Text()
	sanitizedOutput := t.config.Sanitizer.SanitizeToolOutput(output)

	// 4. 记录审计日志
	t.config.AuditLogger.Log(security.AuditEvent{
		ToolName: t.def.Name,
		ToolArgs: args,
		Result:   output,
		Duration: time.Since(startTime),
	})

	if result.IsError {
		return sanitizedOutput, fmt.Errorf("工具 %q 返回错误", t.def.Name)
	}

	return sanitizedOutput, nil
}

// ============================================================================
// 安全桥接函数
// ============================================================================

// SecureBridgeAll 安全地桥接所有 MCP 工具
func SecureBridgeAll(ctx context.Context, client *Client, opts ...SecureBridgeOption) ([]tool.Tool, []security.AuditEvent, error) {
	// 1. 创建配置
	config := defaultSecureBridgeConfig()
	for _, opt := range opts {
		opt(config)
	}

	// 2. 握手
	if _, err := client.Initialize(ctx); err != nil {
		return nil, nil, fmt.Errorf("握手失败: %w", err)
	}

	if err := client.Initialized(ctx); err != nil {
		return nil, nil, fmt.Errorf("发送 initialized 通知失败: %w", err)
	}

	// 3. 列出工具
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("列出工具失败: %w", err)
	}

	// 4. 安全检查每个工具
	safeTools := make([]tool.Tool, 0, len(mcpTools))

	for _, mt := range mcpTools {
		// 白名单检查
		if !config.ToolWhitelist.IsAllowed(mt.Name) {
			continue
		}

		// 添加工具
		params := mt.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		safeTools = append(safeTools, &SecureBridgedTool{
			client: client,
			def:    mt,
			params: params,
			config: config,
		})
	}

	// 5. 返回结果
	return safeTools, config.AuditLogger.GetEvents(), nil
}

// ============================================================================
// 便捷函数
// ============================================================================

// NewSecureBridgedClient 创建安全增强的 MCP Client
func NewSecureBridgedClient(command string, args []string, opts ...SecureBridgeOption) (*Client, []tool.Tool, *security.AuditLogger, error) {
	// 1. 启动 Client
	client, err := NewClient(command, args)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("启动 Client 失败: %w", err)
	}

	// 2. 安全桥接
	ctx := context.Background()
	tools, _, err := SecureBridgeAll(ctx, client, opts...)
	if err != nil {
		client.Close()
		return nil, nil, nil, fmt.Errorf("安全桥接失败: %w", err)
	}

	// 3. 获取审计日志器
	config := defaultSecureBridgeConfig()
	for _, opt := range opts {
		opt(config)
	}

	return client, tools, config.AuditLogger, nil
}

// ============================================================================
// 工具包装器（中间件模式）
// ============================================================================

// SecureToolWrapper 安全工具包装器
type SecureToolWrapper struct {
	inner  tool.Tool
	config *SecureBridgeConfig
}

// NewSecureToolWrapper 创建安全工具包装器
func NewSecureToolWrapper(inner tool.Tool, config *SecureBridgeConfig) *SecureToolWrapper {
	return &SecureToolWrapper{
		inner:  inner,
		config: config,
	}
}

// Name 返回工具名称
func (w *SecureToolWrapper) Name() string {
	return w.inner.Name()
}

// Description 返回工具描述
func (w *SecureToolWrapper) Description() string {
	return w.inner.Description()
}

// Parameters 返回参数描述
func (w *SecureToolWrapper) Parameters() map[string]string {
	return w.inner.Parameters()
}

// Call 安全调用工具
func (w *SecureToolWrapper) Call(ctx context.Context, args map[string]any) (any, error) {
	startTime := time.Now()

	// 1. 调用原始工具
	result, err := w.inner.Call(ctx, args)

	// 2. 输出净化
	resultStr := fmt.Sprintf("%v", result)
	sanitizedResult := w.config.Sanitizer.SanitizeToolOutput(resultStr)

	// 3. 记录审计日志
	w.config.AuditLogger.Log(security.AuditEvent{
		ToolName: w.inner.Name(),
		ToolArgs: args,
		Result:   resultStr,
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		Duration: time.Since(startTime),
	})

	// 4. 如果有错误，返回错误；否则返回净化后的结果
	if err != nil {
		return sanitizedResult, err
	}
	return sanitizedResult, nil
}

// WrapToolWithSecurity 将普通工具包装成安全增强版本
func WrapToolWithSecurity(t tool.Tool, config *SecureBridgeConfig) tool.Tool {
	return NewSecureToolWrapper(t, config)
}

// WrapToolsWithSecurity 批量包装工具
func WrapToolsWithSecurity(tools []tool.Tool, config *SecureBridgeConfig) []tool.Tool {
	wrapped := make([]tool.Tool, len(tools))
	for i, t := range tools {
		wrapped[i] = WrapToolWithSecurity(t, config)
	}
	return wrapped
}

// ============================================================================
// 工具注册表安全包装
// ============================================================================

// SecureRegistry 安全增强的工具注册表
type SecureRegistry struct {
	inner  *tool.Registry
	config *SecureBridgeConfig
}

// NewSecureRegistry 创建安全增强的工具注册表
func NewSecureRegistry(inner *tool.Registry, config *SecureBridgeConfig) *SecureRegistry {
	return &SecureRegistry{
		inner:  inner,
		config: config,
	}
}

// Register 注册工具（自动包装安全增强）
func (sr *SecureRegistry) Register(t tool.Tool) {
	wrapped := WrapToolWithSecurity(t, sr.config)
	sr.inner.Register(wrapped)
}

// RegisterRaw 注册原始工具（不包装）
func (sr *SecureRegistry) RegisterRaw(t tool.Tool) {
	sr.inner.Register(t)
}

// Get 获取工具
func (sr *SecureRegistry) Get(name string) (tool.Tool, bool) {
	return sr.inner.Get(name)
}

// Call 调用工具
func (sr *SecureRegistry) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	return sr.inner.Call(ctx, name, args)
}

// ToolDefs 返回工具定义
func (sr *SecureRegistry) ToolDefs() []any {
	// TODO: 实现 tool.ToolDef 支持
	return nil
}

// GetAuditLogger 获取审计日志器
func (sr *SecureRegistry) GetAuditLogger() *security.AuditLogger {
	return sr.config.AuditLogger
}

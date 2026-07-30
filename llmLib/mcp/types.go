// 文件职责：
// - MCP 核心类型定义
// - JSON-RPC 2.0 消息结构
// - MCP 协议相关类型

package mcp

import (
	"encoding/json"
	"strings"
)

// ============================================================================
// JSON-RPC 2.0 基础类型
// ============================================================================

// rpcRequest 表示 JSON-RPC 请求
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"` // 固定 "2.0"
	ID      *int            `json:"id"`      // 请求 ID（指针：通知无 ID）
	Method  string          `json:"method"`  // 方法名
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcResponse 表示 JSON-RPC 响应
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"` // 固定 "2.0"
	ID      *int            `json:"id"`      // 响应 ID（指针：通知无 ID）
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError 表示 JSON-RPC 错误
type rpcError struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
	Data    any    `json:"data,omitempty"`
}

// ============================================================================
// MCP 核心类型
// ============================================================================

// InitializeParams 是 initialize 方法的参数
type InitializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"` // 协议版本，如 "2025-11-25"
	ClientInfo      ClientInfo        `json:"clientInfo"`      // Client 信息
	Capabilities    ClientCapabilities `json:"capabilities"`    // Client 能力
}

// ClientInfo Client 信息
type ClientInfo struct {
	Name    string `json:"name"`    // 名称
	Version string `json:"version"` // 版本
}

// ClientCapabilities Client 能力声明
type ClientCapabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// RootsCapability Roots 能力
type RootsCapability struct {
	ListChanged bool `json:"listChanged"` // 是否支持根目录变化通知
}

// SamplingCapability Sampling 能力
type SamplingCapability struct{}

// InitializeResult initialize 方法的结果
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"` // 协议版本
	ServerInfo      ServerInfo       `json:"serverInfo"`      // Server 信息
	Capabilities    ServerCapabilities `json:"capabilities"`   // Server 能力
}

// ServerInfo Server 信息
type ServerInfo struct {
	Name    string `json:"name"`    // Server 名称
	Version string `json:"version"` // Server 版本
}

// ServerCapabilities Server 能力声明
type ServerCapabilities struct {
	Tools        *ToolsCapability        `json:"tools,omitempty"`
	Resources    *ResourcesCapability    `json:"resources,omitempty"`
	Prompts      *PromptsCapability      `json:"prompts,omitempty"`
	Logging      *LoggingCapability      `json:"logging,omitempty"`
	Experimental map[string]any          `json:"experimental,omitempty"`
}

// ToolsCapability 工具能力
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"` // 是否支持工具列表变化通知
}

// ResourcesCapability 资源能力
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`   // 是否支持订阅
	ListChanged bool `json:"listChanged"` // 是否支持资源列表变化通知
}

// PromptsCapability 提示词能力
type PromptsCapability struct {
	ListChanged bool `json:"listChanged"` // 是否支持提示词列表变化通知
}

// LoggingCapability 日志能力
type LoggingCapability struct{}

// ============================================================================
// Tools 相关类型
// ============================================================================

// Tool 工具定义
type Tool struct {
	Name        string          `json:"name"`        // 工具名称
	Description string          `json:"description"` // 工具描述
	InputSchema json.RawMessage `json:"inputSchema"` // 输入参数 Schema
}

// ListToolsResult tools/list 方法的结果
type ListToolsResult struct {
	Tools []Tool `json:"tools"` // 工具列表
}

// CallToolParams tools/call 方法的参数
type CallToolParams struct {
	Name      string          `json:"name"`      // 工具名称
	Arguments json.RawMessage `json:"arguments"` // 工具参数
}

// ContentBlock 内容块
type ContentBlock struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text,omitempty"`
}

// CallToolResult tools/call 方法的结果
type CallToolResult struct {
	Content []ContentBlock `json:"content"` // 内容块数组
	IsError bool           `json:"isError,omitempty"`
}

// Text 返回文本结果
func (r CallToolResult) Text() string {
	var sb stringsBuilder
	for _, block := range r.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// ============================================================================
// Resources 相关类型
// ============================================================================

// Resource 资源定义
type Resource struct {
	URI         string `json:"uri"`          // 资源 URI
	Name        string `json:"name"`         // 资源名称
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult resources/list 方法的结果
type ListResourcesResult struct {
	Resources []Resource `json:"resources"` // 资源列表
	NextCursor string    `json:"nextCursor,omitempty"`
}

// ReadResourceParams resources/read 方法的参数
type ReadResourceParams struct {
	URI string `json:"uri"` // 资源 URI
}

// ResourceContents 资源内容
type ResourceContents struct {
	URI      string `json:"uri"`      // 资源 URI
	MimeType string `json:"mimeType"` // MIME 类型
	Text     string `json:"text,omitempty"`
}

// ReadResourceResult resources/read 方法的结果
type ReadResourceResult struct {
	Contents []ResourceContents `json:"contents"` // 资源内容列表
}

// ============================================================================
// Prompts 相关类型
// ============================================================================

// Prompt 提示词模板
type Prompt struct {
	Name        string              `json:"name"`        // 提示词名称
	Description string              `json:"description"` // 提示词描述
	Arguments   []PromptArgument    `json:"arguments,omitempty"`
}

// PromptArgument 提示词参数
type PromptArgument struct {
	Name        string `json:"name"`        // 参数名称
	Description string `json:"description"` // 参数描述
	Required    bool   `json:"required"`    // 是否必需
}

// ListPromptsResult prompts/list 方法的结果
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"` // 提示词列表
}

// GetPromptParams prompts/get 方法的参数
type GetPromptParams struct {
	Name      string            `json:"name"`      // 提示词名称
	Arguments map[string]string `json:"arguments"` // 参数
}

// GetPromptResult prompts/get 方法的结果
type GetPromptResult struct {
	Description string     `json:"description"`
	Messages    []Message  `json:"messages"` // 填充后的消息列表
}

// Message 消息
type Message struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content any    `json:"content"` // string 或 ContentBlock
}

// ============================================================================
// Logging 相关类型
// ============================================================================

// LoggingMessage 日志消息
type LoggingMessage struct {
	Level  string `json:"level"`  // "debug" | "info" | "notice" | "warning" | "error" | "critical" | "alert" | "emergency"
	Data   string `json:"data"`   // 日志数据
	Logger string `json:"logger"` // 日志器名称
}

// SetLevelParams logging/setLevel 的参数
type SetLevelParams struct {
	Level string `json:"level"` // 日志级别
}

// ============================================================================
// 辅助类型
// ============================================================================

// stringsBuilder 辅助类型，用于字符串拼接
type stringsBuilder struct {
	sb strings.Builder
}

func (b *stringsBuilder) WriteString(s string) {
	b.sb.WriteString(s)
}

func (b *stringsBuilder) String() string {
	return b.sb.String()
}

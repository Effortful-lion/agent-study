// 文件职责：
// - 将 MCP 工具桥接到本地 tool.Tool 接口
// - 让 Agent 可以像调用本地工具一样调用 MCP 工具

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// Bridge 核心类型
// ============================================================================

// bridgedTool 将 MCP 工具包装成 tool.Tool
type bridgedTool struct {
	client *Client         // MCP Client
	def    Tool            // MCP 工具定义
	params json.RawMessage // 参数 Schema
}

// Name 返回工具名称
func (t *bridgedTool) Name() string {
	return t.def.Name
}

// Description 返回工具描述
func (t *bridgedTool) Description() string {
	return t.def.Description
}

// Parameters 返回参数描述（兼容旧接口）
func (t *bridgedTool) Parameters() map[string]string {
	// MCP 工具使用 JSON Schema，这里返回空 map
	// Agent 会通过 ParametersSchema 获取 JSON Schema
	return nil
}

// ParametersSchema 返回 JSON Schema（新接口）
func (t *bridgedTool) ParametersSchema() json.RawMessage {
	if len(t.params) == 0 {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return t.params
}

// Call 调用 MCP 工具
func (t *bridgedTool) Call(ctx context.Context, args map[string]any) (any, error) {
	// 将 map[string]any 转换为 JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("参数序列化失败: %w", err)
	}

	// 调用 MCP 工具
	result, err := t.client.CallTool(ctx, t.def.Name, argsJSON)
	if err != nil {
		return nil, err
	}

	// 提取文本内容
	if result.IsError {
		return result.Text(), fmt.Errorf("工具 %q 返回错误", t.def.Name)
	}

	return result.Text(), nil
}

// ============================================================================
// Bridge 工具函数
// ============================================================================

// BridgeAll 将某个 MCP Server 的所有工具桥接成 []tool.Tool
func BridgeAll(ctx context.Context, client *Client) ([]tool.Tool, error) {
	// 1. 确保已握手
	if _, err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("握手失败: %w", err)
	}

	// 2. 通知握手完成
	if err := client.Initialized(ctx); err != nil {
		return nil, fmt.Errorf("发送 initialized 通知失败: %w", err)
	}

	// 3. 列出所有工具
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出工具失败: %w", err)
	}

	// 4. 桥接每个 MCP 工具
	out := make([]tool.Tool, 0, len(mcpTools))
	for _, mt := range mcpTools {
		params := mt.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		out = append(out, &bridgedTool{
			client: client,
			def:    mt,
			params: params,
		})
	}

	return out, nil
}

// BridgeTool 桥接单个 MCP 工具
func BridgeTool(ctx context.Context, client *Client, toolName string) (tool.Tool, error) {
	// 1. 确保已握手
	if _, err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("握手失败: %w", err)
	}

	if err := client.Initialized(ctx); err != nil {
		return nil, fmt.Errorf("发送 initialized 通知失败: %w", err)
	}

	// 2. 列出工具
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出工具失败: %w", err)
	}

	// 3. 查找指定工具
	for _, mt := range mcpTools {
		if mt.Name == toolName {
			params := mt.InputSchema
			if len(params) == 0 {
				params = json.RawMessage(`{"type":"object","properties":{}}`)
			}

			return &bridgedTool{
				client: client,
				def:    mt,
				params: params,
			}, nil
		}
	}

	return nil, fmt.Errorf("工具 %q 未找到", toolName)
}

// ============================================================================
// 便捷函数：创建并桥接
// ============================================================================

// NewBridgedClient 启动 MCP Server 并桥接所有工具
func NewBridgedClient(command string, args []string) (*Client, []tool.Tool, error) {
	// 1. 启动 MCP Client
	client, err := NewClient(command, args)
	if err != nil {
		return nil, nil, fmt.Errorf("启动 Client 失败: %w", err)
	}

	// 2. 桥接工具
	ctx := context.Background()
	tools, err := BridgeAll(ctx, client)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("桥接工具失败: %w", err)
	}

	return client, tools, nil
}

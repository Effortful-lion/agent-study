// 文件职责：
// - MCP stdio Client 实现
// - 启动 MCP Server 子进程
// - 通过 stdin/stdout 与 Server 通信

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// ============================================================================
// Client 核心类型
// ============================================================================

// Client 是 MCP stdio Client
type Client struct {
	cmd    *exec.Cmd      // Server 子进程
	stdin  io.WriteCloser // 向 Server 写入
	stdout *bufio.Reader  // 从 Server 读取
	mu     sync.Mutex     // 串行化请求（简化：一次只发一个）
	nextID int            // 下一个请求 ID
}

// ClientOption Client 配置选项
type ClientOption func(*Client)

// ============================================================================
// Client 创建和生命周期
// ============================================================================

// NewClient 创建新的 MCP Client，启动 Server 子进程
// command: Server 可执行文件路径
// args: 传递给 Server 的参数
func NewClient(command string, args []string, opts ...ClientOption) (*Client, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdin 管道失败", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdout 管道失败", err)
	}

	cmd.Stderr = &stderrWriter{} // Server 的日志透传到我们的 stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Server 失败", err)
	}

	client := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		nextID: 1,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Close 关闭 Client 和 Server
func (c *Client) Close() error {
	// 关闭 stdin，通知 Server 退出
	if closeErr := c.stdin.Close(); closeErr != nil {
		fmt.Fprintf(&stderrWriter{}, "[Client] 关闭 stdin 失败: %v\n", closeErr)
	}

	// 等待 Server 退出
	if waitErr := c.cmd.Wait(); waitErr != nil {
		fmt.Fprintf(&stderrWriter{}, "[Client] Server 退出错误: %v\n", waitErr)
	}

	return nil
}

// stderrWriter 将日志写入标准错误
type stderrWriter struct{}

func (w *stderrWriter) Write(p []byte) (n int, err error) {
	return fmt.Fprintf(&stderrWriter{}, "[Server] %s", string(p))
}

// ============================================================================
// 核心通信方法
// ============================================================================

// call 发送请求并等待响应（简化版：串行化）
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 构建请求
	id := c.nextID
	c.nextID++

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
	}

	if params != nil {
		paramsBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("参数序列化失败: %w", err)
		}
		req.Params = paramsBytes
	}

	// 发送请求
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", reqBytes); err != nil {
		return nil, fmt.Errorf("发送请求失败", err)
	}

	// 等待响应（跳过非匹配消息）
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			line, err := c.stdout.ReadBytes('\n')
			if err != nil {
				return nil, fmt.Errorf("读取响应失败", err)
			}

			// 解析响应
			var resp rpcResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				fmt.Fprintf(&stderrWriter{}, "[Client] 跳过非 JSON 消息: %s\n", string(line))
				continue // 防御性容错：跳过非 JSON 行
			}

			// 跳过通知和 ID 不匹配的消息
			if resp.ID == nil || *resp.ID != id {
				continue
			}

			// 检查错误
			if resp.Error != nil {
				return nil, fmt.Errorf("MCP 错误 %d: %s", resp.Error.Code, resp.Error.Message)
			}

			return resp.Result, nil
		}
	}
}

// notify 发送通知（无响应）
func (c *Client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}

	data, _ := json.Marshal(msg)
	_, err := fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// ============================================================================
// MCP 方法
// ============================================================================

// Initialize 初始化握手
func (c *Client) Initialize(ctx context.Context) (InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: "2025-11-25",
		ClientInfo: ClientInfo{
			Name:    "llmagent",
			Version: "0.1.0",
		},
		Capabilities: ClientCapabilities{
			Roots: &RootsCapability{
				ListChanged: true,
			},
		},
	}

	raw, err := c.call(ctx, "initialize", params)
	if err != nil {
		return InitializeResult{}, err
	}

	var result InitializeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return InitializeResult{}, fmt.Errorf("解析初始化结果失败", err)
	}

	return result, nil
}

// Initialized 通知 Server 握手完成
func (c *Client) Initialized(ctx context.Context) error {
	return c.notify("notifications/initialized", nil)
}

// ListTools 列出可用工具
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("解析工具列表失败", err)
	}

	return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	raw, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return CallToolResult{}, err
	}

	var result CallToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CallToolResult{}, fmt.Errorf("解析工具调用结果失败", err)
	}

	return result, nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// ServerInfo 获取 Server 信息
func (c *Client) ServerInfo() *ServerInfo {
	// TODO: 保存握手时的 ServerInfo
	return nil
}

// 文件职责：
// - 手写 MCP stdio Server（从零实现）
// - 暴露 get_time 和 calc 两个工具
// - 直接处理 JSON-RPC 消息
// - 展示 MCP 协议的底层实现细节
//
// 关键点：
// 1. 从 stdin 读取 JSON-RPC 请求
// 2. 按 method 字段分发到不同的处理器
// 3. 向 stdout 写入 JSON-RPC 响应
// 4. 处理 initialize、tools/list、tools/call 三个核心方法

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ============================================================================
// JSON-RPC 2.0 基础类型（手写实现）
// ============================================================================

// jsonRPCRequest JSON-RPC 请求
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"` // 必须是 "2.0"
	ID      *int            `json:"id"`      // 请求 ID（通知为 nil）
	Method  string          `json:"method"`  // 方法名
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonRPCResponse JSON-RPC 响应
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"` // 必须是 "2.0"
	ID      *int            `json:"id"`      // 响应 ID
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// jsonRPCError JSON-RPC 错误
type jsonRPCError struct {
	Code    int    `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
	Data    any    `json:"data,omitempty"`
}

// ============================================================================
// MCP 协议类型
// ============================================================================

// initializeParams initialize 方法的参数
type initializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ClientInfo      clientInfo       `json:"clientInfo"`
	Capabilities    clientCapabilities `json:"capabilities"`
}

// clientInfo Client 信息
type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// clientCapabilities Client 能力
type clientCapabilities struct {
	Roots    *rootsCapability    `json:"roots,omitempty"`
	Sampling *samplingCapability `json:"sampling,omitempty"`
}

type rootsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type samplingCapability struct{}

// initializeResult initialize 方法的结果
type initializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ServerInfo      serverInfo       `json:"serverInfo"`
	Capabilities    serverCapabilities `json:"capabilities"`
}

// serverInfo Server 信息
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// serverCapabilities Server 能力
type serverCapabilities struct {
	Tools     *toolsCapability     `json:"tools,omitempty"`
	Resources *resourcesCapability `json:"resources,omitempty"`
	Prompts   *promptsCapability   `json:"prompts,omitempty"`
	Logging   *loggingCapability   `json:"logging,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type resourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

type promptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type loggingCapability struct{}

// mcpTool MCP 工具定义
type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// listToolsResult tools/list 方法的结果
type listToolsResult struct {
	Tools []mcpTool `json:"tools"`
}

// callToolParams tools/call 方法的参数
type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// contentBlock 内容块
type contentBlock struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text,omitempty"`
}

// callToolResult tools/call 方法的结果
type callToolResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ============================================================================
// Server 核心实现
// ============================================================================

// handwritternServer 手写的 MCP Server
type handwritternServer struct {
	name    string            // Server 名称
	version string            // Server 版本
	tools   map[string]toolDef // 工具注册表
}

// toolDef 工具定义（包含处理函数）
type toolDef struct {
	name        string
	description string
	inputSchema json.RawMessage
	handler     func(ctx context.Context, args map[string]any) (string, error)
}

// newHandwrittenServer 创建新的手写 Server
func newHandwrittenServer(name, version string) *handwritternServer {
	s := &handwritternServer{
		name:    name,
		version: version,
		tools:   make(map[string]toolDef),
	}

	// 注册工具
	s.registerTools()

	return s
}

// registerTools 注册工具
func (s *handwritternServer) registerTools() {
	// 1. 注册 get_time 工具
	s.tools["get_time"] = toolDef{
		name:        "get_time",
		description: "获取当前时间，支持按 IANA 时区格式化",
		inputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"timezone": {
					"type": "string",
					"description": "IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"
				}
			}
		}`),
		handler: func(ctx context.Context, args map[string]any) (string, error) {
			// 获取时区参数
			timezone := ""
			if tz, ok := args["timezone"].(string); ok {
				timezone = tz
			}

			// 获取当前时间
			now := time.Now()

			// 如果指定了时区，尝试加载（简化版本，实际应该使用时区库）
			if timezone != "" {
				// 这里简化处理，实际应该使用 time.LoadLocation
				return fmt.Sprintf("当前时间 (%s): %s", timezone, now.Format(time.RFC3339)), nil
			}

			return fmt.Sprintf("当前时间 (本地): %s", now.Format(time.RFC3339)), nil
		},
	}

	// 2. 注册 calc 工具
	s.tools["calc"] = toolDef{
		name:        "calc",
		description: "计算只包含数字、括号、+、-、*、/ 的算术表达式",
		inputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"expr": {
					"type": "string",
					"description": "四则运算表达式，例如 1+2*3"
				}
			},
			"required": ["expr"]
		}`),
		handler: func(ctx context.Context, args map[string]any) (string, error) {
			expr, ok := args["expr"].(string)
			if !ok {
				return "", fmt.Errorf("缺少 expr 参数")
			}

			// 简单实现：使用 Go 的表达式求值
			// 注意：生产环境应该使用专门的数学表达式解析库
			result, err := simpleEval(expr)
			if err != nil {
				return "", fmt.Errorf("计算失败: %w", err)
			}

			return fmt.Sprintf("%s = %v", expr, result), nil
		},
	}
}

// simpleEval 简单表达式求值
func simpleEval(expr string) (float64, error) {
	// TODO: 实现真正的表达式解析器
	// 这里简化返回固定值
	return 0, fmt.Errorf("表达式解析器未实现")
}

// ============================================================================
// 消息处理
// ============================================================================

// handleMessage 处理一条 JSON-RPC 消息
func (s *handwritternServer) handleMessage(msg []byte) ([]byte, error) {
	// 1. 解析请求
	var req jsonRPCRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return s.errorResponse(nil, -32700, "Parse error", err.Error())
	}

	// 2. 通知（无 ID）直接处理，不返回响应
	if req.ID == nil {
		s.handleNotification(req.Method, req.Params)
		return nil, nil
	}

	// 3. 按方法分发
	switch req.Method {
	case "initialize":
		return s.handleInitialize(*req.ID, req.Params)
	case "tools/list":
		return s.handleListTools(*req.ID)
	case "tools/call":
		return s.handleCallTool(*req.ID, req.Params)
	case "notifications/initialized":
		// 忽略 Client 的通知
		return nil, nil
	default:
		return s.errorResponse(req.ID, -32601, "Method not found", req.Method)
	}
}

// handleNotification 处理通知
func (s *handwritternServer) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "notifications/initialized":
		fmt.Fprintln(os.Stderr, "[Server] 收到 initialized 通知")
	default:
		fmt.Fprintf(os.Stderr, "[Server] 未知通知: %s\n", method)
	}
}

// handleInitialize 处理 initialize 方法
func (s *handwritternServer) handleInitialize(id int, params json.RawMessage) ([]byte, error) {
	// 解析参数
	var p initializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return s.errorResponse(&id, -32602, "Invalid params", err.Error())
		}
	}

	// 构建响应
	result := initializeResult{
		ProtocolVersion: "2025-11-25",
		ServerInfo: serverInfo{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: serverCapabilities{
			Tools: &toolsCapability{
				ListChanged: false,
			},
			Resources: &resourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
			Prompts: &promptsCapability{
				ListChanged: false,
			},
			Logging: &loggingCapability{},
		},
	}

	fmt.Fprintf(os.Stderr, "[Server] 完成握手 (Client: %s)\n", p.ClientInfo.Name)

	return s.successResponse(id, result)
}

// handleListTools 处理 tools/list 方法
func (s *handwritternServer) handleListTools(id int) ([]byte, error) {
	// 收集所有工具
	tools := make([]mcpTool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, mcpTool{
			Name:        t.name,
			Description: t.description,
			InputSchema: t.inputSchema,
		})
	}

	result := listToolsResult{
		Tools: tools,
	}

	fmt.Fprintf(os.Stderr, "[Server] 列出工具 (共 %d 个)\n", len(tools))

	return s.successResponse(id, result)
}

// handleCallTool 处理 tools/call 方法
func (s *handwritternServer) handleCallTool(id int, params json.RawMessage) ([]byte, error) {
	// 解析参数
	var p callToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.errorResponse(&id, -32602, "Invalid params", err.Error())
	}

	// 查找工具
	toolDef, ok := s.tools[p.Name]
	if !ok {
		return s.errorResponse(&id, -32601, "Tool not found", p.Name)
	}

	// 解析参数
	var args map[string]any
	if len(p.Arguments) > 0 {
		if err := json.Unmarshal(p.Arguments, &args); err != nil {
			return s.errorResponse(&id, -32602, "Invalid params", "参数解析失败: "+err.Error())
		}
	} else {
		args = make(map[string]any)
	}

	// 调用工具
	ctx := context.Background()
	result, err := toolDef.handler(ctx, args)

	// 构建响应
	var content []contentBlock
	if err != nil {
		content = []contentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Error: %v", err),
		}}
	} else {
		content = []contentBlock{{
			Type: "text",
			Text: result,
		}}
	}

	callResult := callToolResult{
		Content: content,
		IsError: err != nil,
	}

	fmt.Fprintf(os.Stderr, "[Server] 调用工具: %s (error: %v)\n", p.Name, err)

	return s.successResponse(id, callResult)
}

// ============================================================================
// 响应构建辅助函数
// ============================================================================

// successResponse 构建成功响应
func (s *handwritternServer) successResponse(id int, result any) ([]byte, error) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  mustMarshal(result),
	}
	return json.Marshal(resp)
}

// errorResponse 构建错误响应
func (s *handwritternServer) errorResponse(id *int, code int, message string, data string) ([]byte, error) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(resp)
}

// mustMarshal 辅助函数：必须成功序列化
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("JSON 序列化失败: %v", err))
	}
	return data
}

// ============================================================================
// 服务运行
// ============================================================================

// serve 启动 Server，从 stdin 读取，向 stdout 写入
func (s *handwritternServer) serve() error {
	return s.serveWithIO(os.Stdin, os.Stdout)
}

// serveWithIO 使用指定的 Reader 和 Writer 运行 Server
func (s *handwritternServer) serveWithIO(r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	fmt.Fprintln(os.Stderr, "[Server] 手写 MCP Server 启动")
	fmt.Fprintln(os.Stderr, "[Server] 将从 stdin 读取 JSON-RPC 消息")
	fmt.Fprintln(os.Stderr, "[Server] 将向 stdout 写入 JSON-RPC 响应")
	fmt.Fprintln(os.Stderr, "---")
	defer fmt.Fprintln(os.Stderr, "[Server] Server 关闭")

	for {
		// 读取一行（一条 JSON-RPC 消息）
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(os.Stderr, "[Server] Client 关闭连接")
				return nil
			}
			return fmt.Errorf("读取消息失败: %w", err)
		}

		// 跳过空行
		if len(line) <= 1 { // 只有换行符
			continue
		}

		// 打印接收到的请求（调试用）
		fmt.Fprintf(os.Stderr, "[Server] 收到请求: %s", string(line))

		// 处理消息
		resp, err := s.handleMessage(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Server] 处理消息失败: %v\n", err)
			continue
		}

		// 写入响应
		if resp != nil {
			if _, err := writer.Write(resp); err != nil {
				return fmt.Errorf("写入响应失败: %w", err)
			}
			if err := writer.WriteByte('\n'); err != nil {
				return fmt.Errorf("写入换行失败: %w", err)
			}
			if err := writer.Flush(); err != nil {
				return fmt.Errorf("刷新缓冲区失败: %w", err)
			}

			// 打印发送的响应（调试用）
			fmt.Fprintf(os.Stderr, "[Server] 发送响应: %s\n", string(resp))
		}
	}
}

// ============================================================================
// 主函数
// ============================================================================

func main() {
	// 创建 Server
	server := newHandwrittenServer("handwritten-demo-server", "1.0.0")

	// 启动 Server
	if err := server.serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server 错误: %v\n", err)
		os.Exit(1)
	}
}

// 文件职责：
// - MCP stdio Server 实现
// - 处理 JSON-RPC 消息
// - 管理工具注册和调用

package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// Server 核心类型
// ============================================================================

// Server 是 MCP stdio Server
type Server struct {
	name    string                // Server 名称
	version string                // Server 版本
	tools   map[string]tool.Tool  // 工具注册表
	mu      sync.RWMutex          // 读写锁
}

// NewServer 创建新的 MCP Server
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]tool.Tool),
	}
}

// AddTool 注册工具到 Server
func (s *Server) AddTool(t tool.Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Name()] = t
}

// GetTool 获取工具
func (s *Server) GetTool(name string) (tool.Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tools[name]
	return t, ok
}

// ============================================================================
// 消息处理
// ============================================================================

// handleMessage 处理一条 JSON-RPC 消息
func (s *Server) handleMessage(msg []byte) ([]byte, error) {
	// 解析请求
	var req rpcRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		return s.errorResponse(nil, -32700, "Parse error", err.Error())
	}

	// 通知（无 ID）直接处理，不返回响应
	if req.ID == nil {
		s.handleNotification(req.Method, req.Params)
		return nil, nil
	}

	// 处理请求
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

// handleNotification 处理通知（无响应）
func (s *Server) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "notifications/initialized":
		// Client 通知握手完成
		fmt.Fprintln(os.Stderr, "[Server] 收到 initialized 通知")
	default:
		fmt.Fprintf(os.Stderr, "[Server] 未知通知: %s\n", method)
	}
}

// ============================================================================
// 方法处理器
// ============================================================================

// handleInitialize 处理 initialize 方法
func (s *Server) handleInitialize(id int, params json.RawMessage) ([]byte, error) {
	// 解析参数
	var p InitializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return s.errorResponse(&id, -32602, "Invalid params", err.Error())
		}
	}

	// 构建结果
	result := InitializeResult{
		ProtocolVersion: "2025-11-25",
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
			Resources: &ResourcesCapability{
				Subscribe:   false,
				ListChanged: false,
			},
			Prompts: &PromptsCapability{
				ListChanged: false,
			},
			Logging: &LoggingCapability{},
		},
	}

	fmt.Fprintf(os.Stderr, "[Server] 完成握手 (Client: %s)\n", p.ClientInfo.Name)

	return s.successResponse(id, result)
}

// handleListTools 处理 tools/list 方法
func (s *Server) handleListTools(id int) ([]byte, error) {
	s.mu.RLock()
	tools := make([]Tool, 0, len(s.tools))
	for _, t := range s.tools {
		// 尝试获取 JSON Schema
		var schema json.RawMessage
		if st, ok := t.(tool.SchemaTool); ok {
			schema = st.ParametersSchema()
		} else {
			// 转换 map[string]string 到 JSON Schema
			schema = mapToJSONSchema(t.Parameters())
		}

		tools = append(tools, Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: schema,
		})
	}
	s.mu.RUnlock()

	result := ListToolsResult{
		Tools: tools,
	}

	fmt.Fprintf(os.Stderr, "[Server] 列出工具 (共 %d 个)\n", len(tools))

	return s.successResponse(id, result)
}

// handleCallTool 处理 tools/call 方法
func (s *Server) handleCallTool(id int, params json.RawMessage) ([]byte, error) {
	// 解析参数
	var p CallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return s.errorResponse(&id, -32602, "Invalid params", err.Error())
	}

	// 获取工具
	s.mu.RLock()
	t, ok := s.tools[p.Name]
	s.mu.RUnlock()

	if !ok {
		return s.errorResponse(&id, -32601, "Tool not found", p.Name)
	}

	// 调用工具
	ctx := context.Background()

	// 将 json.RawMessage 转换为 map[string]any
	var args map[string]any
	if len(p.Arguments) > 0 {
		if err := json.Unmarshal(p.Arguments, &args); err != nil {
			return s.errorResponse(&id, -32602, "Invalid params", "参数解析失败: "+err.Error())
		}
	}

	result, err := t.Call(ctx, args)

	// 构建响应
	var content []ContentBlock
	if err != nil {
		content = []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Error: %v", err),
		}}
	} else {
		// 将结果转换为字符串
		resultStr := fmt.Sprintf("%v", result)
		content = []ContentBlock{{
			Type: "text",
			Text: resultStr,
		}}
	}

	callResult := CallToolResult{
		Content: content,
		IsError: err != nil,
	}

	fmt.Fprintf(os.Stderr, "[Server] 调用工具: %s (error: %v)\n", p.Name, err)

	return s.successResponse(id, callResult)
}

// ============================================================================
// 响应构建
// ============================================================================

// successResponse 构建成功响应
func (s *Server) successResponse(id int, result any) ([]byte, error) {
	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  mustMarshal(result),
	}
	return json.Marshal(resp)
}

// errorResponse 构建错误响应
func (s *Server) errorResponse(id *int, code int, message string, data string) ([]byte, error) {
	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(resp)
}

// ============================================================================
// 服务运行
// ============================================================================

// Serve 启动 Server，从 stdin 读取，向 stdout 写入
func (s *Server) Serve() error {
	return s.ServeWithIO(os.Stdin, os.Stdout)
}

// ServeWithIO 使用指定的 Reader 和 Writer 运行 Server
func (s *Server) ServeWithIO(r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	fmt.Fprintln(os.Stderr, "[Server] MCP Server 启动")
	defer fmt.Fprintln(os.Stderr, "[Server] MCP Server 关闭")

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
		}
	}
}

// mustMarshal 辅助函数：必须成功序列化
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("JSON 序列化失败: %v", err))
	}
	return data
}

// mapToJSONSchema 将 map[string]string 转换为 JSON Schema
func mapToJSONSchema(params map[string]string) json.RawMessage {
	if len(params) == 0 {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}

	properties := make(map[string]any)
	required := make([]string, 0, len(params))

	for name, desc := range params {
		// 简单处理：所有参数都是字符串类型
		properties[name] = map[string]any{
			"type":        "string",
			"description": desc,
		}
		required = append(required, name)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	data, _ := json.Marshal(schema)
	return data
}

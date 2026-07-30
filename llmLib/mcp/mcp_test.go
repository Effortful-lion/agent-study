// 文件职责：
// - MCP Server 测试
// - MCP Client 测试
// - 工具桥接测试

package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/Effortful-lion/agent-study/llmLib/mcp"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 测试用 Server 工厂
// ============================================================================

// newTestServer 创建测试用 Server
func newTestServer() *mcp.Server {
	server := mcp.NewServer("test-server", "1.0.0")

	// 注册测试工具
	server.AddTool(tool.NewJSONSchemaTool(
		"echo",
		"回显工具",
		json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`),
		func(ctx context.Context, args map[string]any) (any, error) {
			return fmt.Sprintf("Echo: %v", args["message"]), nil
		},
	))

	server.AddTool(tool.NewJSONSchemaTool(
		"add",
		"加法工具",
		json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`),
		func(ctx context.Context, args map[string]any) (any, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			return a + b, nil
		},
	))

	return server
}

// ============================================================================
// Server 测试
// ============================================================================

func TestServer_Initialize(t *testing.T) {
	server := newTestServer()

	// 创建管道模拟 stdio
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	go func() {
		server.ServeWithIO(r, w)
	}()

	// TODO: 实现完整的集成测试
}

// ============================================================================
// Client 测试
// ============================================================================

func TestClient_BasicFlow(t *testing.T) {
	// 启动 Server 子进程
	// 这里使用 net.Pipe 模拟父子进程通信
	// 实际测试应该使用真正的子进程

	// TODO: 实现完整的集成测试
}

// ============================================================================
// Bridge 测试
// ============================================================================

func TestBridgeTool(t *testing.T) {
	// TODO: 实现完整的桥接测试
}

// ============================================================================
// 集成测试
// ============================================================================

// createTestClientServer 创建 Client-Server 对
func createTestClientServer() (*mcp.Client, *mcp.Server, func()) {
	// 使用网络连接模拟 stdio
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := listener.Addr().String()

	// 启动 Server
	server := newTestServer()
	go func() {
		conn, _ := listener.Accept()
		server.ServeWithIO(conn, conn)
	}()

	// 启动 Client
	// 这里简化，实际应该通过 exec.Command 启动
	// client, _ := mcp.NewClient("go", []string{"run", "server/main.go"})

	cleanup := func() {
		listener.Close()
	}

	return nil, server, cleanup
}

// ============================================================================
// 消息格式测试
// ============================================================================

func TestJSONRPCMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		req := mcp.rpcRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}

		data, _ := json.Marshal(req)
		expected := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

		if string(data) != expected {
			t.Errorf("请求序列化不匹配\n期望: %s\n实际: %s", expected, string(data))
		}
	})

	t.Run("Response", func(t *testing.T) {
		id := 1
		resp := mcp.rpcResponse{
			JSONRPC: "2.0",
			ID:      &id,
			Result:  json.RawMessage(`{"tools":[]}`),
		}

		data, _ := json.Marshal(resp)
		expected := `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`

		if string(data) != expected {
			t.Errorf("响应序列化不匹配\n期望: %s\n实际: %s", expected, string(data))
		}
	})

	t.Run("Notification", func(t *testing.T) {
		req := map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/initialized",
		}

		data, _ := json.Marshal(req)
		expected := `{"jsonrpc":"2.0","method":"notifications/initialized"}`

		if string(data) != expected {
			t.Errorf("通知序列化不匹配\n期望: %s\n实际: %s", expected, string(data))
		}
	})
}

// ============================================================================
// 工具定义测试
// ============================================================================

func TestToolDefinition(t *testing.T) {
	server := newTestServer()

	// 检查工具
	tools := server.Tools
	if len(tools) != 2 {
		t.Fatalf("期望 2 个工具，实际 %d", len(tools))
	}

	// 检查 echo 工具
	echoTool, ok := tools["echo"]
	if !ok {
		t.Error("缺少 echo 工具")
	} else {
		if echoTool.Name() != "echo" {
			t.Errorf("工具名不匹配: 期望 'echo', 实际 '%s'", echoTool.Name())
		}
		if echoTool.Description() == "" {
			t.Error("工具描述为空")
		}
	}

	// 检查 add 工具
	addTool, ok := tools["add"]
	if !ok {
		t.Error("缺少 add 工具")
	} else {
		if addTool.Name() != "add" {
			t.Errorf("工具名不匹配: 期望 'add', 实际 '%s'", addTool.Name())
		}
	}
}

// ============================================================================
// 工具调用测试
// ============================================================================

func TestToolCallViaBridge(t *testing.T) {
	// 这个测试展示了如何通过桥接调用工具
	server := newTestServer()

	// 手动模拟桥接
	echoTool := &mcp.bridgedTool{
		// 这里简化，实际应该通过 Client 调用
	}

	_ = echoTool
	_ = server
}

// ============================================================================
// 辅助函数
// ============================================================================

// readLine 从 Reader 读取一行
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(line), "\n"), nil
}

// compareJSON 比较两个 JSON 是否相等
func compareJSON(a, b string) bool {
	var ja, jb any
	if err := json.Unmarshal([]byte(a), &ja); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &jb); err != nil {
		return false
	}

	// 简单比较
	aa, _ := json.Marshal(ja)
	bb, _ := json.Marshal(jb)
	return string(aa) == string(bb)
}

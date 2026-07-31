// 文件职责：
// - 使用 mcp.StdioClient 连接手写 Server
// - 使用 mcp.BridgeAll 桥接工具到本地 tool.Tool
// - 展示完整的 MCP 协议流程
// - 演示工具注册到 Agent 的完整过程

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Effortful-lion/agent-study/llmLib/mcp"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 主函数
// ============================================================================

func main() {
	ctx := context.Background()

	// ========== 1. 启动手写 MCP Server ==========
	fmt.Println("========================================")
	fmt.Println("  手写 MCP Server 客户端演示")
	fmt.Println("========================================")
	fmt.Println()

	serverCmd := "go"
	serverArgs := []string{"run", "server.go"}

	fmt.Printf("1. 启动手写 Server: %s %v\n", serverCmd, serverArgs)
	client, err := mcp.NewClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("   ✗ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("   ✓ Server 启动成功")
	fmt.Println()

	// ========== 2. 执行握手 ==========
	fmt.Println("2. 执行 MCP 握手")

	// 2.1 Initialize
	fmt.Print("   2.1 发送 initialize 请求... ")
	serverInfo, err := client.Initialize(ctx)
	if err != nil {
		fmt.Printf("✗\n   ✗ 握手失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓")
	fmt.Printf("       Server: %s v%s\n", serverInfo.ServerInfo.Name, serverInfo.ServerInfo.Version)
	fmt.Printf("       协议版本: %s\n", serverInfo.ProtocolVersion)
	fmt.Printf("       能力: %+v\n", serverInfo.Capabilities)

	// 2.2 Initialized 通知
	fmt.Print("   2.2 发送 initialized 通知... ")
	if err := client.Initialized(ctx); err != nil {
		fmt.Printf("✗\n   ✗ 发送通知失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓")
	fmt.Println()

	// ========== 3. 列出工具 ==========
	fmt.Println("3. 列出 Server 的工具")
	fmt.Print("   发送 tools/list 请求... ")
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		fmt.Printf("✗\n   ✗ 列出工具失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓")
	fmt.Printf("   可用工具 (共 %d 个):\n", len(mcpTools))
	for _, t := range mcpTools {
		fmt.Printf("     • %s: %s\n", t.Name, t.Description)
		fmt.Printf("       Schema: %s\n", string(t.InputSchema))
	}
	fmt.Println()

	// ========== 4. 桥接工具到本地 ==========
	fmt.Println("4. 使用 BridgeAll 桥接 MCP 工具")

	fmt.Println("   4.1 手动桥接演示（逐个桥接）：")
	manualTools := make([]tool.Tool, 0, len(mcpTools))
	for _, mt := range mcpTools {
		fmt.Printf("       桥接工具: %s\n", mt.Name)

		// 使用 BridgeTool 逐个桥接
		bridgedTool, err := mcp.BridgeTool(ctx, client, mt.Name)
		if err != nil {
			fmt.Printf("       ✗ 桥接失败: %v\n", err)
			continue
		}
		manualTools = append(manualTools, bridgedTool)
		fmt.Printf("       ✓ 成功桥接: %s\n", bridgedTool.Name())
	}
	fmt.Printf("   手动桥接完成: %d/%d 个工具\n", len(manualTools), len(mcpTools))

	fmt.Println()
	fmt.Println("   4.2 使用 BridgeAll 一键桥接：")

	// 创建一个新的 Client 连接（因为之前的已经被使用了）
	client2, err := mcp.NewClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("   ✗ 启动 Client 失败: %v\n", err)
		os.Exit(1)
	}
	defer client2.Close()

	// 使用 BridgeAll 一次性桥接所有工具
	bridgedTools, err := mcp.BridgeAll(ctx, client2)
	if err != nil {
		fmt.Printf("   ✗ 桥接失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   ✓ 成功桥接 %d 个工具\n", len(bridgedTools))
	for _, t := range bridgedTools {
		fmt.Printf("     • %s: %s\n", t.Name(), t.Description())
	}
	fmt.Println()

	// ========== 5. 注册到 Registry ==========
	fmt.Println("5. 将桥接的工具注册到 tool.Registry")

	registry := tool.NewRegistryToolSet()

	// 注册桥接的 MCP 工具
	for _, t := range bridgedTools {
		registry.Register(t)
		fmt.Printf("   ✓ 注册 MCP 工具: %s\n", t.Name())
	}

	// 添加一个本地工具演示混合使用
	registry.Register(&localEchoTool{})
	fmt.Println("   ✓ 注册本地工具: local_echo")
	fmt.Println()

	// ========== 6. 直接测试工具调用 ==========
	fmt.Println("6. 直接测试工具调用")

	for _, t := range bridgedTools {
		fmt.Printf("\n   调用 %s:\n", t.Name())

		// 根据工具名准备参数
		var args map[string]any
		switch t.Name() {
		case "get_time":
			args = map[string]any{"timezone": "Asia/Shanghai"}
		case "calc":
			args = map[string]any{"expr": "12 * (3 + 4)"}
		default:
			args = map[string]any{}
		}

		// 调用工具
		start := time.Now()
		result, err := registry.Call(ctx, t.Name(), args)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("     ✗ 失败: %v\n", err)
		} else {
			fmt.Printf("     ✓ 结果: %v\n", result)
			fmt.Printf("     耗时: %v\n", duration)
		}
	}

	// 测试本地工具
	fmt.Printf("\n   调用 local_echo:\n")
	result, err := registry.Call(ctx, "local_echo", map[string]any{"message": "Hello from Agent!"})
	if err != nil {
		fmt.Printf("     ✗ 失败: %v\n", err)
	} else {
		fmt.Printf("     ✓ 结果: %v\n", result)
	}
	fmt.Println()

	// ========== 7. 展示工具定义（供 LLM 使用）==========
	fmt.Println("7. 工具定义（供 LLM 使用）")

	fmt.Println("   方法 A: 使用 registry.ToolDefs()")
	toolDefs := registry.ToolDefs()
	fmt.Printf("   共 %d 个工具定义\n", len(toolDefs))
	for _, td := range toolDefs {
		fmt.Printf("\n   工具: %s\n", td.Function.Name)
		fmt.Printf("   描述: %s\n", td.Function.Description)
		fmt.Printf("   参数 Schema:\n")

		// 格式化 JSON
		var prettyJSON bytesPrettyJSON
		if err := json.Unmarshal(td.Function.Parameters, &prettyJSON); err == nil {
			fmt.Println(prettyJSON)
		} else {
			fmt.Printf("   %s\n", string(td.Function.Parameters))
		}
	}

	fmt.Println()

	// ========== 8. 创建 Agent 并运行 ==========
	fmt.Println("8. 创建 Agent 并运行完整流程")
	fmt.Println("   (模拟 Agent 使用 MCP 工具)")

	// 创建一个模拟的 LLM Provider
	// 实际使用时应该创建真实的 Provider
	fmt.Println("   ⚠  注意: 使用模拟 Provider 演示")
	fmt.Println("   实际使用时:")
	fmt.Println("     provider, _ := llmlib.NewProvider(llmlib.ProviderOpenAI)")
	fmt.Println("     agent := agent.New(provider, \"gpt-4o\", registry)")

	// 模拟一个简单的对话流程
	fmt.Println()
	fmt.Println("   --- 模拟 Agent 对话 ---")
	fmt.Println("   用户: 查询当前时间")

	// Agent 会自动：
	// 1. 从 registry.ToolDefs() 获取工具列表
	// 2. 发送给 LLM
	// 3. LLM 决定调用 get_time
	// 4. Agent 调用 registry.Call("get_time", ...)
	// 5. Registry 路由到 bridgedTool
	// 6. bridgedTool 通过 MCP Client 调用 Server 的 get_time 工具
	// 7. Server 执行 get_time 并返回结果
	// 8. 结果返回给 Agent 再给 LLM

	fmt.Println("   → Agent 从 registry 获取工具定义")
	fmt.Println("   → LLM 决定调用工具: get_time")
	fmt.Println("   → Agent 调用 registry.Call(\"get_time\", {\"timezone\": \"Asia/Shanghai\"})")

	result, err = registry.Call(ctx, "get_time", map[string]any{"timezone": "Asia/Shanghai"})
	if err != nil {
		fmt.Printf("   ✗ 工具调用失败: %v\n", err)
	} else {
		fmt.Printf("   → 工具返回: %v\n", result)
		fmt.Println("   → Agent 将结果返回给 LLM")
		fmt.Println("   → LLM 生成最终回复")
		fmt.Printf("   助手: 当前时间是 %v\n", result)
	}

	fmt.Println()
	fmt.Println("   用户: 计算 12 * (3 + 4)")
	fmt.Println("   → LLM 决定调用工具: calc")
	fmt.Println("   → Agent 调用 registry.Call(\"calc\", {\"expr\": \"12 * (3 + 4)\"})")

	result, err = registry.Call(ctx, "calc", map[string]any{"expr": "12 * (3 + 4)"})
	if err != nil {
		fmt.Printf("   ✗ 工具调用失败: %v\n", err)
	} else {
		fmt.Printf("   → 工具返回: %v\n", result)
		fmt.Println("   → Agent 将结果返回给 LLM")
		fmt.Println("   → LLM 生成最终回复")
		fmt.Printf("   助手: %v\n", result)
	}

	fmt.Println()
	fmt.Println("   --- 模拟结束 ---")
	fmt.Println()

	// ========== 9. 使用 Agent 框架（如果有真实 LLM）==========
	fmt.Println("9. 使用真实 Agent 框架的示例代码")
	fmt.Println()

	fmt.Println("   完整示例代码:")
	fmt.Println()
	fmt.Println(`   // 1. 启动 MCP Server 并桥接
   client, mcpTools, err := mcp.NewBridgedClient(
       "go", []string{"run", "server.go"},
   )
   if err != nil {
       log.Fatal(err)
   }
   defer client.Close()

   // 2. 创建 Registry 并注册工具
   registry := tool.NewRegistryToolSet()
   for _, t := range mcpTools {
       registry.Register(t)
   }

   // 3. 创建 LLM Provider
   provider, err := llmlib.NewProvider(llmlib.ProviderOpenAI)
   if err != nil {
       log.Fatal(err)
   }

   // 4. 创建 Agent
   a := agent.New(provider, "gpt-4o", registry,
       agent.WithSystemPrompt("你是一个有用的助手，可以使用工具来帮助用户。"),
   )

   // 5. 运行 Agent
   ctx := context.Background()
   events, err := a.Run(ctx, "查询当前时间并计算 12 * (3 + 4)")

   for event := range events {
       switch e := event.(type) {
       case agent.ToolCallEvent:
           fmt.Printf("调用工具: %s\n", e.Name)
       case agent.ToolResultEvent:
           fmt.Printf("工具结果: %v\n", e.Result)
       case agent.DoneEvent:
           fmt.Printf("完成: %s\n", e.Message)
       case agent.ErrorEvent:
           fmt.Printf("错误: %v\n", e.Error)
       }
   }
   `)

	fmt.Println()

	// ========== 10. 总结 ==========
	fmt.Println("10. 总结")
	fmt.Println()
	fmt.Println("    ✓ 手写 Server 处理 JSON-RPC 消息")
	fmt.Println("    ✓ 实现 initialize、tools/list、tools/call 三个方法")
	fmt.Println("    ✓ 暴露 get_time 和 calc 两个工具")
	fmt.Println("    ✓ 使用 mcp.Client 连接 Server")
	fmt.Println("    ✓ 使用 BridgeAll/BridgeTool 桥接工具")
	fmt.Println("    ✓ 注册到 tool.Registry")
	fmt.Println("    ✓ Agent 可以像调用本地工具一样调用 MCP 工具")
	fmt.Println()
	fmt.Println("    关键要点：")
	fmt.Println("    • MCP 协议基于 JSON-RPC 2.0")
	fmt.Println("    • Server 从 stdin 读取，向 stdout 写入")
	fmt.Println("    • Bridge 将 MCP 工具包装为本地 tool.Tool")
	fmt.Println("    • Agent 无需感知 MCP 协议细节")
	fmt.Println("    • 一次桥接，到处使用")
}

// ============================================================================
// 本地工具示例
// ============================================================================

// localEchoTool 本地回显工具
type localEchoTool struct{}

func (t *localEchoTool) Name() string {
	return "local_echo"
}

func (t *localEchoTool) Description() string {
	return "本地回显工具，演示混合使用本地和 MCP 工具"
}

func (t *localEchoTool) Parameters() map[string]string {
	return map[string]string{
		"message": "string, 要回显的消息",
	}
}

func (t *localEchoTool) Call(ctx context.Context, args map[string]any) (any, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少 message 参数")
	}
	return fmt.Sprintf("本地回显: %s", message), nil
}

// ============================================================================
// 辅助类型
// ============================================================================

// bytesPrettyJSON 格式化 JSON
type bytesPrettyJSON struct {
	json.RawMessage
}

func (b bytesPrettyJSON) String() string {
	var out bytesPrettyJSONOutput
	if err := json.Indent(&out, b.RawMessage, "", "  "); err != nil {
		return string(b.RawMessage)
	}
	return string(out)
}

type bytesPrettyJSONOutput []byte

func (b bytesPrettyJSONOutput) String() string {
	return string(b)
}

// ============================================================================
// 演示：直接使用 StdioClient
// ============================================================================

// demonstrateStdioClient 演示直接使用 StdioClient 发送原始消息
func demonstrateStdioClient(serverCmd string, serverArgs []string) {
	fmt.Println("演示：直接使用 StdioClient")

	ctx := context.Background()

	// 启动 Server
	client, err := mcp.NewClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("启动失败: %v\n", err)
		return
	}
	defer client.Close()

	// 1. 初始化
	fmt.Println("\n1. 初始化握手")
	serverInfo, err := client.Initialize(ctx)
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}
	fmt.Printf("Server: %s v%s\n", serverInfo.ServerInfo.Name, serverInfo.ServerInfo.Version)

	if err := client.Initialized(ctx); err != nil {
		fmt.Printf("发送 initialized 失败: %v\n", err)
		return
	}

	// 2. 列出工具
	fmt.Println("\n2. 列出工具")
	tools, err := client.ListTools(ctx)
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}
	fmt.Printf("可用工具: %d 个\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  - %s: %s\n", t.Name, t.Description)
	}

	// 3. 调用工具
	fmt.Println("\n3. 调用工具")

	// 调用 get_time
	fmt.Println("\n调用 get_time:")
	result, err := client.CallTool(ctx, "get_time", json.RawMessage(`{"timezone": "Asia/Shanghai"}`))
	if err != nil {
		fmt.Printf("失败: %v\n", err)
	} else {
		fmt.Printf("结果: %s\n", result.Text())
	}

	// 调用 calc
	fmt.Println("\n调用 calc:")
	result, err = client.CallTool(ctx, "calc", json.RawMessage(`{"expr": "2 + 3"}`))
	if err != nil {
		fmt.Printf("失败: %v\n", err)
	} else {
		fmt.Printf("结果: %s\n", result.Text())
	}
}

// ============================================================================
// 演示：原始 JSON-RPC 消息
// ============================================================================

// demonstrateRawMessages 演示原始 JSON-RPC 消息流
func demonstrateRawMessages(serverCmd string, serverArgs []string) {
	fmt.Println("\n演示：原始 JSON-RPC 消息流")

	// 启动 Server
	cmd := exec.Command(serverCmd, serverArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("创建 stdin 失败: %v\n", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("创建 stdout 失败: %v\n", err)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("启动 Server 失败: %v\n", err)
		return
	}
	defer cmd.Process.Kill()
	defer stdin.Close()

	reader := bufio.NewReader(stdout)
	nextID := 1

	// 辅助函数：发送请求并接收响应
	sendRequest := func(method string, params any) ([]byte, error) {
		req := map[string]any{
			"jsonrpc": "2.0",
			"id":      nextID,
			"method":  method,
		}
		if params != nil {
			req["params"] = params
		}

		reqJSON, _ := json.Marshal(req)
		fmt.Printf("\n→ 发送:\n  %s\n", string(reqJSON))

		if _, err := fmt.Fprintf(stdin, "%s\n", reqJSON); err != nil {
			return nil, fmt.Errorf("发送失败: %w", err)
		}

		// 读取响应
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("读取响应失败: %w", err)
		}

		fmt.Printf("← 收到:\n  %s\n", string(line))
		nextID++

		return line, nil
	}

	// 1. 初始化
	fmt.Println("\n--- 1. initialize ---")
	_, err = sendRequest("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo": map[string]any{
			"name":    "demo-client",
			"version": "1.0.0",
		},
	})
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}

	// 2. 发送 initialized 通知
	fmt.Println("\n--- 2. notifications/initialized ---")
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifJSON, _ := json.Marshal(notif)
	fmt.Printf("→ 发送:\n  %s\n", string(notifJSON))
	if _, err := fmt.Fprintf(stdin, "%s\n", notifJSON); err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}

	// 3. 列出工具
	fmt.Println("\n--- 3. tools/list ---")
	_, err = sendRequest("tools/list", nil)
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}

	// 4. 调用工具
	fmt.Println("\n--- 4. tools/call (get_time) ---")
	_, err = sendRequest("tools/call", map[string]any{
		"name":      "get_time",
		"arguments": map[string]any{"timezone": "UTC"},
	})
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}

	fmt.Println("\n--- 5. tools/call (calc) ---")
	_, err = sendRequest("tools/call", map[string]any{
		"name":      "calc",
		"arguments": map[string]any{"expr": "1 + 1"},
	})
	if err != nil {
		fmt.Printf("失败: %v\n", err)
		return
	}

	fmt.Println("\n✓ 原始消息流演示完成")
}

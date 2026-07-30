// 文件职责：
// - MCP stdio Client 演示
// - 启动 Server 并与之交互
// - 展示完整的 MCP 协议流程

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Effortful-lion/agent-study/llmLib/mcp"
)

func main() {
	// 1. 检查参数
	if len(os.Args) < 2 {
		fmt.Println("用法: go run client/main.go <server-command> [args...]")
		fmt.Println("示例: go run client/main.go go run server/main.go")
		os.Exit(1)
	}

	serverCmd := os.Args[1]
	serverArgs := os.Args[2:]

	// 2. 启动 MCP Server
	fmt.Printf("启动 Server: %s %v\n", serverCmd, serverArgs)
	client, err := mcp.NewClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("启动失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// 3. 执行握手
	fmt.Println("\n========== 1. 握手阶段 ==========")
	ctx := context.Background()
	serverInfo, err := client.Initialize(ctx)
	if err != nil {
		fmt.Printf("握手失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server 信息:\n")
	fmt.Printf("  名称: %s\n", serverInfo.ServerInfo.Name)
	fmt.Printf("  版本: %s\n", serverInfo.ServerInfo.Version)
	fmt.Printf("  协议版本: %s\n", serverInfo.ProtocolVersion)
	fmt.Printf("  能力: %+v\n", serverInfo.Capabilities)

	// 通知握手完成
	if err := client.Initialized(ctx); err != nil {
		fmt.Printf("发送 initialized 通知失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ 握手完成")

	// 4. 列出工具
	fmt.Println("\n========== 2. 列出工具 ==========")
	tools, err := client.ListTools(ctx)
	if err != nil {
		fmt.Printf("列出工具失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("可用工具 (共 %d 个):\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  - %s: %s\n", t.Name, t.Description)
		fmt.Printf("    Schema: %s\n", string(t.InputSchema))
	}

	// 5. 调用工具演示
	fmt.Println("\n========== 3. 调用工具 ==========")

	// 5.1 调用 get_time
	fmt.Println("\n5.1 调用 get_time:")
	result, err := client.CallTool(ctx, "get_time", nil)
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  结果: %s\n", result.Text())
	}

	// 5.2 调用 get_time（带参数）
	fmt.Println("\n5.2 调用 get_time (带 timezone):")
	params := map[string]any{"timezone": "Asia/Shanghai"}
	paramsJSON, _ := json.Marshal(params)
	result, err = client.CallTool(ctx, "get_time", paramsJSON)
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  结果: %s\n", result.Text())
	}

	// 5.3 调用 calc
	fmt.Println("\n5.3 调用 calc (12 * (3 + 4)):")
	params = map[string]any{"expr": "12*(3+4)"}
	paramsJSON, _ = json.Marshal(params)
	result, err = client.CallTool(ctx, "calc", paramsJSON)
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  结果: %s\n", result.Text())
	}

	// 6. 完整的消息流演示
	fmt.Println("\n========== 4. 原始消息流演示 ==========")
	demonstrateRawMessages(serverCmd, serverArgs)

	// 7. 桥接工具演示
	fmt.Println("\n========== 5. 桥接工具演示 ==========")
	demonstrateBridging(serverCmd, serverArgs)

	fmt.Println("\n========== 演示完成 ==========")
}

// demonstrateRawMessages 展示原始 JSON-RPC 消息
func demonstrateRawMessages(serverCmd string, serverArgs []string) {
	fmt.Println("启动一个新的 Server 实例来展示原始消息...")

	// 使用低级别的 Client
	cmd := exec.Command(serverCmd, serverArgs...)
	stdin, _ := cmd.StdinPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	stdout := bufio.NewReader(stdoutPipe)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		return
	}
	defer cmd.Process.Kill()
	defer stdin.Close()

	// 辅助函数：发送请求并打印响应
	sendAndPrint := func(method string, params any) {
		id := 1
		req := map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  method,
		}
		if params != nil {
			req["params"] = params
		}

		// 打印请求
		reqJSON, _ := json.Marshal(req)
		fmt.Printf("\n→ 请求:\n  %s\n", string(reqJSON))

		// 发送
		fmt.Fprintf(stdin, "%s\n", reqJSON)

		// 读取响应
		line, _ := stdout.ReadBytes('\n')
		fmt.Printf("← 响应:\n  %s\n", strings.TrimSpace(string(line)))
	}

	// 1. 初始化
	fmt.Println("\n--- initialize ---")
	sendAndPrint("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"clientInfo": map[string]any{
			"name":    "demo-client",
			"version": "1.0.0",
		},
	})

	// 2. 发送 initialized 通知
	fmt.Println("\n--- notifications/initialized ---")
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifJSON, _ := json.Marshal(notif)
	fmt.Printf("→ 通知:\n  %s\n", string(notifJSON))
	fmt.Fprintf(stdin, "%s\n", notifJSON)

	// 3. 列出工具
	fmt.Println("\n--- tools/list ---")
	sendAndPrint("tools/list", nil)

	// 4. 调用工具
	fmt.Println("\n--- tools/call ---")
	sendAndPrint("tools/call", map[string]any{
		"name":      "get_time",
		"arguments": map[string]any{},
	})
}

// demonstrateBridging 演示工具桥接
func demonstrateBridging(serverCmd string, serverArgs []string) {
	// 启动 Client 并桥接工具
	client, tools, err := mcp.NewBridgedClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("桥接失败: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("成功桥接 %d 个工具:\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  - %s\n", t.Name())
		fmt.Printf("    Description: %s\n", t.Description())

		// 检查是否支持 SchemaTool
		if st, ok := t.(interface{ ParametersSchema() json.RawMessage }); ok {
			schema := st.ParametersSchema()
			fmt.Printf("    Schema: %s\n", string(schema))
		}
	}

	fmt.Println("\n桥接后的工具可以直接注册到 Agent:")
	fmt.Println("  registry := tool.NewRegistryToolSet()")
	fmt.Println("  registry.Register(mcpTools...)")
	fmt.Println("  agent := agent.New(provider, model, registry)")
}

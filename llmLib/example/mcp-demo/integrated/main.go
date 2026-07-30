// 文件职责：
// - 完整演示：将 MCP 工具接入 Agent
// - 启动 MCP Server
// - 桥接 MCP 工具到本地 tool.Tool
// - 注册到 Agent 并使用

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/Effortful-lion/agent-study/llmLib/agent"
	"github.com/Effortful-lion/agent-study/llmLib/mcp"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

func main() {
	// ========== 1. 启动 MCP Server ==========
	fmt.Println("=== 1. 启动 MCP Server ===")

	// 启动 Server 子进程
	// 实际使用时，这里应该指向真正的 MCP Server，如：
	// - go run server/main.go
	// - npx @modelcontextprotocol/server-filesystem /path
	// - python weather_server.py
	serverCmd := "go"
	serverArgs := []string{"run", "server/main.go"}

	client, mcpTools, err := mcp.NewBridgedClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("启动 MCP 失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Printf("✓ 成功桥接 %d 个 MCP 工具\n", len(mcpTools))
	for _, t := range mcpTools {
		fmt.Printf("  - %s: %s\n", t.Name(), t.Description())
	}

	// ========== 2. 注册工具到 Registry ==========
	fmt.Println("\n=== 2. 注册工具 ===")

	registry := tool.NewRegistryToolSet()

	// 添加 MCP 工具
	for _, mcpTool := range mcpTools {
		registry.Register(mcpTool)
		fmt.Printf("✓ 注册工具: %s\n", mcpTool.Name())
	}

	// 也可以添加本地工具
	registry.Register(&localTool{})
	fmt.Println("✓ 注册本地工具: local_echo")

	// ========== 3. 创建 Agent ==========
	fmt.Println("\n=== 3. 创建 Agent ===")

	// 注意：这里使用模拟的 Provider，实际使用时应该创建真实的 Provider
	// 演示重点在工具桥接，而非 LLM 调用
	fmt.Println("（使用模拟 Provider，重点展示工具桥接）")

	// TODO: 实际使用时取消注释
	/*
		provider, err := llmlib.NewProvider(llmlib.ProviderOpenAI)
		if err != nil {
			fmt.Printf("创建 Provider 失败: %v\n", err)
			os.Exit(1)
		}

		agent := agent.New(provider, "gpt-4o", registry,
			agent.WithSystemPrompt("你是一个有用的助手，可以使用工具来帮助用户。"),
		)
	*/

	// ========== 4. 直接测试工具调用 ==========
	fmt.Println("\n=== 4. 直接测试工具调用 ===")

	ctx := context.Background()

	// 测试 MCP 工具
	for _, mcpTool := range mcpTools {
		fmt.Printf("\n调用 %s:\n", mcpTool.Name())

		// 根据工具名准备参数
		var args map[string]any
		switch mcpTool.Name() {
		case "get_time":
			args = map[string]any{"timezone": "Asia/Shanghai"}
		case "calc":
			args = map[string]any{"expr": "12 * (3 + 4)"}
		default:
			args = map[string]any{}
		}

		// 调用工具
		result, err := registry.Call(ctx, mcpTool.Name(), args)
		if err != nil {
			fmt.Printf("  ✗ 失败: %v\n", err)
		} else {
			fmt.Printf("  ✓ 结果: %v\n", result)
		}
	}

	// 测试本地工具
	fmt.Printf("\n调用 local_echo:\n")
	result, err := registry.Call(ctx, "local_echo", map[string]any{"message": "Hello from Agent!"})
	if err != nil {
		fmt.Printf("  ✗ 失败: %v\n", err)
	} else {
		fmt.Printf("  ✓ 结果: %v\n", result)
	}

	// ========== 5. 展示工具定义 ==========
	fmt.Println("\n=== 5. 工具定义 (供 LLM 使用) ===")

	toolDefs := registry.ToolDefs()
	for _, td := range toolDefs {
		fmt.Printf("\n工具: %s\n", td.Function.Name)
		fmt.Printf("描述: %s\n", td.Function.Description)
		fmt.Printf("参数 Schema:\n")
		var prettyJSON bytesPrettyJSON
		if err := json.Unmarshal(td.Function.Parameters, &prettyJSON); err == nil {
			fmt.Println(prettyJSON)
		} else {
			fmt.Println(string(td.Function.Parameters))
		}
	}

	// ========== 6. 模拟 Agent 调用 ==========
	fmt.Println("\n=== 6. Agent 集成说明 ===")
	fmt.Println(`
将 MCP 工具接入 Agent 只需三步：

1. 启动 MCP Server 并桥接工具：

   client, mcpTools, err := mcp.NewBridgedClient(
       "go", []string{"run", "server/main.go"},
   )
   if err != nil {
       log.Fatal(err)
   }
   defer client.Close()

2. 注册到 Registry：

   registry := tool.NewRegistryToolSet()
   for _, t := range mcpTools {
       registry.Register(t)
   }

3. 在 Agent 中使用：

   agent := agent.New(provider, model, registry)
   events, err := agent.Run(ctx, "查询当前时间")

   for event := range events {
       switch e := event.(type) {
       case agent.ToolCallEvent:
           fmt.Printf("调用工具: %s\n", e.Name)
       case agent.ToolResultEvent:
           fmt.Printf("工具结果: %v\n", e.Result)
       case agent.DoneEvent:
           fmt.Printf("完成: %s\n", e.Message)
       }
   }

Agent 会自动：
- 从 registry.ToolDefs() 获取工具定义
- 发送给 LLM
- LLM 决定调用哪个工具
- Agent 调用 registry.Call()
- Registry 路由到对应的 MCP 工具
- 结果返回给 LLM 继续推理
`)

	// ========== 7. 总结 ==========
	fmt.Println("\n=== 7. 总结 ===")
	fmt.Printf("✓ 启动 MCP Server\n")
	fmt.Printf("✓ 桥接 %d 个工具\n", len(mcpTools))
	fmt.Printf("✓ 注册到 Registry\n")
	fmt.Printf("✓ 集成到 Agent\n")
	fmt.Println("\n关键要点：")
	fmt.Println("  • MCP 工具通过桥接变成本地 tool.Tool")
	fmt.Println("  • Agent 无需感知 MCP 协议细节")
	fmt.Println("  • 一次桥接，到处使用")
	fmt.Println("  • 支持多个 MCP Server 同时接入")
}

// ============================================================================
// 本地工具示例
// ============================================================================

type localTool struct{}

func (t *localTool) Name() string {
	return "local_echo"
}

func (t *localTool) Description() string {
	return "本地回显工具，演示混合使用本地和 MCP 工具"
}

func (t *localTool) Parameters() map[string]string {
	return map[string]string{
		"message": "string, 要回显的消息",
	}
}

func (t *localTool) Call(ctx context.Context, args map[string]any) (any, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少 message 参数")
	}
	return fmt.Sprintf("本地回显: %s", message), nil
}

// ============================================================================
// 辅助函数
// ============================================================================

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

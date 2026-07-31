// 文件职责：
// - 演示 MCP 工具的安全净化包装
// - 展示 6.7 生态方向与安全中的净化功能
// - 使用 SecureBridgedTool 和 SecureRegistry
// - 集成审计日志、工具白名单、输出净化

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Effortful-lion/agent-study/llmLib/mcp"
	"github.com/Effortful-lion/agent-study/llmLib/security"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ============================================================================
// 主函数
// ============================================================================

func main() {
	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("  MCP 安全净化包装演示")
	fmt.Println("  基于 6.7 生态方向与安全设计")
	fmt.Println("========================================")
	fmt.Println()

	// ========== 1. 创建配置 ==========
	fmt.Println("1. 创建安全配置")

	config := &mcp.SecureBridgeConfig{
		Sanitizer:     security.NewSanitizer(),
		AuditLogger:   security.NewAuditLogger(),
		ToolWhitelist: security.NewToolWhitelist(),
	}

	fmt.Println("   ✓ 创建 Sanitizer（输出净化器）")
	fmt.Println("   ✓ 创建 AuditLogger（审计日志器）")
	fmt.Println("   ✓ 创建 ToolWhitelist（工具白名单）")
	fmt.Println()

	// ========== 2. 配置工具白名单 ==========
	fmt.Println("2. 配置工具白名单")

	// 只允许 get_time 工具
	config.ToolWhitelist.Allow("get_time")
	config.ToolWhitelist.Enable()

	fmt.Println("   ✓ 白名单配置: get_time")
	fmt.Println("   ✗ 禁止使用: calc（未在白名单中）")
	fmt.Println()

	// ========== 3. 启动 MCP Server ==========
	fmt.Println("3. 启动 MCP Server")

	serverCmd := "go"
	serverArgs := []string{"run", "server.go"}

	client, err := mcp.NewClient(serverCmd, serverArgs)
	if err != nil {
		fmt.Printf("   ✗ 启动失败: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Println("   ✓ Server 启动成功")
	fmt.Println()

	// ========== 4. 安全桥接工具 ==========
	fmt.Println("4. 使用 SecureBridgeAll 安全桥接工具")

	secureTools, auditEvents, err := mcp.SecureBridgeAll(ctx, client,
		mcp.WithToolWhitelist([]string{"get_time"}),
	)
	if err != nil {
		fmt.Printf("   ✗ 桥接失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("   ✓ 安全桥接完成: %d 个工具\n", len(secureTools))
	fmt.Println()

	// ========== 5. 查看审计日志 ==========
	fmt.Println("5. 审计日志（桥接过程）")

	auditLogger := security.NewAuditLogger()
	for _, event := range auditEvents {
		auditLogger.Log(event)
	}

	fmt.Printf("   共 %d 条审计记录\n", len(auditEvents))
	for i, event := range auditEvents {
		fmt.Printf("   [%d] %s: %v\n", i+1, event.ToolName, event.Duration)
	}
	fmt.Println()

	// ========== 6. 创建安全 Registry ==========
	fmt.Println("6. 创建安全增强的 Registry")

	registry := tool.NewRegistryToolSet()

	// 使用安全 Registry
	secureRegistry := mcp.NewSecureRegistry(registry, config)

	// 注册工具（自动包装安全增强）
	for _, t := range secureTools {
		secureRegistry.Register(t)
		fmt.Printf("   ✓ 注册安全工具: %s\n", t.Name())
	}
	fmt.Println()

	// ========== 7. 测试工具调用（带安全保护）==========
	fmt.Println("7. 测试工具调用（带安全保护）")
	fmt.Println()

	// 7.1 调用 get_time（在白名单中）
	fmt.Println("   7.1 调用 get_time（在白名单中）:")
	fmt.Println("   ---")

	result, err := secureRegistry.Call(ctx, "get_time", map[string]any{
		"timezone": "Asia/Shanghai",
	})
	if err != nil {
		fmt.Printf("       ✗ 失败: %v\n", err)
	} else {
		fmt.Printf("       ✓ 结果: %v\n", result)
		fmt.Println("       （输出已自动净化）")
	}
	fmt.Println()

	// 7.2 尝试调用 calc（不在白名单中）
	fmt.Println("   7.2 尝试调用 calc（不在白名单中）:")
	fmt.Println("   ---")

	// 检查白名单
	if config.ToolWhitelist.IsAllowed("calc") {
		fmt.Println("       calc 在白名单中")
	} else {
		fmt.Println("       ✗ calc 不在白名单中，调用被阻止")
	}

	result, err = secureRegistry.Call(ctx, "calc", map[string]any{
		"expr": "12 * (3 + 4)",
	})
	if err != nil {
		fmt.Printf("       ✗ 失败: %v\n", err)
	} else {
		fmt.Printf("       ✓ 结果: %v\n", result)
	}
	fmt.Println()

	// 7.3 测试输出净化
	fmt.Println("   7.3 测试输出净化:")
	fmt.Println("   ---")

	// 模拟一个包含敏感信息的输出
	sensitiveOutput := `用户邮箱: user@example.com
电话: 138-1234-5678
地址: 北京市朝阳区某某街道123号
API密钥: sk-1234567890abcdef`

	fmt.Println("   原始输出:")
	fmt.Printf("   %s\n", sensitiveOutput)
	fmt.Println()

	sanitizer := security.NewSanitizer()
	sanitized := sanitizer.SanitizeToolOutput(sensitiveOutput)

	fmt.Println("   净化后输出:")
	fmt.Printf("   %s\n", sanitized)
	fmt.Println()

	// ========== 8. 查看审计日志 ==========
	fmt.Println("8. 查看审计日志")

	events := auditLogger.GetEvents()
	fmt.Printf("   共 %d 条审计记录\n", len(events))
	for i, event := range events {
		fmt.Printf("\n   [%d] 工具调用\n", i+1)
		fmt.Printf("       工具名: %s\n", event.ToolName)
		fmt.Printf("       参数: %v\n", event.ToolArgs)
		fmt.Printf("       结果: %s\n", event.Result)
		if event.Error != "" {
			fmt.Printf("       错误: %s\n", event.Error)
		}
		fmt.Printf("       耗时: %v\n", event.Duration)
	}
	fmt.Println()

	// ========== 9. 安全特性总结 ==========
	fmt.Println("9. 安全特性总结")
	fmt.Println()

	fmt.Println("   ✓ 工具白名单过滤")
	fmt.Println("     - 只允许白名单中的工具被调用")
	fmt.Println("     - 防止恶意工具被执行")
	fmt.Println()

	fmt.Println("   ✓ 输出自动净化")
	fmt.Println("     - 自动标记工具输出边界")
	fmt.Println("     - 过滤敏感信息（邮箱、电话、地址、密钥等）")
	fmt.Println("     - 限制输出长度")
	fmt.Println()

	fmt.Println("   ✓ 全操作审计日志")
	fmt.Println("     - 记录所有工具调用")
	fmt.Println("     - 包含参数、结果、错误、耗时")
	fmt.Println("     - 用于安全审计和问题排查")
	fmt.Println()

	fmt.Println("   ✓ 权限检查")
	fmt.Println("     - 调用前检查工具权限")
	fmt.Println("     - 调用后验证输出安全性")
	fmt.Println()

	// ========== 10. 在 Agent 中使用安全包装 ==========
	fmt.Println("10. 在 Agent 中使用安全包装")
	fmt.Println()

	fmt.Println("   示例代码:")
	fmt.Println()
	fmt.Println(`   // 1. 创建配置
   config := &mcp.SecureBridgeConfig{
       Sanitizer:     security.NewSanitizer(),
       AuditLogger:   security.NewAuditLogger(),
       ToolWhitelist: security.NewToolWhitelist(),
   }

   // 2. 配置白名单
   config.ToolWhitelist.Allow("get_time", "calc")
   config.ToolWhitelist.Enable()

   // 3. 安全桥接
   client, secureTools, err := mcp.NewSecureBridgedClient(
       "go", []string{"run", "server.go"},
       mcp.WithToolWhitelist([]string{"get_time", "calc"}),
   )
   if err != nil {
       log.Fatal(err)
   }
   defer client.Close()

   // 4. 创建 Registry
   registry := tool.NewRegistryToolSet()

   // 5. 使用安全 Registry（自动包装所有工具）
   secureRegistry := mcp.NewSecureRegistry(registry, config)
   for _, t := range secureTools {
       secureRegistry.Register(t)
   }

   // 6. 创建 Agent
   provider, _ := llmlib.NewProvider(llmlib.ProviderOpenAI)
   a := agent.New(provider, "gpt-4o", secureRegistry)

   // 7. 运行 Agent（所有工具调用都受安全保护）
   events, _ := a.Run(ctx, "查询当前时间")
   `)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  演示完成")
	fmt.Println("========================================")
}

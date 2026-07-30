// 文件职责：
// - MCP stdio Server 演示
// - 暴露 get_time 和 calc 两个工具
// - 展示完整的 MCP 协议处理

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Effortful-lion/agent-study/llmLib/mcp"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

func main() {
	// 1. 创建 MCP Server
	server := mcp.NewServer("demo-server", "1.0.0")

	// 2. 注册工具
	registerTools(server)

	// 3. 启动 Server
	fmt.Fprintln(os.Stderr, "演示 MCP Server 启动")
	fmt.Fprintln(os.Stderr, "将从 stdin 读取 JSON-RPC 消息")
	fmt.Fprintln(os.Stderr, "将向 stdout 写入 JSON-RPC 响应")
	fmt.Fprintln(os.Stderr, "---")

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server 错误: %v\n", err)
		os.Exit(1)
	}
}

// registerTools 注册演示工具
func registerTools(server *mcp.Server) {
	// 注册 get_time 工具
	server.AddTool(tool.NewJSONSchemaTool(
		"get_time",
		"获取当前时间，支持按 IANA 时区格式化",
		[]byte(`{
			"type": "object",
			"properties": {
				"timezone": {
					"type": "string",
					"description": "IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"
				}
			}
		}`),
		func(ctx context.Context, args map[string]any) (any, error) {
			// 简单实现，实际应该使用时区库
			return "2025-07-29T14:30:00+08:00", nil
		},
	))

	// 注册 calc 工具
	server.AddTool(tool.NewJSONSchemaTool(
		"calc",
		"计算只包含数字、括号、+、-、*、/ 的算术表达式",
		[]byte(`{
			"type": "object",
			"properties": {
				"expr": {
					"type": "string",
					"description": "四则运算表达式，例如 1+2*3"
				}
			},
			"required": ["expr"]
		}`),
		func(ctx context.Context, args map[string]any) (any, error) {
			expr, ok := args["expr"].(string)
			if !ok {
				return nil, fmt.Errorf("缺少 expr 参数")
			}

			// 简单实现，实际应该使用表达式解析器
			result, err := simpleEval(expr)
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("%s = %v", expr, result), nil
		},
	))
}

// simpleEval 简单表达式求值（仅作演示）
func simpleEval(expr string) (float64, error) {
	// TODO: 实现真正的表达式解析
	return 0, fmt.Errorf("未实现表达式解析")
}

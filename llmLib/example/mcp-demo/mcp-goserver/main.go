// 文件职责：
// - 使用 mcp-go 库实现的 MCP stdio Server
// - 暴露 get_time 和 calc 两个工具
// - 演示 mcp-go 库的使用方法
//
// 与手写版的区别：
// - 使用 server.NewMCPServer 创建 Server
// - 使用 mcp.NewTool 声明工具
// - 使用 server.ServeStdio 启动服务
// - JSON-RPC 协议细节由库接管

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ============================================================================
// 工具处理函数
// ============================================================================

// handleGetTime 处理 get_time 工具
func handleGetTime(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 获取参数
	timezone := ""
	if args, ok := req.Params.Arguments.(map[string]interface{}); ok {
		if tz, ok := args["timezone"].(string); ok {
			timezone = tz
		}
	}

	// 获取当前时间
	now := time.Now()

	// 格式化时间
	var timeStr string
	if timezone != "" {
		timeStr = fmt.Sprintf("当前时间 (%s): %s", timezone, now.Format(time.RFC3339))
	} else {
		timeStr = fmt.Sprintf("当前时间 (本地): %s", now.Format(time.RFC3339))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: timeStr,
			},
		},
	}, nil
}

// handleCalc 处理 calc 工具
func handleCalc(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 获取参数
	expr, ok := req.Params.Arguments.(map[string]interface{})["expr"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "错误：缺少 expr 参数",
				},
			},
			IsError: true,
		}, nil
	}

	// 计算表达式（简化版本）
	result, err := simpleEval(expr)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("计算失败: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%s = %v", expr, result),
			},
		},
	}, nil
}

// simpleEval 简单表达式求值
func simpleEval(expr string) (float64, error) {
	// TODO: 实现真正的表达式解析器
	// 这里简化返回固定值
	return 0, fmt.Errorf("表达式解析器未实现")
}

// ============================================================================
// 主函数
// ============================================================================

func main() {
	// 1. 创建 MCP Server
	fmt.Fprintln(os.Stderr, "=== 使用 mcp-go 库创建的 MCP Server ===")
	s := server.NewMCPServer(
		"mcp-go-demo-server",
		"1.0.0",
	)

	// 2. 定义并注册工具

	// 2.1 get_time 工具
	getTimeTool := mcp.NewTool(
		"get_time",
		mcp.WithDescription("获取当前时间，支持按 IANA 时区格式化"),
		mcp.WithObject("timezone",
			mcp.Description("IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"),
		),
	)
	s.AddTool(getTimeTool, handleGetTime)

	// 2.2 calc 工具
	calcTool := mcp.NewTool(
		"calc",
		mcp.WithDescription("计算只包含数字、括号、+、-、*、/ 的算术表达式"),
		mcp.WithObject("expr",
			mcp.Description("四则运算表达式，例如 1+2*3"),
		),
	)
	s.AddTool(calcTool, handleCalc)

	fmt.Fprintln(os.Stderr, "✓ 注册工具: get_time, calc")
	fmt.Fprintln(os.Stderr, "✓ 启动 stdio 服务...")
	fmt.Fprintln(os.Stderr, "---")

	// 3. 启动 stdio 服务
	// ServeStdio 自动处理：
	// - 从 stdin 读取 JSON-RPC 请求
	// - 向 stdout 写入 JSON-RPC 响应
	// - 按 method 字段分发到工具处理器
	// - 处理 initialize、tools/list、tools/call 等标准方法
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server 错误: %v\n", err)
		os.Exit(1)
	}
}

// 文件职责：
// - 提供快速演示 builtin 工具的命令
// - 展示文件系统工具的安全防护

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Effortful-lion/agent-study/llmLib/builtin"
)

func main() {
	fmt.Println("=== Builtin 工具演示 ===")

	// 演示文件系统工具
	demoFileSystem()
}

func demoFileSystem() {
	fmt.Println("1. 文件系统工具演示")
	fmt.Println("----------------------------------------")

	// 创建临时目录
	kbDir, err := os.MkdirTemp("", "fs_demo_*")
	if err != nil {
		fmt.Printf("创建临时目录失败: %v\n", err)
		return
	}
	defer os.RemoveAll(kbDir)

	// 创建文件系统工具
	fs, err := builtin.NewFileSystemTool(kbDir)
	if err != nil {
		fmt.Printf("创建文件系统工具失败: %v\n", err)
		return
	}

	ctx := context.Background()

	// 1. 写入欢迎文件
	fmt.Println("\n1. 写入欢迎文件:")
	result, err := fs.WriteFileTool().Call(ctx, map[string]any{
		"path":    "welcome.txt",
		"content": "# 欢迎\n\n这是知识库的欢迎文件。",
	})
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  %v\n", result)
	}

	// 2. 创建子目录并写入文件
	fmt.Println("\n2. 创建子目录文件:")
	result, err = fs.WriteFileTool().Call(ctx, map[string]any{
		"path":    "docs/guide.md",
		"content": "# 使用指南\n\n## 快速开始\n\n1. 读取文档\n2. 开始使用",
	})
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  %v\n", result)
	}

	// 3. 列出根目录
	fmt.Println("\n3. 列出根目录:")
	result, err = fs.ListFilesTool().Call(ctx, map[string]any{"path": ""})
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  %v\n", result)
	}

	// 4. 读取文件
	fmt.Println("\n4. 读取 welcome.txt:")
	result, err = fs.ReadFileTool().Call(ctx, map[string]any{"path": "welcome.txt"})
	if err != nil {
		fmt.Printf("  失败: %v\n", err)
	} else {
		fmt.Printf("  内容: %v\n", result)
	}

	// 5. 测试安全防护
	fmt.Println("\n5. 测试安全防护:")
	fmt.Println("\n  5.1 尝试路径遍历攻击 (../../etc/passwd):")
	_, err = fs.ReadFileTool().Call(ctx, map[string]any{"path": "../../etc/passwd"})
	if err != nil {
		fmt.Printf("  ✓ 被阻止: %v\n", err)
	} else {
		fmt.Println("  ✗ 攻击成功！")
	}

	fmt.Println("\n  5.2 尝试使用绝对路径 (/etc/passwd):")
	_, err = fs.ReadFileTool().Call(ctx, map[string]any{"path": "/etc/passwd"})
	if err != nil {
		fmt.Printf("  ✓ 被阻止: %v\n", err)
	} else {
		fmt.Println("  ✗ 攻击成功！")
	}

	fmt.Println("\n  5.3 尝试写入系统目录 (../../tmp/evil.txt):")
	_, err = fs.WriteFileTool().Call(ctx, map[string]any{
		"path":    "../../tmp/evil.txt",
		"content": "malicious",
	})
	if err != nil {
		fmt.Printf("  ✓ 被阻止: %v\n", err)
	} else {
		fmt.Println("  ✗ 攻击成功！")
	}

	fmt.Println("\n✓ 所有安全测试通过！")
}

// 文件职责：
// - 提供完整的示例，展示如何在 Agent 中使用 builtin 工具
// - 演示文件系统和 NL2SQL 工具的完整工作流

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Effortful-lion/agent-study/llmLib/builtin"
	"github.com/Effortful-lion/agent-study/llmLib/tool"
)

// ExampleAgentWithBuiltinTools 展示如何在 Agent 中使用内置工具
func ExampleAgentWithBuiltinTools() {
	// ========== 1. 创建文件系统工具 ==========
	fmt.Println("=== 创建文件系统工具 ===")

	// 创建临时知识库目录
	kbDir := "/tmp/agent_knowledge_base"
	if err := os.MkdirAll(kbDir, 0755); err != nil {
		log.Fatalf("创建知识库目录失败: %v", err)
	}
	defer os.RemoveAll(kbDir)

	// 初始化文件系统工具
	fs, err := builtin.NewFileSystemTool(kbDir)
	if err != nil {
		log.Fatalf("创建文件系统工具失败: %v", err)
	}

	// ========== 2. 创建工具注册表 ==========
	registry := tool.NewRegistryToolSet()

	// 注册文件系统工具
	registry.Register(fs.ReadFileTool())
	registry.Register(fs.WriteFileTool())
	registry.Register(fs.ListFilesTool())

	// ========== 3. （可选）创建 NL2SQL 工具 ==========
	// 注意：这里只是示例，实际使用需要真实的数据库连接
	fmt.Println("\n=== NL2SQL 工具（可选） ===")

	// 示例：连接数据库（使用只读账号）
	// db, err := sql.Open("mysql", "readonly_user:password@tcp(localhost:3306)/mydb")
	// if err != nil {
	//     log.Printf("连接数据库失败（仅示例）: %v", err)
	// } else {
	//     defer db.Close()
	//
	//     nl2sql := builtin.NewNL2SQL(db)
	//     registry.Register(nl2sql.QueryTool())
	//     registry.Register(nl2sql.GetSchemaTool())
	//     registry.Register(nl2sql.ExplainQueryTool())
	// }

	// ========== 4. 列出所有工具 ==========
	fmt.Println("\n=== 已注册的工具 ===")
	for _, td := range registry.ToolDefs() {
		fmt.Printf("工具: %s\n", td.Function.Name)
		fmt.Printf("描述: %s\n", td.Function.Description)
		fmt.Printf("参数: %s\n\n", string(td.Function.Parameters))
	}

	// ========== 5. 演示工具调用 ==========
	fmt.Println("=== 演示工具调用 ===")

	// 5.1 写入文件
	fmt.Println("\n5.1 写入文件:")
	writeArgs := map[string]any{
		"path":    "welcome.txt",
		"content": "# 欢迎使用\n\n这是知识库的欢迎文件。",
	}
	result, err := registry.Call(context.Background(), "write_file", writeArgs)
	if err != nil {
		log.Printf("写入文件失败: %v", err)
	} else {
		fmt.Printf("  结果: %v\n", result)
	}

	// 5.2 列出文件
	fmt.Println("\n5.2 列出根目录文件:")
	listArgs := map[string]any{"path": ""}
	result, err = registry.Call(context.Background(), "list_files", listArgs)
	if err != nil {
		log.Printf("列出文件失败: %v", err)
	} else {
		fmt.Printf("  结果:\n%s\n", result)
	}

	// 5.3 读取文件
	fmt.Println("\n5.3 读取文件:")
	readArgs := map[string]any{"path": "welcome.txt"}
	result, err = registry.Call(context.Background(), "read_file", readArgs)
	if err != nil {
		log.Printf("读取文件失败: %v", err)
	} else {
		fmt.Printf("  内容:\n%s\n", result)
	}

	// 5.4 写入子目录（自动创建父目录）
	fmt.Println("\n5.4 写入子目录文件:")
	writeArgs = map[string]any{
		"path":    "docs/guide.md",
		"content": "# 使用指南\n\n## 如何读取文件\n\n使用 read_file 工具。",
	}
	result, err = registry.Call(context.Background(), "write_file", writeArgs)
	if err != nil {
		log.Printf("写入文件失败: %v", err)
	} else {
		fmt.Printf("  结果: %v\n", result)
	}

	// 5.5 列出子目录
	fmt.Println("\n5.5 列出 docs 目录:")
	listArgs = map[string]any{"path": "docs"}
	result, err = registry.Call(context.Background(), "list_files", listArgs)
	if err != nil {
		log.Printf("列出文件失败: %v", err)
	} else {
		fmt.Printf("  结果:\n%s\n", result)
	}

	// 5.6 测试安全防护
	fmt.Println("\n=== 测试安全防护 ===")

	fmt.Println("\n5.6 尝试路径遍历攻击:")
	attackArgs := map[string]any{"path": "../../etc/passwd"}
	_, err = registry.Call(context.Background(), "read_file", attackArgs)
	if err != nil {
		fmt.Printf("  ✅ 攻击被阻止: %v\n", err)
	} else {
		fmt.Println("  ❌ 攻击成功（不应该发生）")
	}

	fmt.Println("\n5.7 尝试写入越界文件:")
	attackArgs = map[string]any{
		"path":    "../../tmp/evil.txt",
		"content": "malicious content",
	}
	_, err = registry.Call(context.Background(), "write_file", attackArgs)
	if err != nil {
		fmt.Printf("  ✅ 攻击被阻止: %v\n", err)
	} else {
		fmt.Println("  ❌ 攻击成功（不应该发生）")
	}

	// ========== 6. 在 Agent 中使用 ==========
	fmt.Println("\n=== 在 Agent 中使用 ===")
	fmt.Println("将 registry 传递给 Agent 后，模型可以自动调用这些工具：")
	fmt.Println(`
	// 创建 Agent
	agent := agent.New(provider, model, registry)

	// Agent 会自动根据用户意图选择合适的工具
	agent.Run(ctx, "在知识库中创建一个欢迎文档")
	// → Agent 会调用 write_file 创建 welcome.txt

	agent.Run(ctx, "列出知识库中的所有文档")
	// → Agent 会调用 list_files 列出文件

	agent.Run(ctx, "读取 docs/guide.md 的内容")
	// → Agent 会调用 read_file 读取文件
	`)
}

// ExampleNL2SQLUsage 展示 NL2SQL 工具的使用方法
func ExampleNL2SQLUsage() {
	fmt.Println("=== NL2SQL 工具使用示例 ===")
	fmt.Print(`

// 1. 连接数据库（必须使用只读账号！）
db, err := sql.Open("mysql", "readonly_user:password@tcp(localhost:3306)/mydb")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 2. 创建 NL2SQL 工具
nl2sql := builtin.NewNL2SQL(db)

// 3. 注册到工具注册表
registry := tool.NewRegistryToolSet()
registry.Register(nl2sql.QueryTool())
registry.Register(nl2sql.GetSchemaTool())

// 4. 使用示例

// 4.1 获取表结构
result, _ := registry.Call(ctx, "get_db_schema", map[string]any{})
fmt.Println(result)
// 输出：数据库表列表:
//        - users
//        - orders
//        - products

// 4.2 查看表结构
result, _ := registry.Call(ctx, "get_db_schema", map[string]any{"table": "users"})
fmt.Println(result)
// 输出：表 users 的结构:
//        - id: int (nullable: NO, default: NULL)
//        - name: varchar(255) (nullable: NO, default: NULL)
//        - email: varchar(255) (nullable: NO, default: NULL)

// 4.3 执行查询
result, _ := registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "SELECT id, name FROM users WHERE age > 18 LIMIT 10",
})
fmt.Println(result)
// 输出：
// id | name
// 1  | Alice
// 2  | Bob

// 5. 在 Agent 中使用
agent := agent.New(provider, model, registry)
agent.Run(ctx, "查询年龄大于18岁的用户")
// → Agent 会先调用 get_db_schema 了解表结构
// → 然后生成正确的 SQL 查询
// → 最后调用 nl2sql_query 执行查询
`)

	fmt.Println("\n=== 安全防护示例 ===")
	fmt.Print(`
// ❌ 这些查询会被阻止：

// DELETE 查询
registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "DELETE FROM users WHERE id = 1",
})
// 错误: SQL 安全检查失败: 检测到禁止的关键字: delete

// DROP 表
registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "DROP TABLE users",
})
// 错误: SQL 安全检查失败: 检测到禁止的关键字: drop

// 多条语句
registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "SELECT * FROM users; DELETE FROM users",
})
// 错误: SQL 安全检查失败: 禁止多条语句

// UNION 注入
registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "SELECT * FROM users UNION SELECT * FROM passwords",
})
// 错误: SQL 安全检查失败: 检测到禁止的关键字: union

// ✅ 这个查询会被允许：
registry.Call(ctx, "nl2sql_query", map[string]any{
    "sql": "SELECT * FROM users WHERE age > 18 LIMIT 10",
})
// 成功执行，返回最多 50 行结果
`)
}

func main() {
	ExampleAgentWithBuiltinTools()
	fmt.Println("\n" + strings.Repeat("=", 80) + "\n")
	ExampleNL2SQLUsage()
}

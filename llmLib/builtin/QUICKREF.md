# FileSystemTool 快速参考

## 创建文件系统工具

```go
import "github.com/Effortful-lion/agent-study/llmLib/builtin"

// 创建文件系统工具，限制在指定目录
fs, err := builtin.NewFileSystemTool("/app/knowledge_base")
if err != nil {
    log.Fatal(err)
}
```

## 获取工具

```go
// 读取文件工具
readTool := fs.ReadFileTool()

// 写入文件工具
writeTool := fs.WriteFileTool()

// 列出文件工具
listTool := fs.ListFilesTool()
```

## 注册到工具注册表

```go
registry := tool.NewRegistryToolSet()
registry.Register(fs.ReadFileTool())
registry.Register(fs.WriteFileTool())
registry.Register(fs.ListFilesTool())
```

## 安全特性

✅ **路径围栏**：所有访问限制在根目录内
✅ **拒绝绝对路径**：防止绕过围栏
✅ **防止路径遍历**：拒绝 `../../etc/passwd`
✅ **大小限制**：文件读取限制 100KB
✅ **自动创建目录**：写入时自动创建父目录

## 工具参数

### read_file

```json
{
  "path": "相对于根目录的文件路径"
}
```

### write_file

```json
{
  "path": "相对于根目录的文件路径",
  "content": "要写入的内容",
  "append": false
}
```

### list_files

```json
{
  "path": "相对于根目录的目录路径（可选）"
}
```

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/Effortful-lion/agent-study/llmLib/builtin"
    "github.com/Effortful-lion/agent-study/llmLib/tool"
)

func main() {
    // 1. 创建文件系统工具
    fs, err := builtin.NewFileSystemTool("/tmp/knowledge_base")
    if err != nil {
        log.Fatal(err)
    }

    // 2. 创建注册表
    registry := tool.NewRegistryToolSet()
    registry.Register(fs.ReadFileTool())
    registry.Register(fs.WriteFileTool())
    registry.Register(fs.ListFilesTool())

    // 3. 使用工具
    ctx := context.Background()

    // 写入文件
    result, err := registry.Call(ctx, "write_file", map[string]any{
        "path":    "welcome.txt",
        "content": "Hello, World!",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)

    // 读取文件
    result, err = registry.Call(ctx, "read_file", map[string]any{
        "path": "welcome.txt",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)

    // 列出文件
    result, err = registry.Call(ctx, "list_files", map[string]any{
        "path": "",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result)
}
```

## 在 Agent 中使用

```go
// 创建 Agent 并集成文件系统工具
fs, _ := builtin.NewFileSystemTool("/app/knowledge_base")
registry := tool.NewRegistryToolSet()
registry.Register(fs.ReadFileTool())
registry.Register(fs.WriteFileTool())
registry.Register(fs.ListFilesTool())

// 创建 Agent
agent := agent.New(provider, model, registry)

// Agent 会自动使用这些工具
agent.Run(ctx, "读取知识库中的欢迎文档")
agent.Run(ctx, "在知识库中创建新文档")
agent.Run(ctx, "列出所有文档")
```

## 测试

```bash
# 运行所有测试
go test ./builtin/... -v

# 运行特定测试
go test ./builtin/... -v -run TestFileSystemTool_PathJail
go test ./builtin/... -v -run TestFileSystemTool_ReadFileTool
go test ./builtin/... -v -run TestFileSystemTool_WriteFileTool
```

## 常见错误

### 路径越界

```go
// ❌ 会被拒绝
fs.ReadFileTool().Call(ctx, map[string]any{"path": "../../etc/passwd"})
// 错误：路径越界，拒绝访问: ../../etc/passwd

// ✅ 正确
fs.ReadFileTool().Call(ctx, map[string]any{"path": "docs/file.txt"})
```

### 绝对路径

```go
// ❌ 会被拒绝
fs.ReadFileTool().Call(ctx, map[string]any{"path": "/etc/passwd"})
// 错误：拒绝绝对路径，只能使用相对路径: /etc/passwd

// ✅ 使用相对路径
fs.ReadFileTool().Call(ctx, map[string]any{"path": "config.yaml"})
```

### 文件过大

```go
// 文件大于 100KB 会被截断
content := fs.ReadFileTool().Call(ctx, map[string]any{"path": "large.log"})
// 返回：前 100KB 内容 + "... (文件已截断，仅显示前 102400 字节)"
```

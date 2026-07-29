# 内置工具使用指南

本文档说明如何使用 `builtin` 包中的安全约束工具。

## 概述

`builtin` 包提供了两个带安全约束的内置工具：

1. **文件系统工具** - 带路径围栏（path jail）防护
2. **NL2SQL 工具** - 带只读防护

## 1. 文件系统工具

### 安全特性

- **路径围栏（Path Jail）**：所有文件访问都限制在指定根目录内
- **防止路径遍历**：拒绝 `../../etc/passwd` 等越界请求
- **拒绝绝对路径**：只允许相对路径，防止绕过围栏
- **读取大小限制**：文件读取限制为 100KB，防止上下文溢出

### 使用方法

```go
import "github.com/Effortful-lion/agent-study/llmLib/builtin"

// 创建文件系统工具，限制在 /tmp/knowledge_base 目录
fs, err := builtin.NewFileSystemTool("/tmp/knowledge_base")
if err != nil {
    log.Fatalf("创建失败: %v", err)
}

// 获取工具
readFileTool := fs.ReadFileTool()
writeFileTool := fs.WriteFileTool()
listFilesTool := fs.ListFilesTool()

// 注册到工具注册表
registry := tool.NewRegistryToolSet()
registry.Register(readFileTool)
registry.Register(writeFileTool)
registry.Register(listFilesTool)
```

### 提供的工具

#### read_file

读取知识库目录下的文本文件内容。

**参数：**
```json
{
  "path": "相对于根目录的文件路径"
}
```

**示例：**
```go
args := map[string]any{"path": "docs/intro.md"}
result, err := registry.Call(ctx, "read_file", args)
```

#### write_file

向知识库目录写入文本文件内容。

**参数：**
```json
{
  "path": "相对于根目录的文件路径",
  "content": "要写入的内容",
  "append": false // 可选，默认覆盖
}
```

**示例：**
```go
args := map[string]any{
    "path": "notes/today.md",
    "content": "# 今日笔记\n\n- 完成文档编写",
}
result, err := registry.Call(ctx, "write_file", args)
```

#### list_files

列出指定目录下的文件和子目录。

**参数：**
```json
{
  "path": "相对于根目录的目录路径（可选，默认为根目录）"
}
```

**示例：**
```go
args := map[string]any{"path": "docs"}
result, err := registry.Call(ctx, "list_files", args)
```

### 安全示例

```go
// ✅ 合法访问
fs.ReadFileTool().Call(ctx, map[string]any{"path": "docs/readme.md"})   // 成功
fs.ReadFileTool().Call(ctx, map[string]any{"path": "subdir/file.txt"}) // 成功

// ❌ 越界访问（会被拒绝）
fs.ReadFileTool().Call(ctx, map[string]any{"path": "../../etc/passwd"}) // 路径遍历
fs.ReadFileTool().Call(ctx, map[string]any{"path": "/etc/passwd"})      // 绝对路径
```

## 2. NL2SQL 工具

### 安全特性

- **只读防护**：仅允许 SELECT 查询，禁止 INSERT/UPDATE/DELETE 等写操作
- **关键字过滤**：禁止 DROP、ALTER、TRUNCATE、GRANT 等危险关键字
- **防注入**：检测 UNION、注释等 SQL 注入模式
- **超时控制**：查询超时 5 秒，防止慢查询拖垮服务
- **行数限制**：最多返回 50 行结果

**重要**：代码层过滤只是辅助，真正不可逾越的防线是**使用只读数据库账号**连接。

### 使用方法

```go
import (
    "database/sql"
    "github.com/Effortful-lion/agent-study/llmLib/builtin"
)

// 连接数据库（必须使用只读账号！）
db, err := sql.Open("mysql", "readonly_user:password@tcp(localhost:3306)/mydb")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 创建 NL2SQL 工具
nl2sql := builtin.NewNL2SQL(db)

// 注册工具
registry := tool.NewRegistryToolSet()
registry.Register(nl2sql.QueryTool())
registry.Register(nl2sql.GetSchemaTool())
registry.Register(nl2sql.ExplainQueryTool())
```

### 提供的工具

#### nl2sql_query

执行只读 SQL 查询。

**参数：**
```json
{
  "sql": "SELECT * FROM users WHERE age > 18"
}
```

**示例：**
```go
args := map[string]any{
    "sql": "SELECT id, name FROM users LIMIT 10",
}
result, err := registry.Call(ctx, "nl2sql_query", args)
```

#### get_db_schema

获取数据库的表结构信息。

**参数：**
```json
{
  "table": "可选：指定表名，为空则返回所有表"
}
```

**示例：**
```go
// 获取所有表
args := map[string]any{}
result, err := registry.Call(ctx, "get_db_schema", args)

// 获取指定表结构
args := map[string]any{"table": "users"}
result, err := registry.Call(ctx, "get_db_schema", args)
```

#### explain_query

分析查询语句的执行计划。

**参数：**
```json
{
  "sql": "要分析的 SELECT 查询语句"
}
```

**示例：**
```go
args := map[string]any{
    "sql": "SELECT * FROM users WHERE age > 18",
}
result, err := registry.Call(ctx, "explain_query", args)
```

### 安全示例

```go
// ✅ 合法查询
nl2sql.QueryTool().Call(ctx, map[string]any{
    "sql": "SELECT * FROM users WHERE id = 1",
})

// ❌ 危险查询（会被拒绝）
nl2sql.QueryTool().Call(ctx, map[string]any{
    "sql": "DELETE FROM users WHERE id = 1",     // DELETE 被禁止
})
nl2sql.QueryTool().Call(ctx, map[string]any{
    "sql": "DROP TABLE users",                   // DROP 被禁止
})
nl2sql.QueryTool().Call(ctx, map[string]any{
    "sql": "SELECT * FROM users; DROP TABLE x", // 多条语句被禁止
})
nl2sql.QueryTool().Call(ctx, map[string]any{
    "sql": "SELECT * FROM users UNION SELECT * FROM passwords", // UNION 被禁止
})
```

## 最佳实践

### 文件系统工具

1. **选择安全的根目录**：不要让根目录指向系统目录
   ```go
   // ✅ 好：专用的知识库目录
   fs, _ := builtin.NewFileSystemTool("/app/knowledge_base")

   // ❌ 坏：系统目录
   fs, _ := builtin.NewFileSystemTool("/")
   ```

2. **定期检查根目录**：确保文件系统工具的根目录有适当的权限

3. **结合 Agent 使用**：通过 Agent 调用，让模型自动使用这些工具

### NL2SQL 工具

1. **使用只读账号**：这是最重要的安全措施
   ```sql
   -- 在数据库中创建只读用户
   CREATE USER 'readonly'@'%' IDENTIFIED BY 'password';
   GRANT SELECT ON mydb.* TO 'readonly'@'%';
   ```

2. **限制数据库权限**：只给必要的 SELECT 权限

3. **监控查询**：记录所有执行的查询，便于审计

4. **结合 Schema 工具使用**：先获取表结构，再生成正确的查询

## 测试

运行测试验证安全防护：

```bash
go test ./builtin/... -v
```

测试覆盖：
- 路径遍历攻击防护
- 绝对路径拒绝
- 文件读取截断
- SQL 关键字过滤
- 多条语句检测
- UNION 注入检测

## 注意事项

1. **纵深防御**：这些工具提供多层防护，但不应作为唯一的安全措施
2. **数据库账号**：NL2SQL 必须配合只读数据库账号使用
3. **日志审计**：建议记录所有工具调用，便于安全审计
4. **定期更新**：关注安全漏洞，及时更新过滤规则

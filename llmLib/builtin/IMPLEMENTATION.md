# 内置工具实现总结

本文档总结了在 `builtin` 包中实现的带安全约束的内置工具。

## 完成的需求

根据文档 `docs/M06-工具系统、MCP与Skills.md` 第 6.4 节的要求，实现了：

### 1. 文件系统路径围栏（Path Jail）

**文件**：`builtin/filesystem.go`

**安全特性**：
- ✅ 路径围栏：所有文件访问限制在指定根目录内
- ✅ 防止路径遍历：拒绝 `../../etc/passwd` 等越界请求
- ✅ 拒绝绝对路径：防止通过绝对路径绕过围栏
- ✅ 大小限制：文件读取限制 100KB
- ✅ 自动创建目录：写入时自动创建缺失的父目录

**提供的工具**：
1. `read_file` - 读取文本文件
2. `write_file` - 写入文本文件
3. `list_files` - 列出目录内容

**核心实现**：
```go
// safePath 是路径围栏的核心
func (fs *FileSystem) safePath(p string) (string, error) {
    // 1. 拒绝绝对路径
    if filepath.IsAbs(p) {
        return "", fmt.Errorf("拒绝绝对路径，只能使用相对路径: %s", p)
    }

    // 2. 规范化路径
    clean := filepath.Clean(filepath.Join(fs.root, p))

    // 3. 检查是否在根目录内
    if !strings.HasPrefix(clean, fs.root+string(os.PathSeparator)) {
        return "", fmt.Errorf("路径越界，拒绝访问: %s", p)
    }

    return clean, nil
}
```

### 2. NL2SQL 只读防护

**文件**：`builtin/nl2sql.go`

**安全特性**：
- ✅ 只允许 SELECT 查询
- ✅ 关键字过滤：INSERT/UPDATE/DELETE/DROP/ALTER/TRUNCATE/GRANT 等
- ✅ 防注入检测：UNION、注释、多条语句
- ✅ 超时控制：查询超时 5 秒
- ✅ 行数限制：最多返回 50 行
- ✅ Schema 查询：安全的元数据查询

**提供的工具**：
1. `nl2sql_query` - 执行只读 SQL 查询
2. `get_db_schema` - 获取数据库表结构
3. `explain_query` - 分析查询执行计划

**核心实现**：
```go
// isSelectOnly 是代码层粗校验
func isSelectOnly(query string) error {
    q := strings.TrimSpace(strings.ToLower(query))

    // 1. 必须 SELECT 开头
    if !strings.HasPrefix(q, "select") {
        return fmt.Errorf("只允许 SELECT 查询")
    }

    // 2. 禁止多条语句
    if strings.Contains(q, ";") && !strings.HasSuffix(q, ";") {
        return fmt.Errorf("禁止多条语句")
    }

    // 3. 关键字黑名单
    for _, kw := range []string{"insert", "update", "delete", "drop", ...} {
        if strings.Contains(q, kw) {
            return fmt.Errorf("检测到禁止的关键字: %s", kw)
        }
    }

    return nil
}
```

**重要提示**：代码层过滤是辅助防线，真正的安全来自：
- 使用**只读数据库账号**
- 数据库层的权限控制
- 查询审计和监控

## 文件结构

```
builtin/
├── filesystem.go      # 文件系统工具（路径围栏）
├── nl2sql.go          # NL2SQL 工具（只读防护）
├── builtin_test.go    # 完整测试套件
├── README.md          # 使用文档
└── examples/
    └── usage_example.go  # 使用示例
```

## 测试覆盖

运行 `go test ./builtin/... -v` 可以看到完整的测试覆盖：

### 文件系统工具测试
- ✅ 路径遍历攻击防护（5 种场景）
- ✅ 绝对路径拒绝
- ✅ 正常文件读取
- ✅ 大文件截断（100KB 限制）
- ✅ 文件不存在错误
- ✅ 写入文件
- ✅ 子目录自动创建
- ✅ 列出目录内容

### NL2SQL 工具测试
- ✅ SELECT 查询（正常场景）
- ✅ INSERT/UPDATE/DELETE 拒绝
- ✅ DROP/ALTER/TRUNCATE 拒绝
- ✅ 多条语句检测
- ✅ UNION 注入检测
- ✅ 注释注入检测
- ✅ 关键字过滤

**总计**：35 个测试全部通过 ✅

## 使用方法

### 基础用法

```go
import "github.com/Effortful-lion/agent-study/llmLib/builtin"

// 1. 创建文件系统工具
fs, _ := builtin.NewFileSystemTool("/app/knowledge_base")
registry := tool.NewRegistryToolSet()
registry.Register(fs.ReadFileTool())
registry.Register(fs.WriteFileTool())
registry.Register(fs.ListFilesTool())

// 2. 创建 NL2SQL 工具（可选）
db, _ := sql.Open("mysql", "readonly_user:pwd@tcp(localhost:3306)/mydb")
nl2sql := builtin.NewNL2SQL(db)
registry.Register(nl2sql.QueryTool())
registry.Register(nl2sql.GetSchemaTool())

// 3. 在 Agent 中使用
agent := agent.New(provider, model, registry)
agent.Run(ctx, "读取知识库中的文档")
```

### 安全配置

```go
// ✅ 推荐：使用专用的知识库目录
fs, err := builtin.NewFileSystemTool("/app/knowledge_base")

// ✅ 推荐：使用只读数据库账号
db, err := sql.Open("mysql", "readonly_user:password@...")

// ❌ 不推荐：使用根目录
fs, err := builtin.NewFileSystemTool("/")

// ❌ 危险：使用管理员账号
db, err := sql.Open("mysql", "admin:password@...")
```

## 安全原则

### 纵深防御

两个工具都采用纵深防御策略：

**文件系统工具**：
1. 拒绝绝对路径
2. 路径规范化
3. 根目录前缀检查
4. 读取大小限制

**NL2SQL 工具**：
1. 代码层关键字过滤
2. 查询超时控制
3. 结果行数限制
4. 数据库层只读权限（最重要）

### 最小权限

- 文件系统工具只访问指定目录
- NL2SQL 只执行 SELECT 查询
- 限制返回数据量

### 失败安全

- 可疑请求直接拒绝，不降级处理
- 超时强制取消
- 错误信息清晰但不泄露系统细节

## 与文档的对应关系

| 文档要求 | 实现文件 | 状态 |
|---------|---------|------|
| 文件系统路径围栏 | filesystem.go | ✅ 完成 |
| NL2SQL 只读防护 | nl2sql.go | ✅ 完成 |
| safePath 实现 | filesystem.go:46-54 | ✅ 完成 |
| isSelectOnly 实现 | nl2sql.go:37-62 | ✅ 完成 |
| runReadOnly 实现 | nl2sql.go:65-114 | ✅ 完成 |
| 读取量限制 | filesystem.go:104-109 | ✅ 完成 |
| 查询超时 | nl2sql.go:77-79 | ✅ 完成 |
| 行数限制 | nl2sql.go:88-102 | ✅ 完成 |

## 后续改进建议

虽然核心需求已完成，但还可以进一步改进：

1. **文件系统工具**
   - 添加文件写入验证（防止写入恶意内容）
   - 支持文件类型检查
   - 添加文件大小限制（防止写入超大文件）
   - 记录文件访问日志

2. **NL2SQL 工具**
   - 更复杂的 SQL 解析（使用真正的 SQL 解析器）
   - 支持查询结果缓存
   - 添加查询复杂度分析
   - 支持查询黑名单/白名单
   - 记录所有查询日志

3. **通用改进**
   - 添加指标收集（Prometheus）
   - 支持工具调用限流
   - 添加权限管理
   - 支持自定义安全规则

## 验证

所有测试通过：
```bash
$ go test ./builtin/...
Go test: 35 passed in 2 packages
```

构建成功：
```bash
$ go build ./builtin/...
Go build: Success
```

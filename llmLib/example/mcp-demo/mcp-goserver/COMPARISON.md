# 手写版 vs mcp-go 版：详细对比

## 📊 总体对比

| 指标 | 手写版 | mcp-go 版 | 差异 |
|------|--------|----------|------|
| **代码行数** | ~370 行 | ~90 行 | **-76%** |
| **文件数** | 1 (server.go) | 1 (main.go) | 相同 |
| **依赖** | 仅标准库 + 内部包 | mcp-go 库 | mcp-go 引入外部依赖 |
| **编译时间** | ~1s | ~2s | mcp-go 略慢（依赖更多） |
| **运行时依赖** | 无外部依赖 | 需要 mcp-go 库 | 手写版更轻量 |

---

## 📝 代码对比

### 1. Server 创建

#### 手写版

```go
// server.go (35 行)
type handwritternServer struct {
    name    string
    version string
    tools   map[string]toolDef
}

func newHandwrittenServer(name, version string) *handwritternServer {
    s := &handwritternServer{
        name:    name,
        version: version,
        tools:   make(map[string]toolDef),
    }
    s.registerTools()
    return s
}
```

**代码量：** 8 行（不包括 registerTools）

#### mcp-go 版

```go
// main.go (1 行)
s := server.NewMCPServer(
    "mcp-go-demo-server",
    "1.0.0",
)
```

**代码量：** 1 行

**差异：**
- ✅ mcp-go：库自动创建 Server 结构
- ✅ mcp-go：内置 Server 元数据管理
- ⚠️ 手写：需要手动定义结构体

---

### 2. 工具定义

#### 手写版

```go
// server.go (28 行)
type toolDef struct {
    name        string
    description string
    inputSchema json.RawMessage
    handler     func(ctx context.Context, args map[string]any) (string, error)
}

func (s *handwritternServer) registerTools() {
    s.tools["get_time"] = toolDef{
        name:        "get_time",
        description: "获取当前时间，支持按 IANA 时区格式化",
        inputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "timezone": {
                    "type": "string",
                    "description": "IANA 时区名"
                }
            }
        }`),
        handler: func(ctx context.Context, args map[string]any) (string, error) {
            // 处理逻辑
        },
    }
}
```

**代码量：** ~50 行（两个工具）

#### mcp-go 版

```go
// main.go (17 行)
getTimeTool := mcp.NewTool(
    "get_time",
    mcp.WithDescription("获取当前时间，支持按 IANA 时区格式化"),
    mcp.WithObject("timezone",
        mcp.Description("IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"),
    ),
)
s.AddTool(getTimeTool, handleGetTime)
```

**代码量：** ~8 行（每个工具）

**差异：**
- ✅ mcp-go：使用辅助函数构建 Schema
- ✅ mcp-go：类型安全，编译时检查
- ✅ mcp-go：更简洁的 API
- ⚠️ 手写：手动编写 JSON Schema
- ⚠️ 手写：完全控制 Schema 结构

---

### 3. 工具处理函数

#### 手写版

```go
// server.go
func (s *handwritternServer) handleCallTool(id int, params json.RawMessage) ([]byte, error) {
    // 1. 解析参数
    var p callToolParams
    if err := json.Unmarshal(params, &p); err != nil {
        return s.errorResponse(&id, -32602, "Invalid params", err.Error())
    }

    // 2. 查找工具
    toolDef, ok := s.tools[p.Name]
    if !ok {
        return s.errorResponse(&id, -32601, "Tool not found", p.Name)
    }

    // 3. 解析参数
    var args map[string]any
    if len(p.Arguments) > 0 {
        if err := json.Unmarshal(p.Arguments, &args); err != nil {
            return s.errorResponse(&id, -32602, "Invalid params", "参数解析失败: "+err.Error())
        }
    }

    // 4. 调用工具
    result, err := toolDef.handler(ctx, args)

    // 5. 构建响应
    var content []contentBlock
    if err != nil {
        content = []contentBlock{{
            Type: "text",
            Text: fmt.Sprintf("Error: %v", err),
        }}
    } else {
        content = []contentBlock{{
            Type: "text",
            Text: result,
        }}
    }

    callResult := callToolResult{
        Content: content,
        IsError: err != nil,
    }

    // 6. 构建 JSON-RPC 响应
    return s.successResponse(id, callResult)
}
```

**代码量：** ~50 行（包括 JSON-RPC 响应构建）

#### mcp-go 版

```go
// main.go
func handleGetTime(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. 获取参数（库已解析）
    timezone := ""
    if args, ok := req.Params.Arguments.(map[string]interface{}); ok {
        if tz, ok := args["timezone"].(string); ok {
            timezone = tz
        }
    }

    // 2. 业务逻辑
    now := time.Now()
    timeStr := fmt.Sprintf("当前时间 (%s): %s", timezone, now.Format(time.RFC3339))

    // 3. 返回结果（库自动构建响应）
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.TextContent{
                Type: "text",
                Text: timeStr,
            },
        },
    }, nil
}
```

**代码量：** ~25 行（纯业务逻辑）

**差异：**
- ⚠️ 手写：手动解析 JSON-RPC 参数
- ⚠️ 手写：手动构建 JSON-RPC 响应
- ✅ mcp-go：库自动解析参数
- ✅ mcp-go：直接返回结果对象
- ✅ mcp-go：更专注于业务逻辑

---

### 4. JSON-RPC 消息处理

#### 手写版

```go
// server.go (70+ 行)
// 1. 读取 stdin
func (s *handwritternServer) serveWithIO(r io.Reader, w io.Writer) error {
    reader := bufio.NewReader(r)
    writer := bufio.NewWriter(w)

    for {
        // 2. 读取一行
        line, err := reader.ReadBytes('\n')
        if err != nil {
            if err == io.EOF {
                return nil
            }
            return fmt.Errorf("读取消息失败: %w", err)
        }

        // 3. 跳过空行
        if len(line) <= 1 {
            continue
        }

        // 4. 处理消息
        resp, err := s.handleMessage(line)
        if err != nil {
            continue
        }

        // 5. 写入响应
        if resp != nil {
            writer.Write(resp)
            writer.WriteByte('\n')
            writer.Flush()
        }
    }
}

// 6. 按 method 分发
func (s *handwritternServer) handleMessage(msg []byte) ([]byte, error) {
    var req jsonRPCRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return s.errorResponse(nil, -32700, "Parse error", err.Error())
    }

    if req.ID == nil {
        s.handleNotification(req.Method, req.Params)
        return nil, nil
    }

    switch req.Method {
    case "initialize":
        return s.handleInitialize(*req.ID, req.Params)
    case "tools/list":
        return s.handleListTools(*req.ID)
    case "tools/call":
        return s.handleCallTool(*req.ID, req.Params)
    default:
        return s.errorResponse(req.ID, -32601, "Method not found", req.Method)
    }
}

// 7. 构建 JSON-RPC 响应
func (s *handwritternServer) successResponse(id int, result any) ([]byte, error) {
    resp := jsonRPCResponse{
        JSONRPC: "2.0",
        ID:      &id,
        Result:  mustMarshal(result),
    }
    return json.Marshal(resp)
}
```

**代码量：** ~100 行

#### mcp-go 版

```go
// main.go (1 行)
server.ServeStdio(s)
```

**代码量：** 1 行（零代码！）

**差异：**
- ⚠️ 手写：~100 行代码处理传输层
- ✅ mcp-go：一行搞定
- ⚠️ 手写：手动管理 stdio
- ✅ mcp-go：库自动管理
- ⚠️ 手写：手动 JSON 序列化
- ✅ mcp-go：库自动序列化

---

### 5. 错误处理

#### 手写版

```go
// server.go (30+ 行)
// 1. 解析错误
if err := json.Unmarshal(msg, &req); err != nil {
    return s.errorResponse(nil, -32700, "Parse error", err.Error())
}

// 2. 方法未找到
default:
    return s.errorResponse(req.ID, -32601, "Method not found", req.Method)

// 3. 参数错误
if err := json.Unmarshal(params, &p); err != nil {
    return s.errorResponse(&id, -32602, "Invalid params", err.Error())
}

// 4. 工具未找到
if !ok {
    return s.errorResponse(&id, -32601, "Tool not found", p.Name)
}

// 5. 构建错误响应
func (s *handwritternServer) errorResponse(id *int, code int, message string, data string) ([]byte, error) {
    resp := jsonRPCResponse{
        JSONRPC: "2.0",
        ID:      id,
        Error: &jsonRPCError{
            Code:    code,
            Message: message,
            Data:    data,
        },
    }
    return json.Marshal(resp)
}
```

**代码量：** ~50 行

#### mcp-go 版

```go
// main.go (0 行，完全由库处理)
// 如果需要返回错误
return &mcp.CallToolResult{
    Content: []mcp.Content{...},
    IsError: true,
}, nil
```

**代码量：** 0 行（库自动处理 JSON-RPC 错误）

**差异：**
- ⚠️ 手写：手动定义错误码
- ⚠️ 手写：手动构建错误响应
- ✅ mcp-go：库自动处理错误
- ✅ mcp-go：只需关注工具级错误（isError）

---

## 🎯 核心差异总结

### 库完全接管的（黑盒）

| 方面 | 手写版控制权 | mcp-go 控制权 |
|------|------------|-------------|
| **JSON-RPC 协议** | ✅ 完全控制 | ⚠️ 库处理 |
| **消息序列化** | ✅ 手动实现 | ⚠️ 库处理 |
| **消息路由** | ✅ switch-case | ⚠️ 库处理 |
| **错误码** | ✅ 自定义 | ⚠️ 库管理 |
| **stdio 通信** | ✅ bufio 管理 | ⚠️ 库处理 |
| **工具注册表** | ✅ map 实现 | ⚠️ 库管理 |
| **响应格式** | ✅ 手动构建 | ⚠️ 库处理 |

### 仍然需要自己写的

| 方面 | 手写版 | mcp-go 版 |
|------|--------|----------|
| **工具处理函数** | handler 函数 | handler 函数 |
| **业务逻辑** | ✅ 自己实现 | ✅ 自己实现 |
| **工具描述** | ✅ 自己写 | ✅ 自己写 |
| **参数 Schema** | ✅ 手动 JSON | ✅ 通过 API |
| **时间格式化** | ✅ 自己实现 | ✅ 自己实现 |
| **表达式计算** | ✅ 自己实现 | ✅ 自己实现 |

---

## 📊 详细对比表

| 功能 | 手写版 | mcp-go 版 | 差异 |
|------|--------|----------|------|
| **Server 创建** | `newHandwrittenServer()` | `server.NewMCPServer()` | mcp-go 更简洁 |
| **工具定义** | 手动 JSON Schema | `mcp.NewTool()` + 辅助函数 | mcp-go 类型安全 |
| **工具注册** | `s.tools[name] = toolDef` | `s.AddTool(tool, handler)` | mcp-go API 更清晰 |
| **参数解析** | `json.Unmarshal` | 自动解析为 `map[string]interface{}` | mcp-go 更简单 |
| **响应构建** | `json.Marshal` + 手动结构 | 直接返回对象 | mcp-go 更直观 |
| **错误处理** | 手动构建错误响应 | 返回 `IsError: true` | mcp-go 更简单 |
| **JSON-RPC** | 完整实现（~100 行） | `server.ServeStdio()` (1 行) | **mcp-go 减少 99%** |
| **方法分发** | `switch req.Method` | 库自动路由 | mcp-go 完全抽象 |
| **初始化握手** | `handleInitialize()` | 库自动处理 | mcp-go 完全抽象 |
| **工具列表** | `handleListTools()` | 库自动生成 | mcp-go 完全抽象 |

---

## 🏆 优劣分析

### 手写版

#### ✅ 优势

1. **完全控制**
   - 可以自定义每个协议细节
   - 可以添加特殊逻辑

2. **无依赖**
   - 仅依赖标准库
   - 更轻量

3. **学习价值**
   - 深入理解 MCP 协议
   - 掌握 JSON-RPC 2.0
   - 理解协议设计

4. **灵活性**
   - 可以修改任何细节
   - 易于调试和扩展

#### ⚠️ 劣势

1. **代码量大**
   - ~370 行 vs ~90 行
   - 维护成本高

2. **容易出错**
   - 协议细节容易出错
   - 需要处理边界情况

3. **重复工作**
   - 每次都要重新实现
   - 无法复用

4. **标准化难**
   - 不同人实现可能不同
   - 难以保证符合规范

---

### mcp-go 版

#### ✅ 优势

1. **代码简洁**
   - ~90 行（减少 76%）
   - 更易维护

2. **可靠性**
   - 库经过测试
   - 符合规范

3. **开发效率**
   - 快速实现
   - 专注业务逻辑

4. **标准化**
   - 符合 MCP 规范
   - 社区认可

#### ⚠️ 劣势

1. **灵活性降低**
   - 无法修改协议细节
   - 受限于库的实现

2. **依赖管理**
   - 需要管理第三方依赖
   - 可能引入安全问题

3. **学习成本**
   - 需要学习库的 API
   - 文档可能不完善

4. **黑盒**
   - 协议层细节被隐藏
   - 调试困难

---

## 🎓 适用场景

### 使用手写版

- ✅ **学习目的**：理解协议细节
- ✅ **教学演示**：展示工作原理
- ✅ **特殊需求**：需要自定义协议行为
- ✅ **极致控制**：完全掌控每个细节
- ✅ **零依赖**：避免第三方依赖

### 使用 mcp-go 版

- ✅ **生产开发**：快速、可靠
- ✅ **标准实现**：符合规范
- ✅ **团队协作**：标准化代码
- ✅ **维护成本**：减少维护负担
- ✅ **快速原型**：快速验证想法

---

## 📝 总结

### 核心结论

1. **代码量：** mcp-go 版减少 76%（370 行 → 90 行）
2. **协议层：** mcp-go 完全接管（减少 99% 代码）
3. **业务逻辑：** 两者相同（处理函数相似）
4. **开发效率：** mcp-go 显著提升
5. **灵活性：** 手写版更高
6. **学习价值：** 手写版更高

### 最佳实践

**推荐：**
- 学习阶段：先用手写版理解协议
- 生产阶段：使用库快速开发
- 复杂需求：评估是否真的需要手写

**避免：**
- 不要重复造轮子（除非有特殊需求）
- 不要过度依赖黑盒（要理解原理）
- 不要忽视库的限制（评估是否满足需求）

---

**创建时间：** 2026-07-31
**版本：** 1.0

# MCP Inspector 使用指南

## 什么是 MCP Inspector？

MCP Inspector 是官方提供的可视化测试工具，用于：
- 查看 MCP Server 提供的工具
- 测试工具调用
- 查看 JSON-RPC 消息历史
- 验证协议实现

---

## 安装

### 方法 1：使用 npx（推荐）

无需安装，直接使用：

```bash
npx @modelcontextprotocol/inspector
```

### 方法 2：全局安装

```bash
npm install -g @modelcontextprotocol/inspector
```

---

## 使用方法

### 1. 启动 Inspector

在 `mcp-goserver` 目录下运行：

```bash
npx @modelcontextprotocol/inspector go run main.go
```

### 2. 自动打开浏览器

Inspector 会自动：
1. 启动 MCP Server 子进程
2. 建立 stdio 连接
3. 执行握手
4. 打开浏览器界面（通常是 http://localhost:5173）

### 3. 在 Inspector 中验证

#### 步骤 3.1：验证工具列表

1. 点击 **"Tools"** 标签
2. 查看工具列表
3. 应该看到两个工具：
   - `get_time`
   - `calc`

#### 步骤 3.2：查看工具详情

点击 `get_time` 工具，查看：

- ✅ **Name:** `get_time`
- ✅ **Description:** `获取当前时间，支持按 IANA 时区格式化`
- ✅ **Input Schema:**
  ```json
  {
    "type": "object",
    "properties": {
      "timezone": {
        "type": "string",
        "description": "IANA 时区名，例如 Asia/Shanghai；为空时使用本地时区"
      }
    }
  }
  ```

#### 步骤 3.3：调用 get_time

1. 选择 `get_time` 工具
2. 输入参数（Arguments）：
   ```json
   {
     "timezone": "Asia/Shanghai"
   }
   ```
3. 点击 **"Call Tool"**
4. 查看响应：
   ```json
   {
     "content": [
       {
         "type": "text",
         "text": "当前时间 (Asia/Shanghai): 2026-07-31T15:47:38+08:00"
       }
     ]
   }
   ```
5. 验证：
   - ✅ 返回 `content` 数组
   - ✅ 包含时间文本
   - ✅ `isError` 不存在或为 `false`

#### 步骤 3.4：调用 calc

1. 选择 `calc` 工具
2. 输入参数：
   ```json
   {
     "expr": "1+1"
   }
   ```
3. 点击 **"Call Tool"**
4. 查看响应：
   ```json
   {
     "content": [
       {
         "type": "text",
         "text": "计算失败: 表达式解析器未实现"
       }
     ],
     "isError": true
   }
   ```
5. 验证：
   - ✅ 返回 `content` 数组
   - ✅ `isError` 为 `true`

#### 步骤 3.5：查看消息历史

1. 点击 **"Messages"** 标签
2. 查看所有 JSON-RPC 消息
3. 验证消息顺序：
   - ✅ `initialize` 请求 → 响应
   - ✅ `notifications/initialized`（无响应）
   - ✅ `tools/list` 请求 → 响应
   - ✅ `tools/call` 请求 → 响应

---

## 验收点验证清单

使用 Inspector 验证以下验收点：

### ✅ 验收点 1：工具列表正确

- [ ] 在 Tools 标签看到 `get_time` 和 `calc`
- [ ] 工具名称正确
- [ ] 工具描述正确
- [ ] 参数 Schema 正确

### ✅ 验收点 2：get_time 调用成功

- [ ] 调用返回 `content` 数组
- [ ] 内容包含时间文本
- [ ] `isError` 为 `false` 或不存在

### ✅ 验收点 3：calc 调用成功

- [ ] 调用返回 `content` 数组
- [ ] `isError` 为 `true`
- [ ] 包含错误信息

### ✅ 验收点 4：JSON-RPC 消息正确

- [ ] `initialize` 请求/响应格式正确
- [ ] `tools/list` 请求/响应格式正确
- [ ] `tools/call` 请求/响应格式正确
- [ ] 响应包含正确的 `jsonrpc`、`id`、`result`

---

## 常见问题

### Q1: Inspector 无法启动 Server

**可能原因：**
- Server 程序有语法错误
- 缺少依赖

**解决方法：**
```bash
# 先手动测试 Server
go run main.go

# 确保能正常启动
```

### Q2: 工具列表为空

**可能原因：**
- 工具注册失败
- 工具名称重复

**解决方法：**
- 检查 `s.AddTool()` 是否被调用
- 检查工具名称是否唯一

### Q3: 调用工具失败

**可能原因：**
- 工具处理函数返回错误
- 参数格式不正确
- 工具未注册

**解决方法：**
- 检查 Server 的 stderr 输出
- 检查参数 JSON 格式
- 验证工具已注册

### Q4: 响应格式不正确

**可能原因：**
- `CallToolResult` 结构不正确
- `Content` 字段格式错误

**解决方法：**
- 检查 `mcp.CallToolResult` 结构
- 确保使用 `mcp.TextContent`

---

## 截图示例

Inspector 界面包含以下部分：

### Tools 标签

```
┌─────────────────────────────────────┐
│ Tools  Resources  Prompts  Messages │
├─────────────────────────────────────┤
│ • get_time                          │
│   获取当前时间，支持按 IANA 时区... │
│                                      │
│ • calc                              │
│   计算只包含数字、括号、+、-、*、/... │
└─────────────────────────────────────┘
```

### Call Tool 面板

```
┌─────────────────────────────────────┐
│ Tool: get_time                      │
│ Arguments:                          │
│ {                                    │
│   "timezone": "Asia/Shanghai"       │
│ }                                    │
│                                      │
│ [Call Tool]                          │
└─────────────────────────────────────┘
```

### 响应面板

```
┌─────────────────────────────────────┐
│ Response:                            │
│ {                                    │
│   "content": [                       │
│     {                                │
│       "type": "text",                │
│       "text": "当前时间..."          │
│     }                                │
│   ]                                  │
│ }                                    │
└─────────────────────────────────────┘
```

---

## 高级功能

### 查看 Server 日志

Inspector 会显示 Server 的 stderr 输出：

```
[Server] === 使用 mcp-go 库创建的 MCP Server ===
[Server] ✓ 注册工具: get_time, calc
[Server] ✓ 启动 stdio 服务...
[Server] ---
```

### 重新连接

如果连接断开：
1. 停止 Inspector（Ctrl+C）
2. 重新运行 `npx @modelcontextprotocol/inspector go run main.go`

### 调试模式

启用详细日志：
```bash
DEBUG=* npx @modelcontextprotocol/inspector go run main.go
```

---

## 验证完成

完成以下所有项目即表示验证通过：

- [x] Inspector 成功启动
- [x] 工具列表显示正确（2 个工具）
- [x] get_time 调用成功
- [x] calc 调用成功
- [x] 响应格式正确
- [x] isError 标志正确
- [x] JSON-RPC 消息格式正确

---

**创建时间：** 2026-07-31
**状态：** 待手动验证

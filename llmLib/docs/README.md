# 文档索引

本文档目录包含 llmLib 项目的所有文档。

## 📚 文档清单

### MCP 相关文档

1. **[MCP-DEEP-DIVE.md](MCP-DEEP-DIVE.md)** - MCP 协议深度解析
   - 三角色架构（Host/Client/Server）
   - 四类核心原语（Tools/Resources/Prompts/Logging）
   - JSON-RPC 消息格式
   - 会话生命周期
   - stdio vs Streamable HTTP
   - 协议对比
   - 实战示例

2. **[MCP-LEARNING-PATH.md](MCP-LEARNING-PATH.md)** - MCP 学习路径
   - 层层递进的学习路径
   - Level 1: Server 端
   - Level 2: Client ↔ Server 通信
   - Level 3: Bridge 桥接
   - Level 4: Agent 集成
   - 实战建议和验证清单

### 工具系统文档

3. **[M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md)** - 工具系统完整教程
   - 类型安全工具（TypedTool）
   - Function Calling 协议
   - OpenAI/Anthropic/Gemini 边界映射
   - 内置工具安全（文件系统路径围栏、NL2SQL）
   - MCP Server/Client/Bridge 实现
   - Claude Skills
   - 生态方向与安全

### 其他文档

4. **[provider的边界映射.md](provider的边界映射.md)** - Provider 边界映射说明

5. **[后续目标定位.md](后续目标定位.md)** - 项目后续目标

---

## 🗺️ 学习路径建议

### MCP 入门

1. 先阅读 **[MCP-LEARNING-PATH.md](MCP-LEARNING-PATH.md)** 了解学习路径
2. 按照 Level 1 → Level 4 的顺序逐步实践
3. 遇到问题时查阅 **[MCP-DEEP-DIVE.md](MCP-DEEP-DIVE.md)** 深入理解协议

### 工具系统

1. 阅读 **[M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md)**
2. 理解工具系统的完整设计
3. 学习 Function Calling 和 MCP 的区别

### Provider 开发

1. 阅读 **[provider的边界映射.md](provider的边界映射.md)**
2. 了解如何添加新的 LLM Provider

---

## 📂 文档组织

```
docs/
├── README.md                          # 本文档
├── MCP-DEEP-DIVE.md                   # MCP 协议深度解析
├── MCP-LEARNING-PATH.md               # MCP 学习路径
├── M06-工具系统、MCP与Skills.md       # 工具系统完整教程
├── provider的边界映射.md               # Provider 边界映射
└── 后续目标定位.md                      # 项目后续目标
```

---

## 🔍 快速查找

| 我想了解... | 查看文档 |
|-----------|---------|
| MCP 是什么 | [MCP-LEARNING-PATH.md](MCP-LEARNING-PATH.md) Level 1 |
| MCP 协议细节 | [MCP-DEEP-DIVE.md](MCP-DEEP-DIVE.md) |
| 如何学习 MCP | [MCP-LEARNING-PATH.md](MCP-LEARNING-PATH.md) |
| 工具系统设计 | [M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md) |
| Function Calling | [M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md) §6.3 |
| 内置工具安全 | [M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md) §6.4 |
| Claude Skills | [M06-工具系统、MCP与Skills.md](M06-工具系统、MCP与Skills.md) §6.8 |
| Provider 开发 | [provider的边界映射.md](provider的边界映射.md) |

---

## 📝 文档贡献

如需添加或修改文档，请：

1. 在 `docs/` 目录创建或编辑 Markdown 文件
2. 更新本文件的文档清单
3. 确保文档之间有清晰的边界，避免内容重叠
4. 在文档开头添加简要说明和目录

---

最后更新：2025-07-29

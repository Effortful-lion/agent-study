# mini-kb

本地命令行知识库问答 Demo，基于 Go + llmLib 实现。

## 功能

- 从 Markdown/TXT 文档自动建立索引
- 关键词混合检索（标题加权 + 内容匹配）
- Agentic RAG：Agent 自主决定何时检索、检索什么
- 短期记忆：多轮对话保留上下文
- 长期记忆：用户偏好持久化
- 来源标注：答案附带来源文件

## 快速开始

```bash
# 1. 初始化
mini-kb init

# 2. 导入文档
mini-kb ingest ./data

# 3. 单轮问答
mini-kb ask "Go语言的Goroutine是什么"

# 4. 连续对话（带短期记忆）
mini-kb chat
```

## 命令

| 命令 | 说明 |
|------|------|
| `mini-kb init` | 初始化存储目录 |
| `mini-kb ingest <dir>` | 扫描目录并建立索引 |
| `mini-kb ask <question>` | 单轮知识库问答 |
| `mini-kb chat` | 进入连续对话模式 |
| `mini-kb status` | 查看索引状态 |
| `mini-kb sessions` | 查看会话历史 |

## 配置

通过命令行参数或环境变量配置 LLM：

```bash
mini-kb ask "问题" --provider openai --model gpt-4 --api-key $OPENAI_API_KEY
```

支持的提供者：`openai`、`deepseek`、`doubao`、`qwen`、`claude` 等。

## 架构

```
mini-kb/
├── main.go              # CLI 入口
├── internal/
│   ├── config/          # 全局配置
│   ├── document/        # 文档模型
│   ├── index/           # 文本切分和关键词提取
│   ├── retriever/       # 关键词混合检索
│   ├── memory/          # 会话记忆和用户偏好
│   ├── tools/           # 知识库工具（供 Agent 调用）
│   └── agent/           # 知识库 Agent 封装
└── storage/             # JSON 持久化存储
```

## 阶段

- **第一阶段**：文档读取、文本切分、JSON 索引、关键词检索、单轮问答
- **第二阶段**：Agentic RAG（工具调用、多次检索）
- **第三阶段**：记忆能力（会话持久化、多轮上下文、用户偏好）
- **第四阶段**：增强检索（Embedding、向量索引、重排序）

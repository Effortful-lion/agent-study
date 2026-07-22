# 背景

为一个“读资料、按资料作答”的文档问答助手设计提示词与上下文预算。

# 需求

设计一套完整上下文，并做 token 预算分析。

# 验收点

1. 用 prompt.Template 写一个 System 提示词模板，包含角色、规则、资料占位和 1-2 个 few-shot 示例，并开启 missingkey=error；
2. 设计一个需要结构化输出的子任务，例如“判断问题难度，返回 {level, reason}”，用 M02 2.8的 schema.Generate 生成 schema 并写进提示词； =
3. 用 estimateTokens 估算 System、工具定义、一段示例历史、检索片段各部分的 token，并填一张 Budget 表； 
4. 标出哪些部分适合作为 Prompt Caching 的稳定前缀； 
5. 说明你会把“当前时间”放在哪，以及为什么； 
6. 思考如果某次命中的资料片段特别长、把预算撑爆了，你会如何处理。
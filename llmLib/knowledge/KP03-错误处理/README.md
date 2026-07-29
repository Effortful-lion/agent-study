# KP03 — 错误处理（errutil 包）

## 标题和概述

`errutil` 包是 llmLib 的**错误分类与重试中枢**，为 Agent 运行时提供统一的错误类型、智能分类和可靠的重试机制。它解决以下核心问题：

- 将 LLM 调用过程中形形色色的错误（网络超时、授权失败、限流、模型异常、工具错误等）归类为可操作的类别
- 通过 `AgentError` 结构体携带分类信息，使上层逻辑能根据错误类型做出差异化决策
- 实现基于指数退避的 `RetryWithBackoff`，自动处理瞬时故障

## 核心概念

### 1. ErrorCategory — 错误分类体系

11 种错误类别，覆盖 LLM Agent 的完整故障谱：

| 类别 | 说明 | 是否可重试 |
|---|---|---|
| `ErrCategoryNetwork` | 网络问题（连接拒绝、DNS 解析失败等） | ✅ |
| `ErrCategoryAuth` | 授权问题（API Key 无效、权限不足） | ❌ |
| `ErrCategoryRateLimited` | 限流（429 Too Many Requests） | ✅ |
| `ErrCategoryModel` | 模型本身出错（生成失败、格式异常） | ✅ |
| `ErrCategoryTool` | 工具执行失败（参数错误、工具内部错误） | ❌ |
| `ErrCategoryToolNotFound` | 工具不存在 | ❌ |
| `ErrCategoryTimeout` | 超时（408/504 或 context.DeadlineExceeded） | ✅ |
| `ErrCategoryCanceled` | 上下文取消（context.Canceled） | ❌ |
| `ErrCategoryNotFound` | 资源不存在（404） | ❌ |
| `ErrCategoryProviderError` | Provider 服务端错误（5xx） | ✅ |
| `ErrCategoryUnknown` | 未知错误，兜底分类 | ❌ |

可重试的类别在重试时会有不同的退避策略。

### 2. AgentError — 统一错误结构

```go
type AgentError struct {
    Category  ErrorCategory  // 错误分类
    Message   string        // 人类可读描述
    Err       error         // 原始错误（用于调试）
    Retryable bool          // 是否可重试
}
```

- `Error()` 方法格式化输出：`[category] message: original error`
- JSON 序列化时 `Err` 字段被忽略（`json:"-"`），其余字段可序列化

### 3. ClassifyError — 智能错误分类

`ClassifyError(err error, statusCode int)` 综合 HTTP 状态码和错误消息进行判断：

**HTTP 状态码优先匹配：**
- 408/504 → `ErrCategoryTimeout`（可重试）
- 429 → `ErrCategoryRateLimited`（可重试）
- 401/403 → `ErrCategoryAuth`（不可重试）
- 404 → `ErrCategoryNotFound`（不可重试）
- 5xx → `ErrCategoryProviderError`（可重试）

**错误消息关键字匹配：**
- "connection refused" / "timeout" / "dns" → `ErrCategoryNetwork`
- "api key" / "invalid auth" / "permission" → `ErrCategoryAuth`
- "rate limit" / "quota" / "throttled" → `ErrCategoryRateLimited`
- "model" / "generation" → `ErrCategoryModel`
- "not found" → `ErrCategoryNotFound`
- "500" / "5xx" → `ErrCategoryProviderError`

**Context 错误匹配：**
- `context.DeadlineExceeded` → `ErrCategoryTimeout`
- `context.Canceled` → `ErrCategoryCanceled`

### 4. RetryWithBackoff — 指数退避重试

```go
RetryWithBackoff(baseDelay, maxDelay time.Duration, maxRetries int, fn func() error) error
```

- 第一次失败后等待 `baseDelay`，每次失败延迟翻倍
- 延迟上限为 `maxDelay`
- 最后一次重试失败后直接返回最终错误
- 适用于网络抖动、限流、Provider 临时故障等场景

## 类型/函数清单

### 错误分类常量
| 常量 | 源文件 | 说明 |
|---|---|---|
| `ErrCategoryNetwork` | `errutil/error.go` | 网络错误（可重试） |
| `ErrCategoryAuth` | `errutil/error.go` | 授权错误（不可重试） |
| `ErrCategoryRateLimited` | `errutil/error.go` | 限流错误（可重试） |
| `ErrCategoryModel` | `errutil/error.go` | 模型错误（可重试） |
| `ErrCategoryTool` | `errutil/error.go` | 工具错误（不可重试） |
| `ErrCategoryToolNotFound` | `errutil/error.go` | 工具不存在（不可重试） |
| `ErrCategoryTimeout` | `errutil/error.go` | 超时错误（可重试） |
| `ErrCategoryCanceled` | `errutil/error.go` | 取消错误（不可重试） |
| `ErrCategoryNotFound` | `errutil/error.go` | 资源不存在（不可重试） |
| `ErrCategoryProviderError` | `errutil/error.go` | Provider 错误（可重试） |
| `ErrCategoryUnknown` | `errutil/error.go` | 未知错误（不可重试） |

### 类型与函数
| 类型/函数 | 源文件 | 说明 |
|---|---|---|
| `ErrorCategory` | `errutil/error.go` | 错误分类类型（`string` 别名） |
| `AgentError` | `errutil/error.go` | Agent 统一错误结构体 |
| `AgentError.Error()` | `errutil/error.go` | 实现 `error` 接口 |
| `NewAgentError(category, message, err, retryable)` | `errutil/error.go` | 创建 AgentError |
| `ClassifyError(err, statusCode)` | `errutil/error.go` | 错误分类与可重试判断 |
| `RetryWithBackoff(baseDelay, maxDelay, maxRetries, fn)` | `errutil/error.go` | 指数退避重试 |

## 使用示例

```go
import (
    "context"
    "net/http"
    "time"
    "github.com/Effortful-lion/agent-study/llmLib/errutil"
)

// 1. 创建 AgentError
ae := errutil.NewAgentError(
    errutil.ErrCategoryRateLimited,
    "请求过于频繁",
    nil,
    true,
)
fmt.Println(ae.Error()) // [rate_limited] 请求过于频繁

// 2. ClassifyError — 从 HTTP 状态码和错误推断分类
category, retryable := errutil.ClassifyError(
    fmt.Errorf("connection refused"),
    0, // 无 HTTP 状态码
)
// category = "network", retryable = true

category2, retryable2 := errutil.ClassifyError(nil, http.StatusUnauthorized)
// category2 = "auth", retryable2 = false

// 3. 结合 Provider 使用（参考 KP02）
resp, err := provider.Chat(ctx, cfg, messages)
if err != nil {
    category, retryable := errutil.ClassifyError(err, httpStatus)
    if retryable {
        // 自动重试
        retryErr := errutil.RetryWithBackoff(
            1*time.Second,   // 基础延迟 1s
            30*time.Second,  // 最大延迟 30s
            3,               // 最多重试 3 次
            func() error {
                resp, err = provider.Chat(ctx, cfg, messages)
                return err
            },
        )
        if retryErr != nil {
            // 最终失败，按类别处理
            switch category {
            case errutil.ErrCategoryRateLimited:
                // 通知用户降低频率
            case errutil.ErrCategoryNetwork:
                // 检查网络连接
            }
        }
    } else {
        // 不可重试，直接返回
        return nil, errutil.NewAgentError(category, err.Error(), err, false)
    }
}

// 4. Context 取消检测
ctx, cancel := context.WithCancel(context.Background())
cancel()
_, err := provider.Chat(ctx, cfg, messages)
category, _ = errutil.ClassifyError(err, 0)
// category = "canceled"
```

## 关联知识点

- **KP02-Provider体系**：Provider 返回的错误通过 `ClassifyError` 进行分类，决定是否重试
- **KP04-工具系统**：`tool.Registry.Call` 使用 `ErrCategoryToolNotFound` 和 `ErrCategoryTool` 创建工具相关错误
- **Agent 模块**：Agent 主循环根据 `AgentError.Category` 和 `Retryable` 字段决定是否重试模型调用或终止任务
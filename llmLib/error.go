// 文件职责：
// - 定义 Agent 错误分类体系，将错误分为网络、授权、限流、工具、模型等类别。
// - 实现 AgentError 结构体，包含分类、消息、原始错误和是否可重试。
// - 提供 ClassifyError 用于识别错误类别，RetryWithBackoff 实现指数退避重试。

package llmlib

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrorCategory 定义错误分类，用于判断错误是否可重试和采取不同处理策略。
type ErrorCategory string

const (
	ErrCategoryNetwork        ErrorCategory = "network"        // 网络问题（超时、DNS、连接失败等）
	ErrCategoryAuth           ErrorCategory = "auth"           // 授权问题（API Key 无效、权限不足等）
	ErrCategoryRateLimited    ErrorCategory = "rate_limited"   // 限流（请求过于频繁、配额耗尽等）
	ErrCategoryModel          ErrorCategory = "model"          // 模型本身出错（生成失败、格式错误等）
	ErrCategoryTool           ErrorCategory = "tool"           // 工具执行失败（参数错误、工具不存在等）
	ErrCategoryToolNotFound   ErrorCategory = "tool_not_found"
	ErrCategoryTimeout        ErrorCategory = "timeout"        // 超时错误
	ErrCategoryCanceled       ErrorCategory = "canceled"       // 上下文取消错误
	ErrCategoryNotFound       ErrorCategory = "not_found"      // 资源不存在错误
	ErrCategoryProviderError  ErrorCategory = "provider_error" // Provider 错误
	ErrCategoryUnknown        ErrorCategory = "unknown"        // 未知错误，兜底分类
)

// AgentError 是 Agent 运行时的统一错误类型，包含分类信息以便调用方采取合适策略。
// Category: 错误分类，决定是否重试和重试策略
// Message: 人类可读的错误描述
// Err: 原始错误，用于调试和日志
// Retryable: 是否可重试
type AgentError struct {
	Category  ErrorCategory `json:"category"`
	Message   string        `json:"message"`
	Err       error         `json:"-"`
	Retryable bool          `json:"retryable"`
}

// Error 实现 error 接口，返回错误消息。
func (e *AgentError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Category, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Category, e.Message)
}

// NewAgentError 创建一个新的 AgentError。
// category: 错误分类
// message: 错误描述
// err: 原始错误，可为 nil
// retryable: 是否可重试
func NewAgentError(category ErrorCategory, message string, err error, retryable bool) *AgentError {
	return &AgentError{
		Category:  category,
		Message:   message,
		Err:       err,
		Retryable: retryable,
	}
}

// ClassifyError 从错误消息或 HTTP 状态码推断错误类别和是否可重试。
// 支持常见的 LLM API 错误模式识别，如网络超时、401/403 授权错误、429 限流等。
func ClassifyError(err error, statusCode int) (ErrorCategory, bool) {
	if err == nil && statusCode == http.StatusOK {
		return "", true
	}
	if err == nil {
		return ErrCategoryUnknown, false
	}
	if statusCode != 0 {
		switch statusCode {
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			return ErrCategoryTimeout, true
		case http.StatusTooManyRequests:
			return ErrCategoryRateLimited, true
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrCategoryAuth, false
		case http.StatusNotFound:
			return ErrCategoryNotFound, false
		}
		if statusCode >= 500 {
			return ErrCategoryProviderError, true
		}
	}
	switch err {
	case context.DeadlineExceeded:
		return ErrCategoryTimeout, true
	case context.Canceled:
		return ErrCategoryCanceled, false
	}
	msg := err.Error()
	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "connection refused") ||
		strings.Contains(msgLower, "timeout") ||
		strings.Contains(msgLower, "dns"):
		return ErrCategoryNetwork, true
	case strings.Contains(msgLower, "api key") ||
		strings.Contains(msgLower, "invalid auth") ||
		strings.Contains(msgLower, "permission"):
		return ErrCategoryAuth, false
	case strings.Contains(msgLower, "rate limit") ||
		strings.Contains(msgLower, "quota") ||
		strings.Contains(msgLower, "throttled"):
		return ErrCategoryRateLimited, true
	case strings.Contains(msgLower, "model") ||
		strings.Contains(msgLower, "generation"):
		return ErrCategoryModel, true
	case strings.Contains(msgLower, "not found"):
		return ErrCategoryNotFound, false
	case strings.Contains(msgLower, "500") || strings.Contains(msgLower, "5xx"):
		return ErrCategoryProviderError, true
	}
	return ErrCategoryUnknown, false
}

// RetryWithBackoff 实现指数退避重试机制，适合网络和限流类错误。
// baseDelay: 基础延迟时间
// maxDelay: 最大延迟时间
// maxRetries: 最大重试次数
// fn: 待重试的函数，返回错误
// 返回: 最终错误，如果成功则返回 nil
func RetryWithBackoff(baseDelay, maxDelay time.Duration, maxRetries int, fn func() error) error {
	delay := baseDelay
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		}
		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
	return fn()
}
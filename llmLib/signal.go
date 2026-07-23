package llmlib

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// SignalContext 返回一个可被 Ctrl+C / SIGTERM 取消的上下文。
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

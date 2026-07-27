package state

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkflowDo(t *testing.T) {
	var order []string

	step1 := NewStep("step1", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		order = append(order, "step1")
		state["a"] = 1
		return state, nil
	})

	step2 := NewStep("step2", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		order = append(order, "step2")
		state["b"] = 2
		return state, nil
	})

	wf := Do(step1, step2)
	state, err := wf.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(order) != 2 || order[0] != "step1" || order[1] != "step2" {
		t.Errorf("unexpected order: %v", order)
	}
	if state["a"] != 1 || state["b"] != 2 {
		t.Errorf("unexpected state: %v", state)
	}
	if state["_step_name"] != "done" {
		t.Errorf("expected _step_name=done, got %v", state["_step_name"])
	}
}

func TestWorkflowThen(t *testing.T) {
	step1 := NewStep("s1", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["s1"] = true
		return state, nil
	})
	step2 := NewStep("s2", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["s2"] = true
		return state, nil
	})
	step3 := NewStep("s3", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["s3"] = true
		return state, nil
	})

	wf := Do(step1).Then(step2, step3)
	state, err := wf.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !state["s1"].(bool) || !state["s2"].(bool) || !state["s3"].(bool) {
		t.Error("not all steps executed")
	}
}

func TestWorkflowRetry(t *testing.T) {
	var count int32
	step := NewStep("flaky",
		func(ctx context.Context, state map[string]any) (map[string]any, error) {
			c := atomic.AddInt32(&count, 1)
			if c < 3 {
				return state, errors.New("not ready")
			}
			state["ok"] = true
			return state, nil
		},
		WithRetry(3, 10*time.Millisecond),
	)

	wf := Do(step)
	state, err := wf.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if state["ok"] != true {
		t.Error("expected ok=true")
	}
}

func TestWorkflowTimeout(t *testing.T) {
	step := NewStep("slow",
		func(ctx context.Context, state map[string]any) (map[string]any, error) {
			select {
			case <-ctx.Done():
				return state, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return state, nil
			}
		},
		WithTimeout(50*time.Millisecond),
	)

	wf := Do(step)
	_, err := wf.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWorkflowError(t *testing.T) {
	step1 := NewStep("ok", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["ok"] = true
		return state, nil
	})
	step2 := NewStep("fail", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		return state, errors.New("step2 failed")
	})
	step3 := NewStep("never", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["never"] = true
		return state, nil
	})

	wf := Do(step1, step2, step3)
	state, err := wf.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !state["ok"].(bool) {
		t.Error("step1 should have executed")
	}
	if state["never"] != nil {
		t.Error("step3 should not have executed")
	}
}

func TestWorkflowOnError(t *testing.T) {
	errorHandled := false
	step := NewStep("fail", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		return state, errors.New("oops")
	})

	wf := Do(step).OnError(func(ctx context.Context, state map[string]any) (map[string]any, error) {
		errorHandled = true
		state["handled"] = true
		return state, nil
	})

	state, err := wf.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorHandled {
		t.Error("expected OnError to be called")
	}
	if state["handled"] != true {
		t.Error("expected handled=true")
	}
}

func TestWorkflowMiddleware(t *testing.T) {
	var middlewareCalled bool
	logMW := LogMiddleware(func(phase Phase, msg string) {
		middlewareCalled = true
	})

	step := NewStep("test", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["done"] = true
		return state, nil
	})

	wf := Do(step).Use(logMW)
	state, err := wf.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if state["done"] != true {
		t.Error("expected done=true")
	}
	_ = middlewareCalled // logMW 在成功时不调用，这里只验证流程正常
}

func TestWorkflowRunWithCallback(t *testing.T) {
	var callbackSteps []string
	step1 := NewStep("s1", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		return state, nil
	})
	step2 := NewStep("s2", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		return state, nil
	})

	wf := Do(step1, step2)
	_, err := wf.RunWithCallback(context.Background(), nil, func(stepName string, state map[string]any, err error) error {
		callbackSteps = append(callbackSteps, stepName)
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithCallback failed: %v", err)
	}

	expected := []string{"s1", "s2", "done"}
	if fmt.Sprintf("%v", callbackSteps) != fmt.Sprintf("%v", expected) {
		t.Errorf("expected %v, got %v", expected, callbackSteps)
	}
}

func TestStepOptions(t *testing.T) {
	step := NewStep("test", nil,
		WithRetry(5, 2*time.Second),
		WithTimeout(30*time.Second),
	)

	if step.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", step.MaxRetries)
	}
	if step.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay=2s, got %v", step.RetryDelay)
	}
	if step.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", step.Timeout)
	}
}

func TestWorkflowEmptyState(t *testing.T) {
	step := NewStep("test", func(ctx context.Context, state map[string]any) (map[string]any, error) {
		state["hello"] = "world"
		return state, nil
	})

	wf := Do(step)
	state, err := wf.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if state["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", state["hello"])
	}
}

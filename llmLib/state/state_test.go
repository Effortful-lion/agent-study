package state

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMachineBasicFlow(t *testing.T) {
	const (
		PhaseStart   Phase = "start"
		PhaseProcess Phase = "process"
		PhaseEnd     Phase = "end"
	)

	def := &MachineDef{
		Initial: PhaseStart,
		Transitions: []Transition{
			{From: PhaseStart, To: PhaseProcess, Action: func(ctx context.Context, state map[string]any) (map[string]any, error) {
				state["processed"] = true
				return state, nil
			}},
			{From: PhaseProcess, To: PhaseEnd, Action: func(ctx context.Context, state map[string]any) (map[string]any, error) {
				state["done"] = true
				return state, nil
			}},
		},
	}

	m, err := NewMachine(def, nil)
	if err != nil {
		t.Fatalf("NewMachine failed: %v", err)
	}

	if err := m.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if m.Phase() != PhaseEnd {
		t.Errorf("expected phase %q, got %q", PhaseEnd, m.Phase())
	}

	if !m.State()["processed"].(bool) {
		t.Error("expected processed=true")
	}
	if !m.State()["done"].(bool) {
		t.Error("expected done=true")
	}
	if len(m.History()) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(m.History()))
	}
}

func TestMachineConditionalTransition(t *testing.T) {
	const (
		PhaseStart Phase = "start"
		PhaseA     Phase = "a"
		PhaseB     Phase = "b"
		PhaseEnd   Phase = "end"
	)

	def := &MachineDef{
		Initial: PhaseStart,
		Transitions: []Transition{
			{
				From: PhaseStart,
				To:   PhaseA,
				Condition: func(state map[string]any) bool {
					return state["go_a"] == true
				},
			},
			{
				From: PhaseStart,
				To:   PhaseB,
				Condition: func(state map[string]any) bool {
					return state["go_a"] != true
				},
			},
			{From: PhaseA, To: PhaseEnd},
			{From: PhaseB, To: PhaseEnd},
		},
	}

	// 测试走分支 A
	m, _ := NewMachine(def, map[string]any{"go_a": true})
	if err := m.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if m.History()[0].To != PhaseA {
		t.Errorf("expected to go to %q, got %q", PhaseA, m.History()[0].To)
	}

	// 测试走分支 B
	m2, _ := NewMachine(def, map[string]any{"go_a": false})
	if err := m2.Run(context.Background()); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if m2.History()[0].To != PhaseB {
		t.Errorf("expected to go to %q, got %q", PhaseB, m2.History()[0].To)
	}
}

func TestMachineRunUntil(t *testing.T) {
	const (
		PhaseStart Phase = "start"
		PhaseMid   Phase = "mid"
		PhaseEnd   Phase = "end"
	)

	def := &MachineDef{
		Initial: PhaseStart,
		Transitions: []Transition{
			{From: PhaseStart, To: PhaseMid},
			{From: PhaseMid, To: PhaseEnd},
		},
	}

	m, _ := NewMachine(def, nil)
	if err := m.RunUntil(context.Background(), PhaseMid); err != nil {
		t.Fatalf("RunUntil failed: %v", err)
	}

	if m.Phase() != PhaseMid {
		t.Errorf("expected phase %q, got %q", PhaseMid, m.Phase())
	}
}

func TestRetryMiddleware(t *testing.T) {
	callCount := 0
	step := func(ctx context.Context, state map[string]any) (map[string]any, error) {
		callCount++
		if callCount < 3 {
			return state, errors.New("temporary error")
		}
		state["ok"] = true
		return state, nil
	}

	mw := RetryMiddleware(3, 10*time.Millisecond, 50*time.Millisecond, nil)
	wrapped := mw(step)

	state, err := wrapped(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("wrapped step failed: %v", err)
	}
	if state["ok"] != true {
		t.Error("expected ok=true")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryMiddlewareExhausted(t *testing.T) {
	step := func(ctx context.Context, state map[string]any) (map[string]any, error) {
		return state, errors.New("always fails")
	}

	mw := RetryMiddleware(2, 10*time.Millisecond, 50*time.Millisecond, nil)
	wrapped := mw(step)

	_, err := wrapped(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	step := func(ctx context.Context, state map[string]any) (map[string]any, error) {
		select {
		case <-ctx.Done():
			return state, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return state, nil
		}
	}

	mw := TimeoutMiddleware(50 * time.Millisecond)
	wrapped := mw(step)

	_, err := wrapped(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	step := func(ctx context.Context, state map[string]any) (map[string]any, error) {
		panic("oops")
	}

	mw := RecoveryMiddleware()
	wrapped := mw(step)

	state, err := wrapped(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("recovery middleware should return error after panic")
	}
	if state["_error"] == nil {
		t.Error("expected _error in state")
	}
	t.Logf("recovered: %v", err)
}

func TestValidateMachineDef(t *testing.T) {
	tests := []struct {
		name    string
		def     *MachineDef
		wantErr bool
	}{
		{
			name:    "empty initial",
			def:     &MachineDef{Initial: "", Transitions: []Transition{{From: "a", To: "b"}}},
			wantErr: true,
		},
		{
			name:    "no transitions",
			def:     &MachineDef{Initial: "start"},
			wantErr: true,
		},
		{
			name: "no entry transition",
			def: &MachineDef{
				Initial:     "start",
				Transitions: []Transition{{From: "other", To: "end"}},
			},
			wantErr: true,
		},
		{
			name: "valid",
			def: &MachineDef{
				Initial:     "start",
				Transitions: []Transition{{From: "start", To: "end"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

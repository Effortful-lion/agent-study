package llmlib

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAgentBudget_ShouldStop(t *testing.T) {
	tests := []struct {
		name   string
		budget AgentBudgetConfig
		state  *State
		want   bool
	}{
		{
			name:   "no limit",
			budget: AgentBudgetConfig{},
			state:  &State{Step: 100, Usage: Usage{InputTokens: 1000000}},
			want:   false,
		},
		{
			name:   "exceed max steps",
			budget: AgentBudgetConfig{MaxSteps: 5},
			state:  &State{Step: 5},
			want:   true,
		},
		{
			name:   "not exceed max steps",
			budget: AgentBudgetConfig{MaxSteps: 5},
			state:  &State{Step: 4},
			want:   false,
		},
		{
			name:   "exceed max tokens",
			budget: AgentBudgetConfig{MaxTotalTokens: 100},
			state:  &State{Usage: Usage{InputTokens: 60, OutputTokens: 50}},
			want:   true,
		},
		{
			name:   "not exceed max tokens",
			budget: AgentBudgetConfig{MaxTotalTokens: 100},
			state:  &State{Usage: Usage{InputTokens: 40, OutputTokens: 50}},
			want:   false,
		},
		{
			name:   "exceed max duration",
			budget: AgentBudgetConfig{MaxDuration: time.Millisecond},
			state:  &State{StartedAt: time.Now().Add(-2 * time.Millisecond)},
			want:   true,
		},
		{
			name:   "not exceed max duration",
			budget: AgentBudgetConfig{MaxDuration: time.Hour},
			state:  &State{StartedAt: time.Now()},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.budget.ShouldStop(tt.state); got != tt.want {
				t.Errorf("ShouldStop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		category   ErrorCategory
		retryable  bool
	}{
		{name: "nil error", err: nil, statusCode: 200, category: "", retryable: true},
		{name: "timeout", err: context.DeadlineExceeded, statusCode: 0, category: ErrCategoryTimeout, retryable: true},
		{name: "canceled", err: context.Canceled, statusCode: 0, category: ErrCategoryCanceled, retryable: false},
		{name: "auth 401", err: errors.New("status=401 unauthorized"), statusCode: 401, category: ErrCategoryAuth, retryable: false},
		{name: "auth 403", err: errors.New("status=403 forbidden"), statusCode: 403, category: ErrCategoryAuth, retryable: false},
		{name: "not found", err: errors.New("status=404 not found"), statusCode: 404, category: ErrCategoryNotFound, retryable: false},
		{name: "rate limited", err: errors.New("status=429 too many requests"), statusCode: 429, category: ErrCategoryRateLimited, retryable: true},
		{name: "5xx", err: errors.New("status=500 internal server error"), statusCode: 500, category: ErrCategoryProviderError, retryable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCategory, gotRetryable := ClassifyError(tt.err, tt.statusCode)
			if gotCategory != tt.category {
				t.Errorf("ClassifyError() category = %v, want %v", gotCategory, tt.category)
			}
			if gotRetryable != tt.retryable {
				t.Errorf("ClassifyError() retryable = %v, want %v", gotRetryable, tt.retryable)
			}
		})
	}
}

func TestReActParadigm_Parse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantName string
	}{
		{
			name:     "empty input",
			input:    "",
			wantLen:  0,
			wantName: "",
		},
		{
			name:     "action format",
			input:    "Action: search\nAction Input: {\"query\": \"test\"}",
			wantLen:  1,
			wantName: "search",
		},
		{
			name:     "tag format",
			input:    "<function name=\"search\">{\"query\": \"test\"}</function>",
			wantLen:  1,
			wantName: "search",
		},
	}

	p := &ReActParadigm{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Parse(tt.input)
			if err != nil {
				t.Errorf("Parse() error = %v", err)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("Parse() len = %v, want %v", len(got), tt.wantLen)
				return
			}
			if tt.wantLen > 0 && got[0].Name != tt.wantName {
				t.Errorf("Parse() name = %v, want %v", got[0].Name, tt.wantName)
			}
		})
	}
}

func TestLevels(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		wantErr bool
		wantLen int
	}{
		{
			name: "simple linear",
			plan: Plan{Tasks: []Task{
				{ID: "t1"},
				{ID: "t2", DependsOn: []string{"t1"}},
				{ID: "t3", DependsOn: []string{"t2"}},
			}},
			wantErr: false,
			wantLen: 3,
		},
		{
			name: "parallel tasks",
			plan: Plan{Tasks: []Task{
				{ID: "t1"},
				{ID: "t2"},
				{ID: "t3", DependsOn: []string{"t1", "t2"}},
			}},
			wantErr: false,
			wantLen: 2,
		},
		{
			name:    "empty plan",
			plan:    Plan{Tasks: []Task{}},
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "duplicate id",
			plan: Plan{Tasks: []Task{
				{ID: "t1"},
				{ID: "t1"},
			}},
			wantErr: true,
			wantLen: 0,
		},
		{
			name: "missing dependency",
			plan: Plan{Tasks: []Task{
				{ID: "t1", DependsOn: []string{"nonexistent"}},
			}},
			wantErr: true,
			wantLen: 0,
		},
		{
			name: "cycle",
			plan: Plan{Tasks: []Task{
				{ID: "t1", DependsOn: []string{"t2"}},
				{ID: "t2", DependsOn: []string{"t1"}},
			}},
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Levels(tt.plan)
			if (err != nil) != tt.wantErr {
				t.Errorf("Levels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("Levels() len = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

type mockProvider struct {
	responses []*ChatResponse
	idx       int
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	if m.idx >= len(m.responses) {
		return &ChatResponse{Content: "done"}, nil
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return nil, nil
}

func (m *mockProvider) ChatWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (*ChatResponse, error) {
	return m.Chat(ctx, cfg, messages)
}

func (m *mockProvider) ChatStreamWithTools(ctx context.Context, cfg LLMConfig, messages []Message, tools []ToolDef) (<-chan StreamChunk, error) {
	return nil, nil
}

type mockTool struct {
	name        string
	description string
	result      string
}

func (t *mockTool) Name() string                  { return t.name }
func (t *mockTool) Description() string           { return t.description }
func (t *mockTool) Parameters() map[string]string { return map[string]string{} }
func (t *mockTool) Call(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return t.result, nil
}

func TestAgent_Run_WithToolCall(t *testing.T) {
	mockTool := &mockTool{
		name:        "get_time",
		description: "获取当前时间",
		result:      "2024-01-01 12:00:00",
	}

	mock := &mockProvider{
		responses: []*ChatResponse{
			{ToolCalls: []ToolCall{{ID: "call_1", Name: "get_time", Args: json.RawMessage("{}")}}},
			{Content: "当前时间是 2024-01-01 12:00:00"},
		},
	}

	registry := NewRegistryToolSet()
	registry.Register(mockTool)
	agent := New(mock, "test-model", registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := agent.Run(ctx, "现在几点了？")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var eventTypes []EventType
	var finalAnswer string
	for event := range events {
		eventTypes = append(eventTypes, event.Type)
		if event.Type == EventAnswerDelta {
			finalAnswer = event.Text
		}
	}

	expectedTypes := []EventType{EventToolCall, EventToolResult, EventAnswerDelta, EventDone}
	if len(eventTypes) != len(expectedTypes) {
		t.Errorf("event types len = %v, want %v. got: %v", len(eventTypes), len(expectedTypes), eventTypes)
	}

	for i, expected := range expectedTypes {
		if eventTypes[i] != expected {
			t.Errorf("event type[%d] = %v, want %v", i, eventTypes[i], expected)
		}
	}

	if finalAnswer != "当前时间是 2024-01-01 12:00:00" {
		t.Errorf("final answer = %v, want %v", finalAnswer, "当前时间是 2024-01-01 12:00:00")
	}
}

func TestAgent_Run_DirectAnswer(t *testing.T) {
	mock := &mockProvider{
		responses: []*ChatResponse{
			{Content: "直接回答"},
		},
	}

	agent := New(mock, "test-model", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := agent.Run(ctx, "你好")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var eventTypes []EventType
	var finalAnswer string
	for event := range events {
		eventTypes = append(eventTypes, event.Type)
		if event.Type == EventAnswerDelta {
			finalAnswer = event.Text
		}
	}

	expectedTypes := []EventType{EventAnswerDelta, EventDone}
	if len(eventTypes) != len(expectedTypes) {
		t.Errorf("event types len = %v, want %v. got: %v", len(eventTypes), len(expectedTypes), eventTypes)
	}

	if finalAnswer != "直接回答" {
		t.Errorf("final answer = %v, want %v", finalAnswer, "直接回答")
	}
}

type mockReActProvider struct {
	responses []*ChatResponse
	idx       int
}

func (m *mockReActProvider) Name() string { return "mock-react" }

func (m *mockReActProvider) Chat(ctx context.Context, cfg LLMConfig, messages []Message) (*ChatResponse, error) {
	if m.idx >= len(m.responses) {
		return &ChatResponse{Content: "done"}, nil
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp, nil
}

func (m *mockReActProvider) ChatStream(ctx context.Context, cfg LLMConfig, messages []Message) (<-chan StreamChunk, error) {
	return nil, nil
}

func TestAgent_Run_ReActFallback(t *testing.T) {
	mockTool := &mockTool{
		name:        "get_time",
		description: "获取当前时间",
		result:      "2024-01-01 12:00:00",
	}

	mock := &mockReActProvider{
		responses: []*ChatResponse{
			{Content: "我需要获取当前时间。\nAction: get_time\nAction Input: {}"},
			{Content: "当前时间是 2024-01-01 12:00:00"},
		},
	}

	registry := NewRegistryToolSet()
	registry.Register(mockTool)
	agent := New(mock, "test-model", registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := agent.Run(ctx, "现在几点了？")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var eventTypes []EventType
	var thoughts []string
	var finalAnswer string
	for event := range events {
		eventTypes = append(eventTypes, event.Type)
		if event.Type == EventThought {
			thoughts = append(thoughts, event.Text)
		}
		if event.Type == EventAnswerDelta {
			finalAnswer = event.Text
		}
	}

	expectedTypes := []EventType{EventThought, EventToolCall, EventToolResult, EventAnswerDelta, EventDone}
	if len(eventTypes) != len(expectedTypes) {
		t.Errorf("event types len = %v, want %v. got: %v", len(eventTypes), len(expectedTypes), eventTypes)
	}

	for i, expected := range expectedTypes {
		if eventTypes[i] != expected {
			t.Errorf("event type[%d] = %v, want %v", i, eventTypes[i], expected)
		}
	}

	if len(thoughts) == 0 || !strings.Contains(thoughts[0], "我需要获取当前时间") {
		t.Errorf("thought = %v, want to contain '我需要获取当前时间'", thoughts)
	}

	if finalAnswer != "当前时间是 2024-01-01 12:00:00" {
		t.Errorf("final answer = %v, want %v", finalAnswer, "当前时间是 2024-01-01 12:00:00")
	}
}

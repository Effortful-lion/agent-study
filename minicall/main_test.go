package main

import (
	"strings"
	"testing"
)

func TestParseChatResponse(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantText   string
		wantInput  int
		wantOutput int
		wantErr    bool
	}{
		{
			name: "content and usage",
			raw: `{
				"choices": [
					{"message": {"role": "assistant", "content": "hello"}}
				],
				"usage": {"prompt_tokens": 7, "completion_tokens": 11}
			}`,
			wantText:   "hello",
			wantInput:  7,
			wantOutput: 11,
		},
		{
			name:    "empty choices",
			raw:     `{"choices": [], "usage": {"prompt_tokens": 1, "completion_tokens": 2}}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			raw:     `{`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChatResponse(strings.NewReader(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseChatResponse returned nil error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseChatResponse returned error: %v", err)
			}
			if got.Content != tt.wantText || got.InputTokens != tt.wantInput || got.OutputTokens != tt.wantOutput {
				t.Fatalf("response = %#v, want content=%q input=%d output=%d", got, tt.wantText, tt.wantInput, tt.wantOutput)
			}
		})
	}
}

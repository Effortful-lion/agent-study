package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Effortful-lion/agent-study/minicall-tansuo/internal/llm"
	"github.com/Effortful-lion/agent-study/minicall-tansuo/pkg/stream"
)

func TestChatModelImplementsProviderChat(t *testing.T) {
	var gotAuth string
	var gotRequest llm.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		if err := json.Unmarshal(body, &gotRequest); err != nil {
			t.Fatalf("request body is not ChatRequest: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"pong"}}],
			"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
		}`))
	}))
	defer server.Close()

	var provider llm.Provider = NewDeepSeekModel(Config{
		Model:   "deepseek-chat",
		BaseURL: server.URL,
		APIKey:  "sk-test",
	})

	resp, err := provider.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("ping")}},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization = %q, want Bearer sk-test", gotAuth)
	}
	if gotRequest.Model != "deepseek-chat" {
		t.Fatalf("model = %q, want deepseek-chat", gotRequest.Model)
	}
	if gotRequest.Stream {
		t.Fatal("stream = true, want false")
	}
	if len(gotRequest.Messages) != 1 || gotRequest.Messages[0].Content.Text != "ping" {
		t.Fatalf("messages = %#v, want ping", gotRequest.Messages)
	}
	if resp.Content != "pong" {
		t.Fatalf("content = %q, want pong", resp.Content)
	}
	if resp.InputTokens != 2 || resp.OutputTokens != 3 {
		t.Fatalf("tokens = input %d output %d, want 2/3", resp.InputTokens, resp.OutputTokens)
	}
}

func TestChatModelChatStreamReturnsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewDeepSeekModel(Config{
		Model:   "deepseek-chat",
		BaseURL: server.URL,
		APIKey:  "sk-test",
	})

	ch, err := provider.ChatStream(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: llm.TextContent("ping")}},
	})
	if err != nil {
		t.Fatalf("ChatStream returned error: %v", err)
	}

	chunks, err := stream.Collect(context.Background(), ch)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	if chunks[0].Content != "Hel" || chunks[1].Content != "lo" {
		t.Fatalf("chunks = %#v, want Hel/lo", chunks)
	}
}

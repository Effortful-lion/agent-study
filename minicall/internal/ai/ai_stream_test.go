package ai

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatModelStreamInvokeChatWritesDeltaContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewChatModel(Config{
		Model:   "deepseek-chat",
		BaseURL: server.URL,
		APIKey:  "sk-test",
	})

	var out bytes.Buffer
	err := client.StreamInvokeChat(context.Background(), "ping", &out)
	if err != nil {
		t.Fatalf("StreamInvokeChat returned error: %v", err)
	}

	if got := out.String(); got != "Hello" {
		t.Fatalf("output = %q, want Hello", got)
	}
}

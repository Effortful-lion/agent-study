package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientStreamJSONReadsSSEEvents(t *testing.T) {
	var gotAuth string
	var gotAccept string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")

		bodyBytes, _ := io.ReadAll(r.Body)
		gotBody = string(bodyBytes)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)

	var events []string
	err := client.StreamJSON(context.Background(), "/chat/completions", map[string]string{
		"Authorization": "Bearer sk-test",
	}, map[string]any{"stream": true}, func(data string) error {
		events = append(events, data)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamJSON returned error: %v", err)
	}

	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization = %q, want Bearer sk-test", gotAuth)
	}
	if gotAccept != "text/event-stream" {
		t.Fatalf("accept = %q, want text/event-stream", gotAccept)
	}
	if !strings.Contains(gotBody, "\"stream\":true") {
		t.Fatalf("body = %q, want stream true", gotBody)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0] != "{\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}" {
		t.Fatalf("first event = %q", events[0])
	}
	if events[1] != "{\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}" {
		t.Fatalf("second event = %q", events[1])
	}
}

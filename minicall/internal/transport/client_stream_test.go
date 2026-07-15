package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	if events[0] != "{\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}" {
		t.Fatalf("first event = %q", events[0])
	}
	if events[1] != "{\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}" {
		t.Fatalf("second event = %q", events[1])
	}
	if events[2] != "[DONE]" {
		t.Fatalf("third event = %q", events[2])
	}
}

func TestNewClientUsesTransportTimeoutsWithoutGlobalClientTimeout(t *testing.T) {
	client := NewClient("https://example.com")

	if client.httpClient.Timeout != 0 {
		t.Fatalf("http client timeout = %s, want no global timeout", client.httpClient.Timeout)
	}

	tr, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.httpClient.Transport)
	}
	if tr.MaxIdleConns == 0 {
		t.Fatal("MaxIdleConns = 0, want configured connection pool")
	}
	if tr.MaxIdleConnsPerHost == 0 {
		t.Fatal("MaxIdleConnsPerHost = 0, want configured per-host pool")
	}
	if tr.IdleConnTimeout == 0 {
		t.Fatal("IdleConnTimeout = 0, want configured idle timeout")
	}
	if tr.TLSHandshakeTimeout == 0 {
		t.Fatal("TLSHandshakeTimeout = 0, want configured TLS handshake timeout")
	}
}

func TestClientPostJSONRetriesServerErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.PostJSON(context.Background(), "/chat/completions", nil, map[string]string{"hello": "world"}, &out); err != nil {
		t.Fatalf("PostJSON returned error: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if !out.OK {
		t.Fatal("out.OK = false, want true")
	}
}

func TestClientPostJSONStopsWhenContextIsCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	var out struct{}
	err := client.PostJSON(ctx, "/chat/completions", nil, map[string]string{"hello": "world"}, &out)
	if err == nil {
		t.Fatal("PostJSON returned nil error, want context timeout")
	}
	if ctx.Err() == nil {
		t.Fatal("ctx.Err() = nil, want context timeout")
	}
}

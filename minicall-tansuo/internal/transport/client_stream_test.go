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

	retryTr, ok := client.httpClient.Transport.(*retryTransport)
	if !ok {
		t.Fatalf("transport = %T, want *retryTransport", client.httpClient.Transport)
	}

	tr, ok := retryTr.base.(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", retryTr.base)
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

func TestNewHTTPClientRetriesThroughRoundTripper(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewHTTPClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, strings.NewReader(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestClientDoUsesBaseURLAndRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		if attempts < 2 {
			http.Error(w, "try again", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/chat/completions", strings.NewReader(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext returned error: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
		ok   bool
	}{
		{name: "empty", raw: "", ok: false},
		{name: "seconds", raw: "2", want: 2 * time.Second, ok: true},
		{name: "invalid seconds", raw: "abc", ok: false},
		{name: "http date", raw: time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat), ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := retryAfter(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if tt.name == "http date" {
				if got <= 0 || got > 3*time.Second {
					t.Fatalf("duration = %s, want about 2s", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("duration = %s, want %s", got, tt.want)
			}
		})
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

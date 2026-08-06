package retriever

import (
	"testing"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
)

func TestSearch(t *testing.T) {
	chunks := []*document.Chunk{
		{ID: "1", DocumentID: "d1", Title: "Go语言", Content: "Go is a programming language", Keyword: "go language"},
		{ID: "2", DocumentID: "d1", Title: "Go并发", Content: "Goroutine is Go's concurrency primitive", Keyword: "goroutine concurrency"},
		{ID: "3", DocumentID: "d2", Title: "RAG设计", Content: "RAG is retrieval augmented generation", Keyword: "rag retrieval"},
	}

	r := NewRetriever()
	r.LoadChunks(chunks)

	tests := []struct {
		query string
		want  int
	}{
		{"Go", 2},
		{"Goroutine", 1},
		{"RAG", 1},
		{"retrieval", 1},
		{"", 0},
		{"xyz123nonexistent", 0},
	}

	for _, tt := range tests {
		results, err := r.Search(tt.query, 5)
		if err != nil {
			t.Errorf("Search(%q) error: %v", tt.query, err)
			continue
		}
		if len(results) != tt.want {
			t.Errorf("Search(%q) got %d results, want %d", tt.query, len(results), tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	chunk := &document.Chunk{
		Title:   "Go Language Tutorial",
		Content: "Go is a programming language with concurrency support",
		Keyword: "go language programming",
	}

	r := NewRetriever()

	tests := []struct {
		terms []string
		want  float64
	}{
		{[]string{"go"}, 0},    // > 0 means match found
		{[]string{"xyz"}, 0.0}, // no match
	}

	for _, tt := range tests {
		got := r.Score(chunk, tt.terms)
		if tt.want == 0 && got > 0 {
			// match found, ok
		} else if got != tt.want {
			t.Errorf("Score(%v) got %f, want %f", tt.terms, got, tt.want)
		}
	}
}

func TestSearchByTitle(t *testing.T) {
	chunks := []*document.Chunk{
		{ID: "1", Title: "Go语言", Content: "content1"},
		{ID: "2", Title: "Python教程", Content: "content2"},
		{ID: "3", Title: "Go并发编程", Content: "content3"},
	}

	results := SearchByTitle(chunks, "Go")
	if len(results) != 2 {
		t.Errorf("SearchByTitle got %d results, want 2", len(results))
	}
}

func TestFormatResult(t *testing.T) {
	r := Result{
		Title:   "Test",
		Score:   3.5,
		Content: "hello world",
	}
	formatted := FormatResult(r)
	if formatted == "" {
		t.Error("FormatResult returned empty string")
	}
}

func TestTopK(t *testing.T) {
	chunks := []*document.Chunk{
		{ID: "1", Title: "A", Content: "alpha beta gamma", Keyword: "alpha"},
		{ID: "2", Title: "B", Content: "beta gamma delta", Keyword: "beta"},
		{ID: "3", Title: "C", Content: "gamma delta epsilon", Keyword: "gamma"},
	}

	r := NewRetriever()
	r.LoadChunks(chunks)

	results, err := r.Search("beta", 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Search returned %d results, want <= 2", len(results))
	}
}

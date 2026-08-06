package index

import (
	"strings"
	"testing"

	"github.com/Effortful-lion/agent-study/mini-kb/internal/document"
)

func TestChunk(t *testing.T) {
	doc := &document.Document{ID: "test-1", Title: "Test"}
	splitter := NewSplitter(100, 20)

	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"short", "hello", 1},
		{"exact", strings.Repeat("a", 100), 1},
		{"long", strings.Repeat("hello world ", 200), 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := splitter.Chunk(doc, tt.content)
			if err != nil {
				t.Fatalf("Chunk error: %v", err)
			}
			if len(chunks) != tt.want {
				t.Errorf("Chunk got %d chunks, want %d", len(chunks), tt.want)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"", ""},
		{"the quick brown fox jumps", "quick brown jumps"},
		{"Go语言是一种编程语言", "go语言 编程语言"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := ExtractKeywords(tt.text, 5)
			if tt.want == "" {
				if got != "" {
					t.Errorf("ExtractKeywords(%q) got %q, want empty", tt.text, got)
				}
			} else if got == "" {
				t.Errorf("ExtractKeywords(%q) got empty, want non-empty", tt.text)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	words := tokenize("Hello World")
	if len(words) == 0 {
		t.Error("tokenize returned empty for English")
	}

	words = tokenize("中文测试")
	if len(words) == 0 {
		t.Error("tokenize returned empty for Chinese")
	}
}

func TestIsChinese(t *testing.T) {
	if !isChinese("中文") {
		t.Error("isChinese should detect Chinese")
	}
	if isChinese("hello") {
		t.Error("isChinese should not detect English")
	}
}

func TestFindBreakPoint(t *testing.T) {
	tests := []struct {
		text   string
		maxPos int
		want   int
	}{
		{"hello w\norld foo", 10, 8},    // newline at pos 7, break after it (pos 8)
		{"hello world foo bar", 10, -1}, // no newline in range [6,9]
		{"short", 100, -1},              // maxPos > len
	}

	for _, tt := range tests {
		got := findBreakPoint(tt.text, tt.maxPos)
		if got != tt.want {
			t.Errorf("findBreakPoint(%q, %d) got %d, want %d", tt.text, tt.maxPos, got, tt.want)
		}
	}
}

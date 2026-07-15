package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseIntoParsesStructuredOutput(t *testing.T) {
	type Weather struct {
		City string  `json:"city"`
		Temp float64 `json:"temp"`
	}

	got, err := ParseInto[Weather](`{"city":"Shanghai","temp":31.5}`)
	if err != nil {
		t.Fatalf("ParseInto returned error: %v", err)
	}
	if got.City != "Shanghai" || got.Temp != 31.5 {
		t.Fatalf("weather = %#v, want Shanghai/31.5", got)
	}
}

func TestPtrReturnsPointer(t *testing.T) {
	got := Ptr(7)
	if got == nil || *got != 7 {
		t.Fatalf("Ptr result = %#v, want pointer to 7", got)
	}
}

func TestParseStrictRejectsUnknownFields(t *testing.T) {
	type Weather struct {
		City string `json:"city"`
	}

	_, err := ParseStrict[Weather](`{"city":"Shanghai","vendor_extra":"ignored in production"}`)
	if err == nil {
		t.Fatal("ParseStrict returned nil error, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field error", err)
	}
}

func TestToolCallKeepsArgumentsAsRawMessage(t *testing.T) {
	raw := `{"name":"weather","arguments":{"city":"Shanghai","days":3}}`

	call, err := ParseInto[ToolCall](raw)
	if err != nil {
		t.Fatalf("ParseInto returned error: %v", err)
	}
	if call.Name != "weather" {
		t.Fatalf("name = %q, want weather", call.Name)
	}

	type WeatherArgs struct {
		City string `json:"city"`
		Days int    `json:"days"`
	}
	args, err := ParseRawInto[WeatherArgs](call.Args)
	if err != nil {
		t.Fatalf("ParseRawInto returned error: %v", err)
	}
	if args.City != "Shanghai" || args.Days != 3 {
		t.Fatalf("args = %#v, want Shanghai/3", args)
	}
}

func TestMessageContentUnmarshalString(t *testing.T) {
	var content MessageContent
	if err := json.Unmarshal([]byte(`"hello"`), &content); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if content.Text != "hello" {
		t.Fatalf("Text = %q, want hello", content.Text)
	}
	if len(content.Parts) != 0 {
		t.Fatalf("Parts = %#v, want empty", content.Parts)
	}
}

func TestMessageContentUnmarshalParts(t *testing.T) {
	raw := `[
		{"type":"text","text":"look"},
		{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}},
		{"type":"video_url","video_url":{"url":"https://example.com/demo.mp4"}}
	]`

	var content MessageContent
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if content.Text != "" {
		t.Fatalf("Text = %q, want empty", content.Text)
	}
	if len(content.Parts) != 3 {
		t.Fatalf("Parts = %d, want 3", len(content.Parts))
	}
	if content.Parts[0].Text != "look" {
		t.Fatalf("text part = %#v, want look", content.Parts[0])
	}
	if content.Parts[1].ImageURL == nil || content.Parts[1].ImageURL.URL != "https://example.com/cat.png" {
		t.Fatalf("image part = %#v, want image url", content.Parts[1])
	}
	if content.Parts[2].VideoURL == nil || content.Parts[2].VideoURL.URL != "https://example.com/demo.mp4" {
		t.Fatalf("video part = %#v, want video url", content.Parts[2])
	}
}

func TestMessageContentMarshalUsesPartsWhenPresent(t *testing.T) {
	content := MessageContent{
		Text: "fallback",
		Parts: []ContentPart{
			{Type: "text", Text: "hello"},
		},
	}

	got, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if string(got) != `[{"type":"text","text":"hello"}]` {
		t.Fatalf("json = %s, want parts array", got)
	}
}

func TestSafeMarshalReturnsError(t *testing.T) {
	_, err := SafeMarshal(map[string]any{
		"bad": func() {},
	})
	if err == nil {
		t.Fatal("SafeMarshal returned nil error, want unsupported type error")
	}
}

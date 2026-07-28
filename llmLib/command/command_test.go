package command

import "testing"

func TestLoadCommandsParsesQuestion(t *testing.T) {
	builder := LoadCommands()

	rest, err := builder.Parse([]string{"--question", "nihao", "extra"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	got := builder.Get("question")
	if got != "nihao" {
		t.Fatalf("question = %q, want %q", got, "nihao")
	}
	t.Log("parsed question: ", got)

	if len(rest) != 1 || rest[0] != "extra" {
		t.Fatalf("rest = %#v, want %#v", rest, []string{"extra"})
	}
}

func TestRegisterParsesExtendedFlag(t *testing.T) {
	builder := NewCommandBuilder()
	if err := builder.Register("port", "端口号", ":8080"); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if _, err := builder.Parse([]string{"--port", ":9090"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if got := builder.Get("port"); got != ":9090" {
		t.Fatalf("port = %q, want %q", got, ":9090")
	}
}

func TestRegisterUsesDefaultValue(t *testing.T) {
	builder := NewCommandBuilder()
	if err := builder.Register("port", "端口号", ":8080"); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if _, err := builder.Parse(nil); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if got := builder.Get("port"); got != ":8080" {
		t.Fatalf("port = %q, want %q", got, ":8080")
	}
}
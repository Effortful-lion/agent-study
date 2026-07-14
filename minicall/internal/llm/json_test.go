package llm

import "testing"

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

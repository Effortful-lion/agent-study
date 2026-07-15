package stream

import (
	"context"
	"testing"
)

func TestCollectReadsAllItems(t *testing.T) {
	ch := make(chan string, 2)
	ch <- "Hel"
	ch <- "lo"
	close(ch)

	got, err := Collect(context.Background(), ch)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(got) != 2 || got[0] != "Hel" || got[1] != "lo" {
		t.Fatalf("items = %#v, want Hel/lo", got)
	}
}

func TestProcessStopsOnHandlerError(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 1
	close(ch)

	wantErr := errTestHandler
	err := Process(context.Background(), ch, func(int) error {
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

type testError string

func (e testError) Error() string {
	return string(e)
}

const errTestHandler = testError("handler failed")

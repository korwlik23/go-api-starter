package app

import (
	"context"
	"testing"
	"time"
)

func TestA1(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		result <- Run(ctx)
	}()

	select {
	case err := <-result:
		t.Fatalf("Run returned before shutdown: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Run returned an error during context shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

package app

import (
	"context"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
)

func TestA1(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		result <- runApplication(
			ctx,
			func() (config.Config, error) {
				return config.Config{}, nil
			},
			func(config.Config) error { return nil },
			func(context.Context, config.Config) (HTTPServer, func() error, error) {
				return stubHTTPServer{run: func(ctx context.Context) error {
					<-ctx.Done()
					return nil
				}}, func() error { return nil }, nil
			},
		)
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

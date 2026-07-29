package app

import (
	"context"
	"errors"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestW1APlatformCompositionOrder(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runApplication(
		ctx,
		func() (config.Config, error) {
			order = append(order, "load")
			return config.Config{}, nil
		},
		func(config.Config) error {
			order = append(order, "validate")
			return nil
		},
		func(context.Context, config.Config) (HTTPServer, func() error, error) {
			order = append(order, "build")
			return stubHTTPServer{run: func(ctx context.Context) error {
					order = append(order, "serve")
					return ctx.Err()
				}}, func() error {
					order = append(order, "close")
					return nil
				}, nil
		},
	)

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, []string{"load", "validate", "build", "serve", "close"}, order)
}

func TestW1AValidationFailureDoesNotBuildOrEchoSecret(t *testing.T) {
	t.Parallel()

	const secret = "must-never-appear"
	buildCalled := false
	err := runApplication(
		context.Background(),
		func() (config.Config, error) {
			return config.Config{}, nil
		},
		func(config.Config) error {
			return errors.New("configuration rejected")
		},
		func(context.Context, config.Config) (HTTPServer, func() error, error) {
			buildCalled = true
			return nil, nil, errors.New(secret)
		},
	)

	require.Error(t, err)
	require.False(t, buildCalled)
	require.NotContains(t, err.Error(), secret)
}

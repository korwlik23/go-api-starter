package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestA10Dependencies(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("server stopped")
	server := stubHTTPServer{
		run: func(context.Context) error {
			return expectedError
		},
	}

	err := (Dependencies{HTTPServer: server}).Run(context.Background())
	require.ErrorIs(t, err, expectedError)
}

type stubHTTPServer struct {
	run func(context.Context) error
}

func (server stubHTTPServer) Run(ctx context.Context) error {
	return server.run(ctx)
}

package app

import (
	"context"
	"errors"
)

var errHTTPServerRequired = errors.New("app: HTTP server dependency is required")

type HTTPServer interface {
	Run(context.Context) error
}

type Dependencies struct {
	HTTPServer HTTPServer
}

func (dependencies Dependencies) Run(ctx context.Context) error {
	if dependencies.HTTPServer == nil {
		return errHTTPServerRequired
	}
	return dependencies.HTTPServer.Run(ctx)
}

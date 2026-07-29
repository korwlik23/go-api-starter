package app

import (
	"context"
	"errors"

	"github.com/korwlik23/go-api-starter/internal/modular"
	operationsapp "github.com/korwlik23/go-api-starter/internal/modules/operations/application"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
)

var errModuleProjectionUnavailable = errors.New(
	"app: module projection unavailable",
)

type moduleProjectionReader interface {
	CurrentSnapshot(context.Context) (ports.ReconcileSnapshot, error)
}

func newModuleProjectionProbe(
	registry *modular.Registry,
	enabled map[string]bool,
	reader moduleProjectionReader,
) (func(context.Context) error, error) {
	if registry == nil || reader == nil {
		return nil, errModuleProjectionUnavailable
	}
	if _, err := modular.ProjectState(registry, enabled); err != nil {
		return nil, errModuleProjectionUnavailable
	}
	return func(ctx context.Context) error {
		if ctx == nil {
			return errModuleProjectionUnavailable
		}
		snapshot, err := reader.CurrentSnapshot(ctx)
		if err != nil {
			return errModuleProjectionUnavailable
		}
		if err := operationsapp.VerifyProjection(
			registry,
			enabled,
			snapshot,
		); err != nil {
			return errModuleProjectionUnavailable
		}
		return nil
	}, nil
}

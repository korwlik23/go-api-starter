package app

import (
	"context"
	"errors"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/stretchr/testify/require"
)

func TestModuleProjectionProbeFailsClosed(t *testing.T) {
	t.Parallel()

	registry, err := Modules()
	require.NoError(t, err)
	enabled := enabledModuleConfiguration(registry)

	reader := &projectionReaderFake{
		snapshot: ports.ReconcileSnapshot{
			Revision:     1,
			CatalogEpoch: 1,
		},
	}
	probe, err := newModuleProjectionProbe(registry, enabled, reader)
	require.NoError(t, err)
	require.Error(t, probe(context.Background()))

	privateFailure := errors.New("private database failure")
	reader.err = privateFailure
	require.ErrorIs(t, probe(context.Background()), errModuleProjectionUnavailable)
	require.NotErrorIs(t, probe(context.Background()), privateFailure)
}

type projectionReaderFake struct {
	snapshot ports.ReconcileSnapshot
	err      error
}

func (reader *projectionReaderFake) CurrentSnapshot(
	context.Context,
) (ports.ReconcileSnapshot, error) {
	return reader.snapshot, reader.err
}

package application

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/stretchr/testify/require"
)

func TestVerifyProjectionAcceptsPersistedReconcileOutput(t *testing.T) {
	t.Parallel()

	registry := reconcileRegistry(t, true)
	enabled := map[string]bool{"identity": true, "publishing": true}
	projection, err := buildReconcileProjection(
		ReconcileRequest{Registry: registry, Enabled: enabled},
		ports.ReconcileSnapshot{},
		func() (string, error) {
			return "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", nil
		},
	)
	require.NoError(t, err)

	require.NoError(t, VerifyProjection(
		registry,
		enabled,
		ports.ReconcileSnapshot{
			Revision:     projection.Revision,
			CatalogEpoch: projection.CatalogEpoch,
			Modules:      projection.Modules,
			Permissions:  projection.Permissions,
		},
	))
}

func TestVerifyProjectionFailsClosedOnDrift(t *testing.T) {
	t.Parallel()

	registry := reconcileRegistry(t, true)
	enabled := map[string]bool{"identity": true, "publishing": true}
	projection, err := buildReconcileProjection(
		ReconcileRequest{Registry: registry, Enabled: enabled},
		ports.ReconcileSnapshot{},
		func() (string, error) {
			return "018f1f64-7b2a-7c10-8bf2-3c40d5f67585", nil
		},
	)
	require.NoError(t, err)

	tests := map[string]func(*ports.ReconcileSnapshot){
		"module enabled state": func(snapshot *ports.ReconcileSnapshot) {
			snapshot.Modules[1].Enabled = false
		},
		"module checksum": func(snapshot *ports.ReconcileSnapshot) {
			snapshot.Modules[0].ModuleChecksum[0] ^= 0xff
		},
		"catalog checksum": func(snapshot *ports.ReconcileSnapshot) {
			snapshot.Modules[0].CatalogChecksum[0] ^= 0xff
		},
		"permission active state": func(snapshot *ports.ReconcileSnapshot) {
			snapshot.Permissions[0].Active = false
		},
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snapshot := ports.ReconcileSnapshot{
				Revision:     projection.Revision,
				CatalogEpoch: projection.CatalogEpoch,
				Modules:      cloneModuleRecords(projection.Modules),
				Permissions:  append([]ports.PermissionRecord(nil), projection.Permissions...),
			}
			mutate(&snapshot)
			require.ErrorIs(t, VerifyProjection(registry, enabled, snapshot), ErrProjectionDrift)
		})
	}
}

func cloneModuleRecords(input []ports.ModuleRecord) []ports.ModuleRecord {
	result := make([]ports.ModuleRecord, 0, len(input))
	for _, module := range input {
		module.ModuleChecksum = append([]byte(nil), module.ModuleChecksum...)
		module.ConfigChecksum = append([]byte(nil), module.ConfigChecksum...)
		module.CatalogChecksum = append([]byte(nil), module.CatalogChecksum...)
		result = append(result, module)
	}
	return result
}

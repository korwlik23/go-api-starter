package application

import (
	"bytes"
	"errors"
	"sort"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
)

var ErrProjectionDrift = errors.New("operations: module projection drift")

func VerifyProjection(
	registry *modular.Registry,
	enabled map[string]bool,
	snapshot ports.ReconcileSnapshot,
) error {
	if registry == nil || snapshot.Revision < 1 || snapshot.CatalogEpoch < 1 {
		return ErrProjectionDrift
	}
	state, err := modular.ProjectState(registry, enabled)
	if err != nil || len(snapshot.Modules) != len(state.Ordered()) {
		return ErrProjectionDrift
	}
	permissions, valid := canonicalSnapshotPermissions(snapshot.Permissions)
	if !valid || !permissionsMatchState(state, permissions) {
		return ErrProjectionDrift
	}
	configChecksum, err := checksumConfig(state)
	if err != nil {
		return ErrProjectionDrift
	}
	catalogChecksum, err := checksumCatalog(state, permissions)
	if err != nil {
		return ErrProjectionDrift
	}

	modules := make(map[string]ports.ModuleRecord, len(snapshot.Modules))
	for _, module := range snapshot.Modules {
		if _, duplicate := modules[module.ID]; duplicate {
			return ErrProjectionDrift
		}
		modules[module.ID] = module
	}
	for _, expected := range state.Ordered() {
		persisted, exists := modules[expected.ID]
		manifest, declared := registry.Manifest(expected.ID)
		if !exists || !declared {
			return ErrProjectionDrift
		}
		moduleChecksum, checksumErr := checksumValue(manifest)
		if checksumErr != nil ||
			persisted.Enabled != expected.Enabled ||
			persisted.ReconcileRevision != snapshot.Revision ||
			persisted.CatalogEpoch != snapshot.CatalogEpoch ||
			!bytes.Equal(persisted.ModuleChecksum, moduleChecksum) ||
			!bytes.Equal(persisted.ConfigChecksum, configChecksum) ||
			!bytes.Equal(persisted.CatalogChecksum, catalogChecksum) {
			return ErrProjectionDrift
		}
	}
	return nil
}

func canonicalSnapshotPermissions(
	input []ports.PermissionRecord,
) ([]ports.PermissionRecord, bool) {
	permissions := append([]ports.PermissionRecord(nil), input...)
	sort.Slice(permissions, func(left, right int) bool {
		return permissions[left].Key < permissions[right].Key
	})
	for index, permission := range permissions {
		if permission.Key == "" ||
			permission.ModuleID == "" ||
			(index > 0 && permissions[index-1].Key == permission.Key) {
			return nil, false
		}
	}
	return permissions, true
}

func permissionsMatchState(
	state *modular.StateProjection,
	persisted []ports.PermissionRecord,
) bool {
	expected := make(map[string]modular.ProjectedPermission)
	for _, module := range state.Ordered() {
		for _, permission := range module.Permissions {
			expected[permission.Key] = permission
		}
	}
	for _, permission := range persisted {
		projected, exists := expected[permission.Key]
		if !exists {
			if permission.Active {
				return false
			}
			continue
		}
		owner, _, canonical := strings.Cut(projected.Key, ".")
		if !canonical ||
			permission.ModuleID != owner ||
			permission.Scope != string(projected.Scope) ||
			permission.Tier != string(projected.Tier) ||
			permission.Delegable != projected.Delegable ||
			permission.Active != projected.Active {
			return false
		}
		delete(expected, permission.Key)
	}
	return len(expected) == 0
}
